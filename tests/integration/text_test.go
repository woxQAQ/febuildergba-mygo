//go:build integration

package integration

import (
	"strings"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
)

func editText(t *testing.T, s *game.Session) {
	entries, err := s.Texts.Entries()
	if err != nil {
		t.Skipf("text table unavailable: %v", err)
	}
	for _, entry := range entries {
		if entry.Err != nil || !entry.Editable || entry.Content == "" ||
			strings.Contains(entry.Content, "[") {
			continue
		}
		// Reuse the input's characters instead of assuming an English/Japanese
		// string can be encoded by a particular ROM's font and Huffman tree.
		content := entry.Content + entry.Content
		t.Logf("text ID=%d", entry.ID)
		before := s.Document.Snapshot()
		edit, err := s.Texts.PlanID(entry.ID, content)
		checkPlan(t, s, before, err)
		if err := s.Texts.Apply(edit); err != nil {
			t.Fatal(err)
		}
		reopened := historyAndReopen(t, s, before)
		value, err := reopened.Texts.Read(entry.ID)
		if err != nil || value.Content != content {
			t.Fatalf("edited text after reopening: %v", err)
		}
		return
	}
	t.Skip("no editable nonempty plain text entry")
}
