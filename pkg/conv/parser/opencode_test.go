package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/XiaoConstantine/sgrep/pkg/conv"
)

func TestOpenCodeParser_ParseSession(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "storage")

	sessionDir := filepath.Join(basePath, "session", "prj_test")
	messageDir := filepath.Join(basePath, "message", "ses_test")
	userPartsDir := filepath.Join(basePath, "part", "msg_user")
	assistPartsDir := filepath.Join(basePath, "part", "msg_assist")

	for _, dir := range []string{sessionDir, messageDir, userPartsDir, assistPartsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
	}

	now := time.Now().UTC()
	userTime := now.Format(time.RFC3339Nano)
	assistTime := now.Add(time.Second).Format(time.RFC3339Nano)

	session := opencodeSession{
		ID:        "ses_test",
		ProjectID: "prj_test",
		Version:   "0.14.6",
		Directory: "/path/to/project",
	}
	session.Time.Created = userTime
	session.Time.Updated = assistTime
	sessionData, _ := json.Marshal(session)

	sessionPath := filepath.Join(sessionDir, "ses_test.json")
	if err := os.WriteFile(sessionPath, sessionData, 0644); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	userMsg := opencodeMessage{
		ID:        "msg_user",
		SessionID: "ses_test",
		Role:      "user",
	}
	userMsg.Time.Created = userTime
	userData, _ := json.Marshal(userMsg)
	if err := os.WriteFile(filepath.Join(messageDir, "msg_user.json"), userData, 0644); err != nil {
		t.Fatalf("failed to write user message: %v", err)
	}

	assistMsg := opencodeMessage{
		ID:        "msg_assist",
		SessionID: "ses_test",
		Role:      "assistant",
	}
	assistMsg.Time.Created = assistTime
	assistData, _ := json.Marshal(assistMsg)
	if err := os.WriteFile(filepath.Join(messageDir, "msg_assist.json"), assistData, 0644); err != nil {
		t.Fatalf("failed to write assistant message: %v", err)
	}

	userPart := map[string]any{
		"type": "text",
		"text": "Hello from user",
	}
	userPartData, _ := json.Marshal(userPart)
	if err := os.WriteFile(filepath.Join(userPartsDir, "part_1.json"), userPartData, 0644); err != nil {
		t.Fatalf("failed to write user part: %v", err)
	}

	assistPart := map[string]any{
		"type":    "text",
		"content": "Hello from assistant",
	}
	assistPartData, _ := json.Marshal(assistPart)
	if err := os.WriteFile(filepath.Join(assistPartsDir, "part_1.json"), assistPartData, 0644); err != nil {
		t.Fatalf("failed to write assistant part: %v", err)
	}

	parser := NewOpenCodeParserWithPath(basePath)
	sessions, err := parser.Parse(sessionPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Agent != conv.AgentOpenCode {
		t.Fatalf("expected agent %s, got %s", conv.AgentOpenCode, sessions[0].Agent)
	}
	if len(sessions[0].Turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(sessions[0].Turns))
	}
	if sessions[0].Turns[0].UserContent != "Hello from user" {
		t.Errorf("unexpected user content: %q", sessions[0].Turns[0].UserContent)
	}
	if sessions[0].Turns[0].AssistContent != "Hello from assistant" {
		t.Errorf("unexpected assistant content: %q", sessions[0].Turns[0].AssistContent)
	}
}

func TestOpenCodeParser_Discover(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "storage")
	sessionDir := filepath.Join(basePath, "session", "prj_test")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("failed to create session dir: %v", err)
	}

	sessionPath := filepath.Join(sessionDir, "ses_test.json")
	if err := os.WriteFile(sessionPath, []byte(`{"id":"ses_test"}`), 0644); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	parser := NewOpenCodeParserWithPath(basePath)
	paths, err := parser.Discover()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
}
