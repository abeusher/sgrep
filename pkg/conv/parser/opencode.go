package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/XiaoConstantine/sgrep/pkg/conv"
)

// OpenCodeParser parses OpenCode conversation files.
type OpenCodeParser struct {
	basePath string
}

type opencodeSession struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectID"`
	Title     string `json:"title"`
	Version   string `json:"version"`
	Directory string `json:"directory"`
	Time      struct {
		Created string `json:"created"`
		Updated string `json:"updated"`
	} `json:"time"`
}

type opencodeMessage struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionID"`
	Role      string `json:"role"`
	Agent     string `json:"agent"`
	ModelID   string `json:"modelID"`
	Provider  string `json:"providerID"`
	Time      struct {
		Created   string `json:"created"`
		Completed string `json:"completed"`
	} `json:"time"`
}

type opencodePart struct {
	Type    string
	Text    string
	Created time.Time
	Name    string
}

// NewOpenCodeParser creates a new OpenCode parser.
func NewOpenCodeParser() *OpenCodeParser {
	homeDir, _ := os.UserHomeDir()
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(homeDir, ".local", "share")
	}
	return &OpenCodeParser{
		basePath: filepath.Join(dataHome, "opencode", "storage"),
	}
}

// NewOpenCodeParserWithPath creates a parser with a custom base path.
func NewOpenCodeParserWithPath(basePath string) *OpenCodeParser {
	return &OpenCodeParser{basePath: basePath}
}

// AgentType returns the agent type.
func (p *OpenCodeParser) AgentType() conv.AgentType {
	return conv.AgentOpenCode
}

// DefaultPath returns the default path for OpenCode conversations.
func (p *OpenCodeParser) DefaultPath() string {
	return p.basePath
}

// Discover finds all OpenCode session files.
func (p *OpenCodeParser) Discover() ([]string, error) {
	var paths []string
	sessionsDir := filepath.Join(p.basePath, "session")

	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		return nil, nil
	}

	err := filepath.Walk(sessionsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasPrefix(info.Name(), "ses_") && strings.HasSuffix(info.Name(), ".json") {
			paths = append(paths, path)
		}
		return nil
	})

	return paths, err
}

// Parse reads an OpenCode session file and returns a session.
func (p *OpenCodeParser) Parse(sourcePath string) ([]*conv.Session, error) {
	session, err := p.loadSession(sourcePath)
	if err != nil || session == nil {
		return nil, err
	}

	messages, err := p.loadMessages(session.ID)
	if err != nil {
		return nil, err
	}

	if len(messages) == 0 {
		return nil, nil
	}

	turns := p.messagesToTurns(messages)
	if len(turns) == 0 {
		return nil, nil
	}

	result := &conv.Session{
		ID:           session.ID,
		Agent:        conv.AgentOpenCode,
		AgentVersion: session.Version,
		SourcePath:   sourcePath,
		ProjectPath:  session.Directory,
		ProjectName:  filepath.Base(session.Directory),
		Turns:        turns,
	}

	if created, ok := parseOpenCodeTime(session.Time.Created); ok {
		result.StartedAt = created
	}
	if updated, ok := parseOpenCodeTime(session.Time.Updated); ok {
		result.EndedAt = updated
	}

	if result.StartedAt.IsZero() {
		result.StartedAt = turns[0].Timestamp
	}
	if result.EndedAt.IsZero() {
		result.EndedAt = turns[len(turns)-1].Timestamp
	}

	return []*conv.Session{result}, nil
}

func (p *OpenCodeParser) loadSession(sourcePath string) (*opencodeSession, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, err
	}

	var session opencodeSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	if session.ID == "" {
		return nil, nil
	}
	return &session, nil
}

type opencodeMessageEntry struct {
	message   opencodeMessage
	content   string
	timestamp time.Time
}

func (p *OpenCodeParser) loadMessages(sessionID string) ([]opencodeMessageEntry, error) {
	messagesDir := filepath.Join(p.basePath, "message", sessionID)
	entries, err := os.ReadDir(messagesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var messages []opencodeMessageEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(messagesDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var msg opencodeMessage
		if err := json.Unmarshal(data, &msg); err != nil || msg.ID == "" {
			continue
		}

		content := p.loadMessageContent(msg.ID)
		timestamp := p.messageTimestamp(msg, path)

		messages = append(messages, opencodeMessageEntry{
			message:   msg,
			content:   content,
			timestamp: timestamp,
		})
	}

	sort.Slice(messages, func(i, j int) bool {
		if !messages[i].timestamp.IsZero() && !messages[j].timestamp.IsZero() {
			return messages[i].timestamp.Before(messages[j].timestamp)
		}
		return messages[i].message.ID < messages[j].message.ID
	})

	return messages, nil
}

func (p *OpenCodeParser) messageTimestamp(msg opencodeMessage, path string) time.Time {
	if ts, ok := parseOpenCodeTime(msg.Time.Created); ok {
		return ts
	}
	if ts, ok := parseOpenCodeTime(msg.Time.Completed); ok {
		return ts
	}
	info, err := os.Stat(path)
	if err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

func (p *OpenCodeParser) loadMessageContent(messageID string) string {
	partsDir := filepath.Join(p.basePath, "part", messageID)
	entries, err := os.ReadDir(partsDir)
	if err != nil {
		return ""
	}

	var parts []opencodePart
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(partsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		text, partType, created := parseOpenCodePart(data)
		if text == "" || !shouldIncludeOpenCodePart(partType) {
			continue
		}

		parts = append(parts, opencodePart{
			Type:    partType,
			Text:    text,
			Created: created,
			Name:    entry.Name(),
		})
	}

	if len(parts) == 0 {
		return ""
	}

	sort.Slice(parts, func(i, j int) bool {
		if !parts[i].Created.IsZero() && !parts[j].Created.IsZero() {
			return parts[i].Created.Before(parts[j].Created)
		}
		return parts[i].Name < parts[j].Name
	})

	var snippets []string
	for _, part := range parts {
		if part.Text != "" {
			snippets = append(snippets, part.Text)
		}
	}
	return strings.Join(snippets, "\n")
}

func (p *OpenCodeParser) messagesToTurns(messages []opencodeMessageEntry) []conv.Turn {
	var turns []conv.Turn
	var currentTurn *conv.Turn
	turnIndex := 0

	for _, msg := range messages {
		switch msg.message.Role {
		case "user":
			if currentTurn != nil && currentTurn.UserContent != "" {
				turns = append(turns, *currentTurn)
				turnIndex++
			}
			currentTurn = &conv.Turn{
				Index:       turnIndex,
				UserContent: msg.content,
				Timestamp:   msg.timestamp,
			}
		case "assistant":
			if currentTurn == nil {
				currentTurn = &conv.Turn{
					Index:     turnIndex,
					Timestamp: msg.timestamp,
				}
			}
			currentTurn.AssistContent = msg.content
			currentTurn.HasCode = containsCode(msg.content)
			currentTurn.CodeLangs = detectCodeLanguages(msg.content)
			if currentTurn.Timestamp.IsZero() {
				currentTurn.Timestamp = msg.timestamp
			}

			turns = append(turns, *currentTurn)
			currentTurn = nil
			turnIndex++
		}
	}

	if currentTurn != nil && (currentTurn.UserContent != "" || currentTurn.AssistContent != "") {
		turns = append(turns, *currentTurn)
	}

	return turns
}

func parseOpenCodePart(data []byte) (string, string, time.Time) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", "", time.Time{}
	}

	partType, _ := raw["type"].(string)
	text := extractOpenCodeText(raw)
	created := parseOpenCodeTimeField(raw, "time", "created")

	return text, partType, created
}

func extractOpenCodeText(raw map[string]any) string {
	keys := []string{"text", "content", "value", "patch", "diff"}
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
				return text
			}
		}
	}

	if data, ok := raw["data"].(map[string]any); ok {
		for _, key := range keys {
			if value, ok := data[key]; ok {
				if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
					return text
				}
			}
		}
	}

	return ""
}

func parseOpenCodeTimeField(raw map[string]any, path ...string) time.Time {
	current := any(raw)
	for _, key := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			return time.Time{}
		}
		value, ok := obj[key]
		if !ok {
			return time.Time{}
		}
		current = value
	}
	if text, ok := current.(string); ok {
		if parsed, ok := parseOpenCodeTime(text); ok {
			return parsed
		}
	}
	return time.Time{}
}

func parseOpenCodeTime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	if ts, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return ts, true
	}
	if ts, err := time.Parse(time.RFC3339, value); err == nil {
		return ts, true
	}
	return time.Time{}, false
}

func shouldIncludeOpenCodePart(partType string) bool {
	switch partType {
	case "text", "reasoning", "file", "patch":
		return true
	default:
		return false
	}
}

// RegisterOpenCode registers the OpenCode parser in the registry.
func RegisterOpenCode() {
	Register(NewOpenCodeParser())
}
