package claudesdk

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStatSession_FromIndex(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	projectPath := filepath.Join(homeDir, "repo-a")
	sessionID := "123e4567-e89b-12d3-a456-426614174000"
	sessionContent := `{"type":"user","message":{"role":"user","content":"hello"}}` + "\n"

	sessionPath, projectDir := writeSessionFixture(t, homeDir, projectPath, sessionID, sessionContent)
	writeSessionsIndex(t, projectDir, []sessionsIndexEntry{
		{
			SessionID:    sessionID,
			FullPath:     sessionPath,
			ProjectPath:  projectPath,
			MessageCount: ptrInt(9),
			Created:      "2026-03-01T08:00:00Z",
			Modified:     "2026-03-01T08:30:00Z",
		},
	})

	stat, err := StatSession(
		context.Background(),
		sessionID,
		WithCwd(projectPath),
		WithClaudeHome(filepath.Join(homeDir, ".claude")),
	)
	require.NoError(t, err)
	require.Equal(t, sessionID, stat.SessionID)
	require.Equal(t, int64(len(sessionContent)), stat.SizeBytes)
	require.NotZero(t, stat.LastModified)
	require.NotNil(t, stat.MessageCount)
	require.Equal(t, 9, *stat.MessageCount)
	require.NotNil(t, stat.CreatedAt)
	require.NotNil(t, stat.UpdatedAt)
	require.Equal(t, time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC), *stat.CreatedAt)
	require.Equal(t, time.Date(2026, 3, 1, 8, 30, 0, 0, time.UTC), *stat.UpdatedAt)
}

func TestStatSession_FallbackWithoutIndex(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	projectPath := filepath.Join(homeDir, "repo-b")
	sessionID := "123e4567-e89b-12d3-a456-426614174001"

	writeSessionFixture(t, homeDir, projectPath, sessionID, "test\n")

	stat, err := StatSession(
		context.Background(),
		sessionID,
		WithCwd(projectPath),
		WithClaudeHome(filepath.Join(homeDir, ".claude")),
	)
	require.NoError(t, err)
	require.Equal(t, sessionID, stat.SessionID)
	require.Nil(t, stat.MessageCount)
	require.Nil(t, stat.CreatedAt)
	require.Nil(t, stat.UpdatedAt)
}

func TestStatSession_NotFound(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	projectPath := filepath.Join(homeDir, "repo-c")
	sessionID := "123e4567-e89b-12d3-a456-426614174002"

	stat, err := StatSession(
		context.Background(),
		sessionID,
		WithCwd(projectPath),
		WithClaudeHome(filepath.Join(homeDir, ".claude")),
	)
	require.Nil(t, stat)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrSessionNotFound))
}

func TestStatSession_InvalidSessionID(t *testing.T) {
	t.Parallel()

	stat, err := StatSession(context.Background(), "not-a-uuid")
	require.Nil(t, stat)
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrSessionNotFound))
}

func TestStatSession_ProjectScopedLookup(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	projectA := filepath.Join(homeDir, "repo-a")
	projectB := filepath.Join(homeDir, "repo-b")
	sessionID := "123e4567-e89b-12d3-a456-426614174003"

	sessionPath, projectDir := writeSessionFixture(t, homeDir, projectA, sessionID, "x\n")
	writeSessionsIndex(t, projectDir, []sessionsIndexEntry{
		{
			SessionID:   sessionID,
			FullPath:    sessionPath,
			ProjectPath: projectA,
		},
	})

	stat, err := StatSession(
		context.Background(),
		sessionID,
		WithCwd(projectB),
		WithClaudeHome(filepath.Join(homeDir, ".claude")),
	)
	require.Nil(t, stat)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrSessionNotFound))
}

func TestStatSession_UsesClaudeHomeOverride(t *testing.T) {
	defaultHome := t.TempDir()
	customHome := t.TempDir()
	projectPath := filepath.Join(customHome, "repo-x")
	sessionID := "123e4567-e89b-12d3-a456-426614174004"

	writeSessionFixture(t, customHome, projectPath, sessionID, "hello\n")
	setHomeEnv(t, defaultHome)

	stat, err := StatSession(
		context.Background(),
		sessionID,
		WithCwd(projectPath),
		WithClaudeHome(filepath.Join(customHome, ".claude")),
	)
	require.NoError(t, err)
	require.Equal(t, sessionID, stat.SessionID)
}

func TestStatSession_DefaultsUseCWDAndHome(t *testing.T) {
	homeDir := t.TempDir()
	projectPath := filepath.Join(homeDir, "repo-default")
	sessionID := "123e4567-e89b-12d3-a456-426614174005"

	writeSessionFixture(t, homeDir, projectPath, sessionID, "data\n")
	setHomeEnv(t, homeDir)

	originalWD, err := os.Getwd()
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	require.NoError(t, os.MkdirAll(projectPath, 0o755))
	require.NoError(t, os.Chdir(projectPath))

	stat, err := StatSession(context.Background(), sessionID)
	require.NoError(t, err)
	require.Equal(t, sessionID, stat.SessionID)
}

func TestStatSession_MalformedIndexTimestampsIgnored(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	projectPath := filepath.Join(homeDir, "repo-y")
	sessionID := "123e4567-e89b-12d3-a456-426614174006"
	sessionPath, projectDir := writeSessionFixture(t, homeDir, projectPath, sessionID, "ok\n")
	writeSessionsIndex(t, projectDir, []sessionsIndexEntry{
		{
			SessionID:    sessionID,
			FullPath:     sessionPath,
			ProjectPath:  projectPath,
			MessageCount: ptrInt(4),
			Created:      "not-a-time",
			Modified:     "also-not-a-time",
		},
	})

	stat, err := StatSession(
		context.Background(),
		sessionID,
		WithCwd(projectPath),
		WithClaudeHome(filepath.Join(homeDir, ".claude")),
	)
	require.NoError(t, err)
	require.NotNil(t, stat.MessageCount)
	require.Equal(t, 4, *stat.MessageCount)
	require.Nil(t, stat.CreatedAt)
	require.Nil(t, stat.UpdatedAt)
}

func setHomeEnv(t *testing.T, homeDir string) {
	t.Helper()

	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
}

func writeSessionFixture(
	t *testing.T,
	homeDir string,
	projectPath string,
	sessionID string,
	content string,
) (string, string) {
	t.Helper()

	projectDir := filepath.Join(homeDir, ".claude", "projects", projectBucketName(projectPath))
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	sessionPath := filepath.Join(projectDir, sessionID+".jsonl")
	require.NoError(t, os.WriteFile(sessionPath, []byte(content), 0o600))

	return sessionPath, projectDir
}

func writeSessionsIndex(t *testing.T, projectDir string, entries []sessionsIndexEntry) {
	t.Helper()

	indexData := sessionsIndex{
		Entries: entries,
	}

	raw, err := json.Marshal(indexData)
	require.NoError(t, err)

	indexPath := filepath.Join(projectDir, "sessions-index.json")
	require.NoError(t, os.WriteFile(indexPath, raw, 0o600))
}

func ptrInt(v int) *int {
	return &v
}
