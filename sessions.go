package claudesdk

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	sessionLiteReadBufSize    = 64 * 1024
	sessionMaxSanitizedLength = 200
	sessionUserType           = "user"
)

var (
	sessionUUIDPattern          = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	sessionSkipPromptPattern    = regexp.MustCompile(`^(?:<local-command-stdout>|<session-start-hook>|<tick>|<goal>|\[Request interrupted by user[^\]]*\]|\s*<ide_opened_file>[\s\S]*</ide_opened_file>\s*$|\s*<ide_selection>[\s\S]*</ide_selection>\s*$)`)
	sessionCommandNamePattern   = regexp.MustCompile(`<command-name>(.*?)</command-name>`)
	sessionSanitizePattern      = regexp.MustCompile(`[^a-zA-Z0-9]`)
	sessionTranscriptEntryTypes = map[string]struct{}{
		"user":       {},
		"assistant":  {},
		"progress":   {},
		"system":     {},
		"attachment": {},
	}
)

// SDKSessionInfo contains metadata for a persisted Claude session.
//
//nolint:tagliatelle // Session responses preserve existing snake_case JSON fields.
type SDKSessionInfo struct {
	SessionID    string  `json:"session_id"`
	Summary      string  `json:"summary"`
	LastModified int64   `json:"last_modified"`
	FileSize     int64   `json:"file_size"`
	CustomTitle  *string `json:"custom_title,omitempty"`
	FirstPrompt  *string `json:"first_prompt,omitempty"`
	GitBranch    *string `json:"git_branch,omitempty"`
	Cwd          *string `json:"cwd,omitempty"`
}

// SessionMessage contains a top-level user or assistant message from a saved transcript.
//
//nolint:tagliatelle // Session responses preserve existing snake_case JSON fields.
type SessionMessage struct {
	Type            string         `json:"type"`
	UUID            string         `json:"uuid"`
	SessionID       string         `json:"session_id"`
	Message         map[string]any `json:"message"`
	ParentToolUseID *string        `json:"parent_tool_use_id,omitempty"`
}

// SessionListOptions controls ListSessions behavior.
type SessionListOptions struct {
	Directory        string
	Limit            int
	IncludeWorktrees *bool
	ClaudeHome       string
}

// SessionMessagesOptions controls GetSessionMessages behavior.
type SessionMessagesOptions struct {
	Directory  string
	Limit      int
	Offset     int
	ClaudeHome string
}

// ListSessions lists Claude sessions for a project or across all projects.
func ListSessions(options SessionListOptions) []SDKSessionInfo {
	store := newSessionStore(options.ClaudeHome)

	includeWorktrees := true
	if options.IncludeWorktrees != nil {
		includeWorktrees = *options.IncludeWorktrees
	}

	if strings.TrimSpace(options.Directory) != "" {
		return store.listSessionsForProject(options.Directory, options.Limit, includeWorktrees)
	}

	return store.listAllSessions(options.Limit)
}

// GetSessionMessages returns top-level conversation messages from a saved session.
func GetSessionMessages(sessionID string, options SessionMessagesOptions) []SessionMessage {
	store := newSessionStore(options.ClaudeHome)

	return store.getSessionMessages(sessionID, options.Directory, options.Limit, options.Offset)
}

type sessionStore struct {
	claudeHome string
}

type sessionLiteFile struct {
	mtime int64
	size  int64
	head  string
	tail  string
}

type transcriptEntry map[string]any

func newSessionStore(claudeHome string) sessionStore {
	return sessionStore{claudeHome: strings.TrimSpace(claudeHome)}
}

func (s sessionStore) configHome() string {
	if s.claudeHome != "" {
		return filepath.Clean(s.claudeHome)
	}

	if configDir := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR")); configDir != "" {
		return filepath.Clean(configDir)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Clean(".claude")
	}

	return filepath.Join(homeDir, ".claude")
}

func (s sessionStore) projectsDir() string {
	return filepath.Join(s.configHome(), "projects")
}

func sanitizeProjectPath(name string) string {
	sanitized := sessionSanitizePattern.ReplaceAllString(name, "-")
	if sanitized == "" {
		return "-"
	}

	if len(sanitized) <= sessionMaxSanitizedLength {
		return sanitized
	}

	return sanitized[:sessionMaxSanitizedLength] + "-" + simpleSessionHash(name)
}

func simpleSessionHash(value string) string {
	hash := int32(0)
	for _, ch := range value {
		hash = hash*31 + ch
	}

	if hash < 0 {
		hash = -hash
	}

	if hash == 0 {
		return "0"
	}

	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"

	out := make([]byte, 0, 8)
	n := int64(hash)

	for n > 0 {
		out = append(out, digits[n%36])
		n /= 36
	}

	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}

	return string(out)
}

func canonicalizeSessionPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = filepath.Clean(path)
	}

	resolved, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return filepath.Clean(resolved)
	}

	return filepath.Clean(absPath)
}

func (s sessionStore) findProjectDir(projectPath string) string {
	projectsDir := s.projectsDir()
	sanitized := sanitizeProjectPath(projectPath)

	exact := filepath.Join(projectsDir, sanitized)
	if info, err := os.Stat(exact); err == nil && info.IsDir() {
		return exact
	}

	if len(sanitized) <= sessionMaxSanitizedLength {
		return ""
	}

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return ""
	}

	prefix := sanitized[:sessionMaxSanitizedLength] + "-"
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			return filepath.Join(projectsDir, entry.Name())
		}
	}

	return ""
}

func readSessionLite(path string) *sessionLiteFile {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}

	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.Size() == 0 {
		return nil
	}

	headBytes := make([]byte, sessionLiteReadBufSize)

	headN, err := file.Read(headBytes)
	if err != nil && headN == 0 {
		return nil
	}

	head := string(headBytes[:headN])
	tail := head

	if info.Size() > sessionLiteReadBufSize {
		if _, err := file.Seek(info.Size()-sessionLiteReadBufSize, 0); err == nil {
			tailBytes := make([]byte, sessionLiteReadBufSize)

			tailN, tailErr := file.Read(tailBytes)
			if tailErr == nil || tailN > 0 {
				tail = string(tailBytes[:tailN])
			}
		}
	}

	return &sessionLiteFile{
		mtime: info.ModTime().UnixMilli(),
		size:  info.Size(),
		head:  head,
		tail:  tail,
	}
}

func extractJSONStringField(text string, key string) *string {
	patterns := []string{`"` + key + `":"`, `"` + key + `": "`}

	for _, pattern := range patterns {
		searchFrom := 0
		for {
			idx := strings.Index(text[searchFrom:], pattern)
			if idx < 0 {
				break
			}

			valueStart := searchFrom + idx + len(pattern)

			for i := valueStart; i < len(text); i++ {
				if text[i] == '\\' {
					i++

					continue
				}

				if text[i] == '"' {
					value := unescapeJSONString(text[valueStart:i])

					return &value
				}
			}

			searchFrom = valueStart
		}
	}

	return nil
}

func extractLastJSONStringField(text string, key string) *string {
	var last *string

	patterns := []string{`"` + key + `":"`, `"` + key + `": "`}

	for _, pattern := range patterns {
		searchFrom := 0
		for {
			idx := strings.Index(text[searchFrom:], pattern)
			if idx < 0 {
				break
			}

			valueStart := searchFrom + idx + len(pattern)
			found := false

			for i := valueStart; i < len(text); i++ {
				if text[i] == '\\' {
					i++

					continue
				}

				if text[i] == '"' {
					value := unescapeJSONString(text[valueStart:i])
					last = &value
					searchFrom = i + 1
					found = true

					break
				}
			}

			if !found {
				break
			}
		}
	}

	return last
}

func unescapeJSONString(value string) string {
	if !strings.Contains(value, `\`) {
		return value
	}

	var result string
	if err := json.Unmarshal([]byte(`"`+value+`"`), &result); err == nil {
		return result
	}

	return value
}

func extractFirstPromptFromHead(head string) *string {
	lines := strings.Split(head, "\n")
	commandFallback := ""

	for _, line := range lines {
		if !strings.Contains(line, `"type":"`+sessionUserType+`"`) &&
			!strings.Contains(line, `"type": "`+sessionUserType+`"`) {
			continue
		}

		if strings.Contains(line, `"tool_result"`) ||
			strings.Contains(line, `"isMeta":true`) ||
			strings.Contains(line, `"isMeta": true`) ||
			strings.Contains(line, `"isCompactSummary":true`) ||
			strings.Contains(line, `"isCompactSummary": true`) {
			continue
		}

		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if entry["type"] != sessionUserType {
			continue
		}

		messageData, ok := entry["message"].(map[string]any)
		if !ok {
			continue
		}

		var texts []string

		switch content := messageData["content"].(type) {
		case string:
			texts = append(texts, content)
		case []any:
			for _, block := range content {
				blockData, ok := block.(map[string]any)
				if !ok || blockData["type"] != "text" {
					continue
				}

				if text, ok := blockData["text"].(string); ok {
					texts = append(texts, text)
				}
			}
		}

		for _, raw := range texts {
			result := strings.TrimSpace(strings.ReplaceAll(raw, "\n", " "))
			if result == "" {
				continue
			}

			if match := sessionCommandNamePattern.FindStringSubmatch(result); len(match) > 1 {
				if commandFallback == "" {
					commandFallback = match[1]
				}

				continue
			}

			if sessionSkipPromptPattern.MatchString(result) {
				continue
			}

			if len(result) > 200 {
				result = strings.TrimSpace(result[:200]) + "..."
			}

			return &result
		}
	}

	if commandFallback != "" {
		return &commandFallback
	}

	return nil
}

func (s sessionStore) readSessionsFromDir(projectDir string, projectPath string) []SDKSessionInfo {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil
	}

	sessions := make([]SDKSessionInfo, 0, len(entries))

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}

		sessionID := strings.TrimSuffix(entry.Name(), ".jsonl")
		if !sessionUUIDPattern.MatchString(sessionID) {
			continue
		}

		lite := readSessionLite(filepath.Join(projectDir, entry.Name()))
		if lite == nil {
			continue
		}

		firstLine := lite.head
		if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
			firstLine = firstLine[:idx]
		}

		if strings.Contains(firstLine, `"isSidechain":true`) || strings.Contains(firstLine, `"isSidechain": true`) {
			continue
		}

		customTitle := extractLastJSONStringField(lite.tail, "customTitle")
		firstPrompt := extractFirstPromptFromHead(lite.head)

		summary := customTitle
		if summary == nil {
			summary = extractLastJSONStringField(lite.tail, "summary")
		}

		if summary == nil {
			summary = firstPrompt
		}

		if summary == nil || *summary == "" {
			continue
		}

		gitBranch := extractLastJSONStringField(lite.tail, "gitBranch")
		if gitBranch == nil {
			gitBranch = extractJSONStringField(lite.head, "gitBranch")
		}

		sessionCwd := extractJSONStringField(lite.head, "cwd")
		if sessionCwd == nil && strings.TrimSpace(projectPath) != "" {
			projectPathCopy := projectPath
			sessionCwd = &projectPathCopy
		}

		sessions = append(sessions, SDKSessionInfo{
			SessionID:    sessionID,
			Summary:      *summary,
			LastModified: lite.mtime,
			FileSize:     lite.size,
			CustomTitle:  customTitle,
			FirstPrompt:  firstPrompt,
			GitBranch:    gitBranch,
			Cwd:          sessionCwd,
		})
	}

	return sessions
}

func (s sessionStore) listSessionsForProject(directory string, limit int, includeWorktrees bool) []SDKSessionInfo {
	canonicalDir := canonicalizeSessionPath(directory)
	if canonicalDir == "" {
		return nil
	}

	if !includeWorktrees {
		projectDir := s.findProjectDir(canonicalDir)
		if projectDir == "" {
			return nil
		}

		return applySessionSortAndLimit(s.readSessionsFromDir(projectDir, canonicalDir), limit)
	}

	worktrees := getGitWorktreePaths(canonicalDir)
	if len(worktrees) <= 1 {
		projectDir := s.findProjectDir(canonicalDir)
		if projectDir == "" {
			return nil
		}

		return applySessionSortAndLimit(s.readSessionsFromDir(projectDir, canonicalDir), limit)
	}

	projectsDir := s.projectsDir()

	dirEntries, err := os.ReadDir(projectsDir)
	if err != nil {
		projectDir := s.findProjectDir(canonicalDir)
		if projectDir == "" {
			return nil
		}

		return applySessionSortAndLimit(s.readSessionsFromDir(projectDir, canonicalDir), limit)
	}

	type indexedWorktree struct {
		path   string
		prefix string
	}

	indexed := make([]indexedWorktree, 0, len(worktrees))
	for _, worktree := range worktrees {
		indexed = append(indexed, indexedWorktree{
			path:   worktree,
			prefix: sanitizeProjectPath(worktree),
		})
	}

	sort.Slice(indexed, func(i, j int) bool {
		return len(indexed[i].prefix) > len(indexed[j].prefix)
	})

	var sessions []SDKSessionInfo

	seen := make(map[string]struct{})

	if projectDir := s.findProjectDir(canonicalDir); projectDir != "" {
		seen[strings.ToLower(filepath.Base(projectDir))] = struct{}{}
		sessions = append(sessions, s.readSessionsFromDir(projectDir, canonicalDir)...)
	}

	for _, entry := range dirEntries {
		if !entry.IsDir() {
			continue
		}

		name := strings.ToLower(entry.Name())
		if _, ok := seen[name]; ok {
			continue
		}

		for _, worktree := range indexed {
			isMatch := entry.Name() == worktree.prefix ||
				(len(worktree.prefix) >= sessionMaxSanitizedLength && strings.HasPrefix(entry.Name(), worktree.prefix+"-"))
			if !isMatch {
				continue
			}

			seen[name] = struct{}{}

			sessions = append(sessions, s.readSessionsFromDir(filepath.Join(projectsDir, entry.Name()), worktree.path)...)

			break
		}
	}

	return applySessionSortAndLimit(deduplicateSessionInfos(sessions), limit)
}

func (s sessionStore) listAllSessions(limit int) []SDKSessionInfo {
	entries, err := os.ReadDir(s.projectsDir())
	if err != nil {
		return nil
	}

	var sessions []SDKSessionInfo

	for _, entry := range entries {
		if entry.IsDir() {
			sessions = append(sessions, s.readSessionsFromDir(filepath.Join(s.projectsDir(), entry.Name()), "")...)
		}
	}

	return applySessionSortAndLimit(deduplicateSessionInfos(sessions), limit)
}

func deduplicateSessionInfos(sessions []SDKSessionInfo) []SDKSessionInfo {
	byID := make(map[string]SDKSessionInfo, len(sessions))
	for _, session := range sessions {
		existing, ok := byID[session.SessionID]
		if !ok || session.LastModified > existing.LastModified {
			byID[session.SessionID] = session
		}
	}

	result := make([]SDKSessionInfo, 0, len(byID))
	for _, session := range byID {
		result = append(result, session)
	}

	return result
}

func applySessionSortAndLimit(sessions []SDKSessionInfo, limit int) []SDKSessionInfo {
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastModified > sessions[j].LastModified
	})

	if limit > 0 && len(sessions) > limit {
		return sessions[:limit]
	}

	return sessions
}

func getGitWorktreePaths(cwd string) []string {
	if strings.TrimSpace(cwd) == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	cmd.Dir = cwd

	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var paths []string

	for _, line := range strings.Split(string(output), "\n") {
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}

		paths = append(paths, filepath.Clean(strings.TrimPrefix(line, "worktree ")))
	}

	return paths
}

func (s sessionStore) readSessionFile(sessionID string, directory string) string {
	fileName := sessionID + ".jsonl"

	if strings.TrimSpace(directory) != "" {
		canonicalDir := canonicalizeSessionPath(directory)
		if projectDir := s.findProjectDir(canonicalDir); projectDir != "" {
			if content, ok := tryReadSessionFile(projectDir, fileName); ok {
				return content
			}
		}

		for _, worktree := range getGitWorktreePaths(canonicalDir) {
			if worktree == canonicalDir {
				continue
			}

			if projectDir := s.findProjectDir(worktree); projectDir != "" {
				if content, ok := tryReadSessionFile(projectDir, fileName); ok {
					return content
				}
			}
		}

		return ""
	}

	entries, err := os.ReadDir(s.projectsDir())
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if content, ok := tryReadSessionFile(filepath.Join(s.projectsDir(), entry.Name()), fileName); ok {
			return content
		}
	}

	return ""
}

func (s sessionStore) findSessionFilePath(sessionID string, directory string) string {
	if !sessionUUIDPattern.MatchString(sessionID) {
		return ""
	}

	fileName := sessionID + ".jsonl"

	if strings.TrimSpace(directory) != "" {
		canonicalDir := canonicalizeSessionPath(directory)
		if projectDir := s.findProjectDir(canonicalDir); projectDir != "" {
			path := filepath.Join(projectDir, fileName)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}

		for _, worktree := range getGitWorktreePaths(canonicalDir) {
			if projectDir := s.findProjectDir(worktree); projectDir != "" {
				path := filepath.Join(projectDir, fileName)
				if _, err := os.Stat(path); err == nil {
					return path
				}
			}
		}

		return ""
	}

	content := s.readSessionFile(sessionID, "")
	if content == "" {
		return ""
	}

	entries, err := os.ReadDir(s.projectsDir())
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		path := filepath.Join(s.projectsDir(), entry.Name(), fileName)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

func tryReadSessionFile(projectDir string, fileName string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(projectDir, fileName))
	if err != nil {
		return "", false
	}

	return string(data), true
}

func parseTranscriptEntries(content string) []transcriptEntry {
	lines := strings.Split(content, "\n")
	entries := make([]transcriptEntry, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry transcriptEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		entryType, _ := entry["type"].(string)
		if _, ok := sessionTranscriptEntryTypes[entryType]; !ok {
			continue
		}

		if _, ok := entry["uuid"].(string); !ok {
			continue
		}

		entries = append(entries, entry)
	}

	return entries
}

func buildConversationChain(entries []transcriptEntry) []transcriptEntry {
	if len(entries) == 0 {
		return nil
	}

	byUUID := make(map[string]transcriptEntry, len(entries))
	entryIndex := make(map[string]int, len(entries))
	parentUUIDs := make(map[string]struct{}, len(entries))

	for i, entry := range entries {
		uuid, _ := entry["uuid"].(string)
		byUUID[uuid] = entry
		entryIndex[uuid] = i

		if parent, ok := entry["parentUuid"].(string); ok && parent != "" {
			parentUUIDs[parent] = struct{}{}
		}
	}

	var terminals []transcriptEntry

	for _, entry := range entries {
		uuid, _ := entry["uuid"].(string)
		if _, ok := parentUUIDs[uuid]; !ok {
			terminals = append(terminals, entry)
		}
	}

	var leaves []transcriptEntry

	for _, terminal := range terminals {
		current := terminal
		seen := make(map[string]struct{})

		for current != nil {
			uuid, _ := current["uuid"].(string)
			if _, ok := seen[uuid]; ok {
				break
			}

			seen[uuid] = struct{}{}

			entryType, _ := current["type"].(string)
			if entryType == sessionUserType || entryType == "assistant" {
				leaves = append(leaves, current)

				break
			}

			parent, _ := current["parentUuid"].(string)
			current = byUUID[parent]
		}
	}

	if len(leaves) == 0 {
		return nil
	}

	mainLeaves := make([]transcriptEntry, 0, len(leaves))
	for _, leaf := range leaves {
		if truthy(leaf["isSidechain"]) || truthy(leaf["isMeta"]) {
			continue
		}

		if teamName, _ := leaf["teamName"].(string); teamName != "" {
			continue
		}

		mainLeaves = append(mainLeaves, leaf)
	}

	leafCandidates := mainLeaves
	if len(leafCandidates) == 0 {
		leafCandidates = leaves
	}

	best := leafCandidates[0]
	bestUUID, _ := best["uuid"].(string)
	bestIdx := entryIndex[bestUUID]

	for _, candidate := range leafCandidates[1:] {
		candidateUUID, _ := candidate["uuid"].(string)

		idx := entryIndex[candidateUUID]
		if idx > bestIdx {
			best = candidate
			bestIdx = idx
		}
	}

	var chain []transcriptEntry

	seen := make(map[string]struct{})

	current := best
	for current != nil {
		uuid, _ := current["uuid"].(string)
		if _, ok := seen[uuid]; ok {
			break
		}

		seen[uuid] = struct{}{}

		chain = append(chain, current)

		parent, _ := current["parentUuid"].(string)
		current = byUUID[parent]
	}

	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	return chain
}

func truthy(value any) bool {
	boolVal, _ := value.(bool)

	return boolVal
}

func isVisibleSessionMessage(entry transcriptEntry) bool {
	entryType, _ := entry["type"].(string)
	if entryType != sessionUserType && entryType != "assistant" {
		return false
	}

	if truthy(entry["isMeta"]) || truthy(entry["isSidechain"]) {
		return false
	}

	teamName, _ := entry["teamName"].(string)

	return teamName == ""
}

func toSessionMessage(entry transcriptEntry) SessionMessage {
	entryType, _ := entry["type"].(string)
	uuid, _ := entry["uuid"].(string)

	sessionID, _ := entry["sessionId"].(string)
	if sessionID == "" {
		sessionID, _ = entry["session_id"].(string)
	}

	messageData, _ := entry["message"].(map[string]any)

	return SessionMessage{
		Type:            entryType,
		UUID:            uuid,
		SessionID:       sessionID,
		Message:         messageData,
		ParentToolUseID: nil,
	}
}

func (s sessionStore) getSessionMessages(sessionID string, directory string, limit int, offset int) []SessionMessage {
	if !sessionUUIDPattern.MatchString(sessionID) {
		return nil
	}

	content := s.readSessionFile(sessionID, directory)
	if content == "" {
		return nil
	}

	entries := parseTranscriptEntries(content)

	chain := buildConversationChain(entries)
	if len(chain) == 0 {
		return nil
	}

	messages := make([]SessionMessage, 0, len(chain))
	for _, entry := range chain {
		if isVisibleSessionMessage(entry) {
			messages = append(messages, toSessionMessage(entry))
		}
	}

	if offset > 0 {
		if offset >= len(messages) {
			return nil
		}

		messages = messages[offset:]
	}

	if limit > 0 && len(messages) > limit {
		messages = messages[:limit]
	}

	return messages
}
