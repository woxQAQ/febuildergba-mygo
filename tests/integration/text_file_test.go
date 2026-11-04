//go:build integration

package integration

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
)

func editTextFile(t *testing.T, s *game.Session) {
	entries, err := s.Texts.Entries()
	if err != nil {
		t.Skipf("text table unavailable: %v", err)
	}
	file, err := s.Texts.File()
	if err != nil {
		t.Fatal(err)
	}
	var ids []uint16
	var contents, bodies []string
	for _, entry := range entries {
		if entry.Err != nil || !entry.Editable || entry.Content == "" ||
			strings.ContainsAny(entry.Content, "[\\\n\r") ||
			len(entry.Content) > 1024 {
			continue
		}
		encoded, err := file.Export([]uint16{entry.ID})
		if err != nil {
			t.Fatal(err)
		}
		body := strings.Split(string(encoded), "\r\n")[1]
		candidate, err := file.Import(
			[]byte(fmt.Sprintf("[%04X]\n%s%s\n", entry.ID, body, body)),
		)
		if err != nil {
			t.Fatal(err)
		}
		if row := candidate.Entries()[0]; row.Err != nil {
			if errors.Is(row.Err, text.ErrTextEncoding) {
				continue
			}
			t.Fatal(row.Err)
		}
		ids, contents, bodies = append(
			ids,
			entry.ID,
		), append(
			contents,
			entry.Content+entry.Content,
		), append(
			bodies,
			body,
		)
		if len(ids) == 2 {
			break
		}
	}
	if len(ids) < 2 {
		t.Skip("fewer than two readable editable plain-text IDs")
	}
	original, err := file.Export(ids)
	if err != nil {
		t.Fatal(err)
	}
	exportPath := filepath.Join(t.TempDir(), "export.txt")
	if err := os.WriteFile(exportPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	var edited strings.Builder
	for index, id := range ids {
		body := bodies[index]
		fmt.Fprintf(&edited, "[%04X]\r\n%s%s\r\n", id, body, body)
	}
	if err := os.WriteFile(
		exportPath,
		[]byte(edited.String()),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	input, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := file.Import(input)
	if err != nil {
		t.Fatal(err)
	}
	before := s.Document.Snapshot()
	plan, err := s.Texts.PlanImport(draft)
	checkPlan(t, s, before, err)
	if err := s.Texts.ApplyImport(plan); err != nil {
		t.Fatal(err)
	}
	reopened := historyAndReopen(t, s, before)
	for index, id := range ids {
		value, err := reopened.Texts.Read(id)
		if err != nil || value.Content != contents[index] {
			t.Fatalf("text %04X after reopen: %v", id, err)
		}
	}
	loaded, err := reopened.Texts.File()
	if err != nil {
		t.Fatal(err)
	}
	output, err := loaded.Export(ids)
	if err != nil || len(output) == 0 {
		t.Fatal("edited file export", err)
	}
	unchanged, err := loaded.Import(output)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range unchanged.Entries() {
		if entry.Changed || entry.Err != nil {
			t.Fatal("saved file changed on reimport", entry.ID, entry.Err)
		}
	}
	if len(output) > text.MaxFileSize {
		t.Fatal("export exceeded file limit")
	}
	t.Logf(
		"edited existing text IDs %04X and %04X in one file operation",
		ids[0],
		ids[1],
	)
}
