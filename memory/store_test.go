package memory

import (
	"path/filepath"
	"testing"
)

func TestStoreSessionAndMessages(t *testing.T) {
	tempDir := t.TempDir()
	
	// Override the config dir by modifying environment or creating store directly
	storePath := filepath.Join(tempDir, "history.json")
	s := &Store{
		path: storePath,
		data: storeData{Context: map[string]string{}},
	}

	sess, err := s.NewSession("chat")
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("expected non-empty session ID")
	}

	if err := s.SetSessionTitle(sess.ID, "Test Session"); err != nil {
		t.Fatalf("SetSessionTitle failed: %v", err)
	}

	msg1 := Message{Role: "user", Content: "Hello world", SessionID: sess.ID}
	msg2 := Message{Role: "assistant", Content: "Hi there", SessionID: sess.ID}
	if err := s.SaveMessage(msg1); err != nil {
		t.Fatalf("SaveMessage 1 failed: %v", err)
	}
	if err := s.SaveMessage(msg2); err != nil {
		t.Fatalf("SaveMessage 2 failed: %v", err)
	}

	msgs, err := s.GetMessages(sess.ID, 10)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	count, err := s.CountMessages(sess.ID)
	if err != nil || count != 2 {
		t.Errorf("expected count 2, got %d (err: %v)", count, err)
	}

	export, err := s.ExportSession(sess.ID)
	if err != nil {
		t.Fatalf("ExportSession failed: %v", err)
	}
	if len(export) == 0 {
		t.Error("expected non-empty export string")
	}
}

func TestGenerateTitleHeuristic(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"this is a long sentence with more than eight words in it for testing", "this is a long sentence with more than"},
	}

	for _, tc := range tests {
		res := GenerateTitleHeuristic(tc.input)
		if res != tc.expected {
			t.Errorf("GenerateTitleHeuristic(%q) = %q; want %q", tc.input, res, tc.expected)
		}
	}
}
