package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/XiaoConstantine/sgrep/pkg/conv"
)

func TestClaudeParser_AgentType(t *testing.T) {
	p := NewClaudeParser()
	if p.AgentType() != conv.AgentClaudeCode {
		t.Errorf("expected agent type %s, got %s", conv.AgentClaudeCode, p.AgentType())
	}
}

func TestClaudeParser_ParseEmptyFile(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	p := NewClaudeParserWithPath(tmpDir)
	sessions, err := p.Parse(testFile)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestClaudeParser_ParseSingleTurn(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	// Create JSONL with user and assistant messages using the claudeMessage format
	now := time.Now().Format(time.RFC3339)
	entries := []claudeMessage{
		{
			SessionID: "session-1",
			Version:   "1.0.0",
			CWD:       "/path/to/project",
			Timestamp: now,
			Message: struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			}{
				Role:    "user",
				Content: "How do I use Go?",
			},
		},
		{
			SessionID: "session-1",
			Version:   "1.0.0",
			CWD:       "/path/to/project",
			Timestamp: now,
			Message: struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			}{
				Role:    "assistant",
				Content: "Go is a statically typed language...",
			},
		},
	}

	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	enc := json.NewEncoder(f)
	for _, entry := range entries {
		if err := enc.Encode(entry); err != nil {
			t.Fatalf("failed to encode entry: %v", err)
		}
	}
	_ = f.Close()

	p := NewClaudeParserWithPath(tmpDir)
	sessions, err := p.Parse(testFile)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if len(sessions[0].Turns) != 1 {
		t.Errorf("expected 1 turn, got %d", len(sessions[0].Turns))
	}
	if sessions[0].Turns[0].UserContent != "How do I use Go?" {
		t.Errorf("unexpected user content: %s", sessions[0].Turns[0].UserContent)
	}
}

func TestClaudeParser_ParseMultipleSessions(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	now := time.Now().Format(time.RFC3339)
	entries := []claudeMessage{
		{
			SessionID: "session-1",
			Version:   "1.0.0",
			CWD:       "/path/to/project",
			Timestamp: now,
			Message: struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			}{Role: "user", Content: "Session 1 question"},
		},
		{
			SessionID: "session-1",
			Version:   "1.0.0",
			CWD:       "/path/to/project",
			Timestamp: now,
			Message: struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			}{Role: "assistant", Content: "Session 1 answer"},
		},
		{
			SessionID: "session-2",
			Version:   "1.0.0",
			CWD:       "/path/to/project2",
			Timestamp: now,
			Message: struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			}{Role: "user", Content: "Session 2 question"},
		},
		{
			SessionID: "session-2",
			Version:   "1.0.0",
			CWD:       "/path/to/project2",
			Timestamp: now,
			Message: struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			}{Role: "assistant", Content: "Session 2 answer"},
		},
	}

	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	enc := json.NewEncoder(f)
	for _, entry := range entries {
		if err := enc.Encode(entry); err != nil {
			t.Fatalf("failed to encode entry: %v", err)
		}
	}
	_ = f.Close()

	p := NewClaudeParserWithPath(tmpDir)
	sessions, err := p.Parse(testFile)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestClaudeParser_Discover(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory structure similar to Claude Code
	projectDir := filepath.Join(tmpDir, "projects", "myproject")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	// Create a JSONL file
	jsonlFile := filepath.Join(projectDir, "conversation.jsonl")
	if err := os.WriteFile(jsonlFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create jsonl file: %v", err)
	}

	p := NewClaudeParserWithPath(tmpDir)
	paths, err := p.Discover()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(paths) != 1 {
		t.Errorf("expected 1 path, got %d", len(paths))
	}
}

func TestExtractContent(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "simple string content",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name: "array content with text",
			input: []interface{}{
				map[string]interface{}{"type": "text", "text": "Array text"},
			},
			expected: "Array text",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "nil",
			input:    nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractContent(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
