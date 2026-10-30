//go:build integration

package integration

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// Each discovered input runs independently; no particular release or filename
// is required. Every workflow receives a fresh copy in its own temporary directory.
func TestROMWorkflows(t *testing.T) {
	const directory = "../../.rom"
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("no local .rom directory")
	}
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range entries {
		if entry.IsDir() ||
			!strings.EqualFold(filepath.Ext(entry.Name()), ".gba") {
			continue
		}
		found = true
		t.Run(entry.Name(), func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(directory, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			workflows := []struct {
				name string
				run  func(*testing.T, *game.Session)
			}{
				{"save", unchangedSave},
				{"records", editRecords},
				{"relocation", relocateRecords},
				{"text", editText},
				{"portraits", editPortrait},
				{"png", replacePNG},
				{"maps", editMap},
				{"map-resize", resizeMap},
				{"map-changes", editMapChanges},
				{"deployments", editDeployment},
				{
					"turns",
					func(t *testing.T, s *game.Session) { editEvents(t, s, event.TurnEvents) },
				},
				{
					"always",
					func(t *testing.T, s *game.Session) { editEvents(t, s, event.AlwaysEvents) },
				},
				{
					"talks",
					func(t *testing.T, s *game.Session) { editEvents(t, s, event.TalkEvents) },
				},
				{
					"locations",
					func(t *testing.T, s *game.Session) { editEvents(t, s, event.LocationEvents) },
				},
				{"scripts", editScript},
				{"script-scenes", editScriptScene},
				{"script-focus", editScriptFocus},
				{"script-loads", editScriptLoads},
				{"script-control", editScriptControl},
				{"script-presentation", editScriptPresentation},
				{"script-routes", editScriptRoutes},
				{"script-movement", editScriptMovement},
				{"script-map-changes", linkMapChange},
				{"script-unit-conditions", editScriptUnitConditions},
			}
			// Identify the input once, so an unsupported header has one explicit skip.
			probe := openCopy(t, source)
			t.Logf("adapter=%s, bytes=%d", probe.Version(), len(source))
			for _, workflow := range workflows {
				t.Run(workflow.name, func(t *testing.T) {
					workflow.run(t, openCopy(t, source))
				})
			}
		})
	}
	if !found {
		t.Skip("no .gba files in .rom")
	}
}

func openCopy(t *testing.T, data []byte) *game.Session {
	t.Helper()
	path := filepath.Join(t.TempDir(), "copy.gba")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := game.Open(path)
	if errors.Is(err, edit.ErrVersion) {
		t.Skipf("unsupported ROM header: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func requireTable(
	t *testing.T,
	s *game.Session,
	kind records.Kind,
) *records.Table {
	t.Helper()
	table := s.Records.Table(kind)
	if table == nil {
		t.Skipf("no %s table", kind)
	}
	if table.Err != nil {
		t.Skipf("%s layout unavailable: %v", kind, table.Err)
	}
	if len(table.Rows) == 0 {
		t.Skipf("%s table is empty", kind)
	}
	return table
}

// Discovery may find unsupported resources. Once an editable candidate is
// selected, unexpected planning or application errors are failures, not skips.
func checkPlan(t *testing.T, s *game.Session, before rom.Snapshot, err error) {
	t.Helper()
	current := s.Document.Snapshot()
	if current.Revision != before.Revision ||
		!bytes.Equal(current.Data, before.Data) {
		t.Fatal("preview changed the document")
	}
	if errors.Is(err, space.ErrCapacity) ||
		errors.Is(err, graphics.ErrFormat) {
		t.Skipf("operation unavailable for this input: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func historyAndReopen(
	t *testing.T,
	s *game.Session,
	before rom.Snapshot,
) *game.Session {
	t.Helper()
	after := s.Document.Snapshot()
	if bytes.Equal(before.Data, after.Data) || !s.Document.State().Dirty {
		t.Fatal("edit did not change the document")
	}
	if !s.Document.Undo() ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
		t.Fatal("undo did not restore the complete input")
	}
	s.Refresh()
	if !s.Document.Redo() ||
		!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
		t.Fatal("redo did not restore the complete edit")
	}
	s.Refresh()
	return saveAndReopen(t, s, after.Data)
}

func saveAndReopen(
	t *testing.T,
	s *game.Session,
	expected []byte,
) *game.Session {
	t.Helper()
	if err := s.Document.Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.Document.State().Path)
	if err != nil || !bytes.Equal(data, expected) {
		t.Fatalf("saved bytes differ: %v", err)
	}
	reopened, err := game.Open(s.Document.State().Path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Version() != s.Version() ||
		reopened.Document.State().Dirty ||
		!bytes.Equal(reopened.Document.Snapshot().Data, expected) {
		t.Fatal("reopen changed bytes, version or saved state")
	}
	return reopened
}

func unchangedSave(t *testing.T, s *game.Session) {
	before := s.Document.Snapshot()
	reopened := saveAndReopen(t, s, before.Data)
	if reopened.Document.State().CanUndo {
		t.Fatal("opening an unchanged file created edit history")
	}
}
