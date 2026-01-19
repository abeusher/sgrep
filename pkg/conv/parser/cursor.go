package parser

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/XiaoConstantine/sgrep/pkg/conv"
)

// CursorParser parses Cursor IDE conversation files.
type CursorParser struct {
	basePath string
}

// cursorChatData represents the chat data structure in Cursor's SQLite database.
type cursorChatData struct {
	Tabs []cursorTab `json:"tabs"`
}

type cursorTab struct {
	TabID        string               `json:"tabId"`
	ChatTitle    string               `json:"chatTitle"`
	Bubbles      []cursorBubble       `json:"bubbles"`
	Conversation []cursorConversation `json:"conversation,omitempty"`
}

type cursorBubble struct {
	Type       string `json:"type"` // "user" or "ai"
	Text       string `json:"text"`
	RawText    string `json:"rawText,omitempty"`
	Selections []struct {
		URI string `json:"uri"`
	} `json:"selections,omitempty"`
}

type cursorConversation struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// NewCursorParser creates a new Cursor parser.
func NewCursorParser() *CursorParser {
	homeDir, _ := os.UserHomeDir()
	var basePath string

	switch runtime.GOOS {
	case "darwin":
		basePath = filepath.Join(homeDir, "Library", "Application Support", "Cursor", "User", "workspaceStorage")
	case "linux":
		basePath = filepath.Join(homeDir, ".config", "Cursor", "User", "workspaceStorage")
	case "windows":
		basePath = filepath.Join(homeDir, "AppData", "Roaming", "Cursor", "User", "workspaceStorage")
	default:
		basePath = filepath.Join(homeDir, ".cursor", "workspaceStorage")
	}

	return &CursorParser{basePath: basePath}
}

// NewCursorParserWithPath creates a parser with a custom base path.
func NewCursorParserWithPath(basePath string) *CursorParser {
	return &CursorParser{basePath: basePath}
}

// AgentType returns the agent type.
func (p *CursorParser) AgentType() conv.AgentType {
	return conv.AgentCursor
}

// DefaultPath returns the default path for Cursor conversations.
func (p *CursorParser) DefaultPath() string {
	return p.basePath
}

// Discover finds all Cursor conversation database files.
func (p *CursorParser) Discover() ([]string, error) {
	var paths []string

	if _, err := os.Stat(p.basePath); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(p.basePath)
	if err != nil {
		return nil, nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dbPath := filepath.Join(p.basePath, entry.Name(), "state.vscdb")
		if _, err := os.Stat(dbPath); err == nil {
			paths = append(paths, dbPath)
		}
	}

	return paths, nil
}

// Parse reads a Cursor SQLite database and returns sessions.
func (p *CursorParser) Parse(sourcePath string) ([]*conv.Session, error) {
	db, err := openCursorDB(sourcePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	// Query for chat data (legacy key)
	var chatDataJSON string
	err = db.QueryRow(`
		SELECT value FROM ItemTable
		WHERE key = 'workbench.panel.aichat.view.aichat.chatdata'
	`).Scan(&chatDataJSON)

	var chatData cursorChatData
	if err == nil && chatDataJSON != "" {
		if err := json.Unmarshal([]byte(chatDataJSON), &chatData); err != nil {
			return nil, err
		}
	} else if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Fall back to aiService keys used by newer Cursor versions
	if len(chatData.Tabs) == 0 {
		var promptsJSON string
		err = db.QueryRow(`SELECT value FROM ItemTable WHERE key = 'aiService.prompts'`).Scan(&promptsJSON)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		var generationsJSON string
		err = db.QueryRow(`SELECT value FROM ItemTable WHERE key = 'aiService.generations'`).Scan(&generationsJSON)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}

		chatData, err = parseCursorAIService(promptsJSON, generationsJSON)
		if err != nil {
			return nil, err
		}
	}

	// Also try to get workspace info for project path
	var workspacePath string
	_ = db.QueryRow(`
		SELECT value FROM ItemTable
		WHERE key = 'workbench.uri'
	`).Scan(&workspacePath)

	// Clean up workspace path (remove file:// prefix)
	workspacePath = strings.TrimPrefix(workspacePath, "file://")
	projectName := filepath.Base(workspacePath)
	if projectName == "" {
		projectName = filepath.Base(filepath.Dir(sourcePath))
	}

	// Get file modification time for approximate session time
	fileInfo, _ := os.Stat(sourcePath)
	var modTime time.Time
	if fileInfo != nil {
		modTime = fileInfo.ModTime()
	} else {
		modTime = time.Now()
	}

	var sessions []*conv.Session

	for _, tab := range chatData.Tabs {
		session := p.tabToSession(tab, sourcePath, workspacePath, projectName, modTime)
		if session != nil && len(session.Turns) > 0 {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

type cursorPrompt struct {
	Text string `json:"text"`
}

type cursorGeneration struct {
	TextDescription string `json:"textDescription"`
}

func parseCursorAIService(promptsJSON, generationsJSON string) (cursorChatData, error) {
	if promptsJSON == "" && generationsJSON == "" {
		return cursorChatData{}, nil
	}

	var prompts []cursorPrompt
	if promptsJSON != "" {
		if err := json.Unmarshal([]byte(promptsJSON), &prompts); err != nil {
			return cursorChatData{}, err
		}
	}

	var generations []cursorGeneration
	if generationsJSON != "" {
		if err := json.Unmarshal([]byte(generationsJSON), &generations); err != nil {
			return cursorChatData{}, err
		}
	}

	if len(prompts) == 0 && len(generations) == 0 {
		return cursorChatData{}, nil
	}

	var conversation []cursorConversation
	maxLen := len(prompts)
	if len(generations) > maxLen {
		maxLen = len(generations)
	}

	for i := 0; i < maxLen; i++ {
		if i < len(prompts) {
			text := strings.TrimSpace(prompts[i].Text)
			if text != "" {
				conversation = append(conversation, cursorConversation{
					Role:    "user",
					Content: text,
				})
			}
		}
		if i < len(generations) {
			text := strings.TrimSpace(generations[i].TextDescription)
			if text != "" {
				conversation = append(conversation, cursorConversation{
					Role:    "assistant",
					Content: text,
				})
			}
		}
	}

	if len(conversation) == 0 {
		return cursorChatData{}, nil
	}

	return cursorChatData{
		Tabs: []cursorTab{{
			TabID:        "cursor-aiservice",
			ChatTitle:    "Cursor AI Service",
			Conversation: conversation,
		}},
	}, nil
}

// tabToSession converts a Cursor tab to a Session.
func (p *CursorParser) tabToSession(tab cursorTab, sourcePath, projectPath, projectName string, modTime time.Time) *conv.Session {
	// Generate session ID from tab ID
	h := sha256.Sum256([]byte(sourcePath + tab.TabID))
	sessionID := fmt.Sprintf("cursor-%x", h[:6])

	session := &conv.Session{
		ID:          sessionID,
		Agent:       conv.AgentCursor,
		SourcePath:  sourcePath,
		ProjectPath: projectPath,
		ProjectName: projectName,
		StartedAt:   modTime.Add(-24 * time.Hour), // Approximate
		EndedAt:     modTime,
	}

	// First try bubbles format
	if len(tab.Bubbles) > 0 {
		session.Turns = p.bubblesToTurns(tab.Bubbles)
	}

	// Fall back to conversation format
	if len(session.Turns) == 0 && len(tab.Conversation) > 0 {
		session.Turns = p.conversationToTurns(tab.Conversation)
	}

	return session
}

// bubblesToTurns converts Cursor bubbles to turns.
func (p *CursorParser) bubblesToTurns(bubbles []cursorBubble) []conv.Turn {
	// Sort bubbles (they should already be in order)
	var turns []conv.Turn
	var currentTurn *conv.Turn
	turnIndex := 0

	for _, bubble := range bubbles {
		content := bubble.Text
		if bubble.RawText != "" {
			content = bubble.RawText
		}

		switch bubble.Type {
		case "user":
			if currentTurn != nil && currentTurn.UserContent != "" {
				turns = append(turns, *currentTurn)
				turnIndex++
			}
			currentTurn = &conv.Turn{
				Index:       turnIndex,
				UserContent: content,
			}

		case "ai":
			if currentTurn == nil {
				currentTurn = &conv.Turn{
					Index: turnIndex,
				}
			}
			currentTurn.AssistContent = content
			currentTurn.HasCode = containsCode(content)
			currentTurn.CodeLangs = detectCodeLanguages(content)

			turns = append(turns, *currentTurn)
			currentTurn = nil
			turnIndex++
		}
	}

	// Save incomplete turn
	if currentTurn != nil && currentTurn.UserContent != "" {
		turns = append(turns, *currentTurn)
	}

	return turns
}

// conversationToTurns converts Cursor conversation array to turns.
func (p *CursorParser) conversationToTurns(conversations []cursorConversation) []conv.Turn {
	var turns []conv.Turn
	var currentTurn *conv.Turn
	turnIndex := 0

	for _, msg := range conversations {
		switch msg.Role {
		case "user":
			if currentTurn != nil && currentTurn.UserContent != "" {
				turns = append(turns, *currentTurn)
				turnIndex++
			}
			currentTurn = &conv.Turn{
				Index:       turnIndex,
				UserContent: msg.Content,
			}

		case "assistant":
			if currentTurn == nil {
				currentTurn = &conv.Turn{
					Index: turnIndex,
				}
			}
			currentTurn.AssistContent = msg.Content
			currentTurn.HasCode = containsCode(msg.Content)
			currentTurn.CodeLangs = detectCodeLanguages(msg.Content)

			turns = append(turns, *currentTurn)
			currentTurn = nil
			turnIndex++
		}
	}

	if currentTurn != nil && currentTurn.UserContent != "" {
		turns = append(turns, *currentTurn)
	}

	return turns
}

// RegisterCursor registers the Cursor parser in the registry.
func RegisterCursor() {
	Register(NewCursorParser())
}
