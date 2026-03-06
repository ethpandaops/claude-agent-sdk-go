package claudesdk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestListSessionsSingleProject(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	claudeHome := filepath.Join(homeDir, ".claude")
	projectPath := filepath.Join(homeDir, "repo")
	projectDir := filepath.Join(claudeHome, "projects", projectBucketName(projectPath))

	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	sessionID := "123e4567-e89b-12d3-a456-426614174100"
	writeSessionJSONL(t, filepath.Join(projectDir, sessionID+".jsonl"), []map[string]any{
		{
			"type":      "user",
			"message":   map[string]any{"role": "user", "content": "What is 2+2?"},
			"cwd":       projectPath,
			"gitBranch": "main",
		},
		{
			"type":    "assistant",
			"message": map[string]any{"role": "assistant", "content": "4"},
		},
	})

	sessions := ListSessions(SessionListOptions{
		Directory:        projectPath,
		IncludeWorktrees: boolPtr(false),
		ClaudeHome:       claudeHome,
	})

	require.Len(t, sessions, 1)
	require.Equal(t, sessionID, sessions[0].SessionID)
	require.Equal(t, "What is 2+2?", sessions[0].Summary)
	require.NotNil(t, sessions[0].FirstPrompt)
	require.Equal(t, "What is 2+2?", *sessions[0].FirstPrompt)
	require.NotNil(t, sessions[0].GitBranch)
	require.Equal(t, "main", *sessions[0].GitBranch)
	require.NotNil(t, sessions[0].Cwd)
	require.Equal(t, projectPath, *sessions[0].Cwd)
}

func TestListSessionsSummaryPrecedenceAndSort(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	claudeHome := filepath.Join(homeDir, ".claude")
	projectPath := filepath.Join(homeDir, "repo")
	projectDir := filepath.Join(claudeHome, "projects", projectBucketName(projectPath))

	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	oldID := "123e4567-e89b-12d3-a456-426614174101"
	newID := "123e4567-e89b-12d3-a456-426614174102"

	oldPath := filepath.Join(projectDir, oldID+".jsonl")
	newPath := filepath.Join(projectDir, newID+".jsonl")

	writeSessionJSONL(t, oldPath, []map[string]any{
		{"type": "user", "message": map[string]any{"role": "user", "content": "older prompt"}},
		{"type": "summary", "summary": "older summary"},
	})
	writeSessionJSONL(t, newPath, []map[string]any{
		{"type": "user", "message": map[string]any{"role": "user", "content": "newer prompt"}},
		{"type": "summary", "summary": "auto summary", "customTitle": "Custom Title"},
	})

	require.NoError(t, os.Chtimes(oldPath, time.Unix(1, 0), time.Unix(1, 0)))
	require.NoError(t, os.Chtimes(newPath, time.Unix(2, 0), time.Unix(2, 0)))

	sessions := ListSessions(SessionListOptions{
		Directory:        projectPath,
		IncludeWorktrees: boolPtr(false),
		ClaudeHome:       claudeHome,
	})

	require.Len(t, sessions, 2)
	require.Equal(t, newID, sessions[0].SessionID)
	require.Equal(t, "Custom Title", sessions[0].Summary)
	require.NotNil(t, sessions[0].CustomTitle)
	require.Equal(t, "Custom Title", *sessions[0].CustomTitle)
	require.Equal(t, oldID, sessions[1].SessionID)
	require.Equal(t, "older summary", sessions[1].Summary)
}

func TestGetSessionMessagesBuildsConversationChainAndPages(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	claudeHome := filepath.Join(homeDir, ".claude")
	projectPath := filepath.Join(homeDir, "repo")
	projectDir := filepath.Join(claudeHome, "projects", projectBucketName(projectPath))

	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	sessionID := "123e4567-e89b-12d3-a456-426614174103"
	sessionPath := filepath.Join(projectDir, sessionID+".jsonl")
	writeSessionJSONL(t, sessionPath, []map[string]any{
		{
			"type":      "user",
			"uuid":      "u1",
			"sessionId": sessionID,
			"message":   map[string]any{"role": "user", "content": "first"},
		},
		{
			"type":       "assistant",
			"uuid":       "a1",
			"parentUuid": "u1",
			"sessionId":  sessionID,
			"message":    map[string]any{"role": "assistant", "content": "reply"},
		},
		{
			"type":       "user",
			"uuid":       "u2",
			"parentUuid": "a1",
			"sessionId":  sessionID,
			"message":    map[string]any{"role": "user", "content": "second"},
		},
		{
			"type":        "assistant",
			"uuid":        "side-a",
			"parentUuid":  "u1",
			"sessionId":   sessionID,
			"isSidechain": true,
			"message":     map[string]any{"role": "assistant", "content": "ignore me"},
		},
	})

	allMessages := GetSessionMessages(sessionID, SessionMessagesOptions{
		Directory:  projectPath,
		ClaudeHome: claudeHome,
	})
	require.Len(t, allMessages, 3)
	require.Equal(t, []string{"u1", "a1", "u2"}, []string{allMessages[0].UUID, allMessages[1].UUID, allMessages[2].UUID})

	page := GetSessionMessages(sessionID, SessionMessagesOptions{
		Directory:  projectPath,
		ClaudeHome: claudeHome,
		Limit:      2,
		Offset:     1,
	})
	require.Len(t, page, 2)
	require.Equal(t, []string{"a1", "u2"}, []string{page[0].UUID, page[1].UUID})
}

func TestGetSessionMessagesInvalidSessionIDReturnsEmpty(t *testing.T) {
	t.Parallel()

	messages := GetSessionMessages("not-a-uuid", SessionMessagesOptions{})
	require.Nil(t, messages)
}

func writeSessionJSONL(t *testing.T, path string, entries []map[string]any) {
	t.Helper()

	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		data, err := json.Marshal(entry)
		require.NoError(t, err)

		lines = append(lines, string(data))
	}

	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
}

func boolPtr(v bool) *bool {
	return &v
}
