package claudesdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var sessionIDPattern = regexp.MustCompile(
	"^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$",
)

// SessionStat contains local metadata for a persisted Claude session.
//
// This metadata is read from local Claude persistence files. It does not
// represent live remote state or lock/occupancy status.
type SessionStat struct {
	SessionID    string
	SizeBytes    int64
	LastModified time.Time
	MessageCount *int
	CreatedAt    *time.Time
	UpdatedAt    *time.Time
}

type sessionsIndex struct {
	Entries []sessionsIndexEntry `json:"entries"`
}

type sessionsIndexEntry struct {
	SessionID    string `json:"sessionId"`
	FullPath     string `json:"fullPath"`
	ProjectPath  string `json:"projectPath"`
	MessageCount *int   `json:"messageCount"`
	Created      string `json:"created"`
	Modified     string `json:"modified"`
}

// StatSession returns metadata for a locally persisted session.
//
// The lookup is project-scoped and read-only; no prompt is sent and no session
// content is modified. Returns ErrSessionNotFound when the session cannot be found
// in local Claude persistence for the resolved project path.
func StatSession(
	ctx context.Context,
	sessionID string,
	opts ...Option,
) (*SessionStat, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if !sessionIDPattern.MatchString(sessionID) {
		return nil, fmt.Errorf("invalid session ID %q: must be UUID format", sessionID)
	}

	options := applyAgentOptions(opts)

	projectPath := options.Cwd
	if projectPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolve current project path: %w", err)
		}

		projectPath = cwd
	}

	claudeHome := options.ClaudeHome
	if claudeHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve user home: %w", err)
		}

		claudeHome = filepath.Join(home, ".claude")
	}

	var err error

	projectPath, err = normalizeAbsolutePath(projectPath, "project path")
	if err != nil {
		return nil, err
	}

	claudeHome, err = normalizeAbsolutePath(claudeHome, "Claude home")
	if err != nil {
		return nil, err
	}

	projectsDir := filepath.Join(claudeHome, "projects")

	path, indexEntry, err := findSessionPath(ctx, projectsDir, projectPath, sessionID)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrSessionNotFound
		}

		return nil, fmt.Errorf("stat session file: %w", err)
	}

	stat := &SessionStat{
		SessionID:    sessionID,
		SizeBytes:    info.Size(),
		LastModified: info.ModTime(),
	}

	applyIndexMetadata(stat, indexEntry)

	return stat, nil
}

func normalizeAbsolutePath(path, label string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("%s cannot be empty", label)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", label, err)
	}

	return filepath.Clean(absPath), nil
}

func findSessionPath(
	ctx context.Context,
	projectsDir string,
	projectPath string,
	sessionID string,
) (string, *sessionsIndexEntry, error) {
	projectBucket := projectBucketName(projectPath)
	projectDir := filepath.Join(projectsDir, projectBucket)

	// Fast path: project's own index file.
	projectIndexPath := filepath.Join(projectDir, "sessions-index.json")

	sessionPath, entry, found, err := findInIndex(projectIndexPath, sessionID, projectPath)
	if err != nil {
		return "", nil, err
	}

	if found {
		return sessionPath, entry, nil
	}

	// Fallback: scan other project indexes in case local naming differs.
	if entries, err := os.ReadDir(projectsDir); err == nil {
		for _, dirEntry := range entries {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return "", nil, ctxErr
			}

			if !dirEntry.IsDir() {
				continue
			}

			indexPath := filepath.Join(projectsDir, dirEntry.Name(), "sessions-index.json")

			sessionPath, entry, found, err = findInIndex(indexPath, sessionID, projectPath)
			if err != nil {
				return "", nil, err
			}

			if found {
				return sessionPath, entry, nil
			}
		}
	}

	// Final fallback for stale/missing index.
	fallbackPath := filepath.Join(projectDir, sessionID+".jsonl")
	if _, err := os.Stat(fallbackPath); err == nil {
		return fallbackPath, nil, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", nil, fmt.Errorf("check session file: %w", err)
	}

	return "", nil, ErrSessionNotFound
}

func findInIndex(
	indexPath string,
	sessionID string,
	projectPath string,
) (string, *sessionsIndexEntry, bool, error) {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, false, nil
		}

		return "", nil, false, fmt.Errorf("read sessions index %q: %w", indexPath, err)
	}

	var index sessionsIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return "", nil, false, fmt.Errorf("parse sessions index %q: %w", indexPath, err)
	}

	for _, entry := range index.Entries {
		if entry.SessionID != sessionID || entry.ProjectPath != projectPath {
			continue
		}

		if entry.FullPath == "" {
			return "", nil, false, nil
		}

		if _, err := os.Stat(entry.FullPath); err == nil {
			match := entry

			return entry.FullPath, &match, true, nil
		} else if errors.Is(err, os.ErrNotExist) {
			return "", nil, false, nil
		} else {
			return "", nil, false, fmt.Errorf("stat indexed session file %q: %w", entry.FullPath, err)
		}
	}

	return "", nil, false, nil
}

func projectBucketName(projectPath string) string {
	path := filepath.ToSlash(strings.TrimSpace(projectPath))
	path = strings.TrimPrefix(path, "/")
	path = strings.ReplaceAll(path, ":", "-")
	path = strings.ReplaceAll(path, "/", "-")

	if path == "" {
		return "-"
	}

	if strings.HasPrefix(path, "-") {
		return path
	}

	return "-" + path
}

func applyIndexMetadata(stat *SessionStat, entry *sessionsIndexEntry) {
	if stat == nil || entry == nil {
		return
	}

	if entry.MessageCount != nil {
		count := *entry.MessageCount
		stat.MessageCount = &count
	}

	if created, err := time.Parse(time.RFC3339, entry.Created); err == nil {
		stat.CreatedAt = &created
	}

	if updated, err := time.Parse(time.RFC3339, entry.Modified); err == nil {
		stat.UpdatedAt = &updated
	}
}
