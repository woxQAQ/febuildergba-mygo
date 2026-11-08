package rom_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func TestSavedContentAfterManualRevert(t *testing.T) {
	original := []byte{0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80}
	doc := openContentDocument(t, original)
	initial := doc.Snapshot()

	applyContent(t, doc, "first edit", 2, []byte{0xAA, 0xBB, 0xCC})
	assertContentState(
		t,
		doc,
		[]byte{0x10, 0x20, 0xAA, 0xBB, 0xCC, 0x60, 0x70, 0x80},
		true,
	)
	applyContent(t, doc, "overlapping edit", 3, []byte{0xDD, 0xEE})
	applyContent(t, doc, "restore part", 2, original[2:4])
	assertContentState(
		t,
		doc,
		[]byte{0x10, 0x20, 0x30, 0x40, 0xEE, 0x60, 0x70, 0x80},
		true,
	)
	applyContent(t, doc, "restore remainder", 4, original[4:5])
	assertContentState(t, doc, original, false)
	state := doc.State()
	if !state.CanUndo || state.CanRedo ||
		state.UndoLabel != "restore remainder" ||
		state.Revision <= initial.Revision {
		t.Fatalf("restoring saved bytes lost edit history: %+v", state)
	}
	if err := doc.Apply(
		initial.Revision,
		"stale",
		[]rom.Change{{Offset: 0, Data: []byte{0xFF}}},
	); !errors.Is(
		err,
		rom.ErrStale,
	) {
		t.Fatalf("restoring bytes accepted an outdated edit: %v", err)
	}
	if !doc.Undo() {
		t.Fatal("cannot undo manual restoration")
	}
	assertContentState(
		t,
		doc,
		[]byte{0x10, 0x20, 0x30, 0x40, 0xEE, 0x60, 0x70, 0x80},
		true,
	)
	if !doc.Redo() {
		t.Fatal("cannot redo manual restoration")
	}
	assertContentState(t, doc, original, false)
	if !doc.Undo() {
		t.Fatal("cannot undo before branching")
	}
	applyContent(t, doc, "restore on new branch", 4, original[4:5])
	assertContentState(t, doc, original, false)
	if doc.State().CanRedo {
		t.Fatal("discarded history survived the new branch")
	}
	if err := doc.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(
		doc.State().Path + ".backups",
	); !errors.Is(
		err,
		os.ErrNotExist,
	) {
		t.Fatalf("unchanged content created a backup: %v", err)
	}
}

func TestSavedContentFollowsSuccessfulSave(t *testing.T) {
	for _, saveAs := range []bool{false, true} {
		name := "save"
		if saveAs {
			name = "save as"
		}
		t.Run(name, func(t *testing.T) {
			original := []byte{0x10, 0x20, 0x30, 0x40}
			saved := []byte{0x10, 0xAA, 0x30, 0x40}
			doc := openContentDocument(t, original)
			applyContent(t, doc, "saved edit", 1, []byte{0xAA})
			var err error
			if saveAs {
				err = doc.SaveAs(filepath.Join(t.TempDir(), "copy.gba"), false)
			} else {
				err = doc.Save()
			}
			if err != nil {
				t.Fatal(err)
			}
			assertContentState(t, doc, saved, false)
			applyContent(t, doc, "temporary edit", 1, []byte{0xBB})
			assertContentState(t, doc, []byte{0x10, 0xBB, 0x30, 0x40}, true)
			applyContent(t, doc, "restore saved value", 1, []byte{0xAA})
			assertContentState(t, doc, saved, false)
			applyContent(t, doc, "restore original value", 1, original[1:2])
			assertContentState(t, doc, original, true)
			if !doc.Undo() {
				t.Fatal("cannot return to saved content")
			}
			assertContentState(t, doc, saved, false)
			if err := doc.Save(); err != nil {
				t.Fatal(err)
			}
			if !doc.Redo() {
				t.Fatal("saving discarded redo history")
			}
			assertContentState(t, doc, original, true)
			reopened, err := rom.Open(doc.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			assertContentState(t, reopened, saved, false)
		})
	}
}

func TestSavedContentWithExpansionAndHistoryBranch(t *testing.T) {
	original := []byte{0x10, 0x20, 0x30, 0x40}
	expanded := []byte{0x10, 0x20, 0xAA, 0xBB, 0xCC, 0xDD, 0xFF, 0xFF}
	doc := openContentDocument(t, original)
	if err := doc.ApplyEdit(doc.State().Revision, "expand and write", rom.Edit{
		Size: 8,
		Changes: []rom.Change{
			{Offset: 2, Data: []byte{0xAA, 0xBB, 0xCC, 0xDD}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	assertContentState(t, doc, expanded, true)
	if err := doc.Save(); err != nil {
		t.Fatal(err)
	}
	assertContentState(t, doc, expanded, false)
	if !doc.Undo() {
		t.Fatal("cannot undo saved expansion")
	}
	assertContentState(t, doc, original, true)
	if err := doc.ApplyEdit(
		doc.State().Revision,
		"expand on new branch",
		rom.Edit{Size: 8},
	); err != nil {
		t.Fatal(err)
	}
	assertContentState(
		t,
		doc,
		[]byte{0x10, 0x20, 0x30, 0x40, 0xFF, 0xFF, 0xFF, 0xFF},
		true,
	)
	applyContent(t, doc, "restore expanded content", 2, expanded[2:6])
	assertContentState(t, doc, expanded, false)
	if !doc.Undo() {
		t.Fatal("cannot undo content restoration")
	}
	if !doc.Undo() {
		t.Fatal("cannot undo the new expansion branch")
	}
	assertContentState(t, doc, original, true)
	if err := doc.Save(); err != nil {
		t.Fatal(err)
	}
	assertContentState(t, doc, original, false)
	if !doc.Redo() {
		t.Fatal("cannot redo expansion after saving shorter content")
	}
	assertContentState(
		t,
		doc,
		[]byte{0x10, 0x20, 0x30, 0x40, 0xFF, 0xFF, 0xFF, 0xFF},
		true,
	)
	if !doc.Undo() {
		t.Fatal("cannot undo expansion to shorter checkpoint")
	}
	assertContentState(t, doc, original, false)
}

func TestSavedContentWithConcurrentEditing(t *testing.T) {
	doc := openContentDocument(t, bytes.Repeat([]byte{0x10}, 256<<10))
	edits := make(chan error, 1)
	go func() {
		for i := range 128 {
			if err := doc.Apply(
				doc.State().Revision,
				"concurrent edit",
				[]rom.Change{
					{Offset: i, Data: []byte{byte(i % 2)}},
				},
			); err != nil {
				edits <- err
				return
			}
		}
		edits <- nil
	}()
	for range 8 {
		if err := doc.Save(); err != nil {
			t.Error(err)
		}
	}
	if err := <-edits; err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(doc.State().Path)
	if err != nil {
		t.Fatal(err)
	}
	current := doc.Snapshot().Data
	assertContentState(t, doc, current, !bytes.Equal(current, saved))
	applyContent(t, doc, "restore actual saved snapshot", 0, saved)
	assertContentState(t, doc, saved, false)
}

func openContentDocument(t *testing.T, data []byte) *rom.Document {
	t.Helper()
	path := filepath.Join(t.TempDir(), "content.gba")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	doc, err := rom.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func applyContent(
	t *testing.T,
	doc *rom.Document,
	label string,
	offset int,
	data []byte,
) {
	t.Helper()
	if err := doc.Apply(
		doc.State().Revision,
		label,
		[]rom.Change{{Offset: offset, Data: data}},
	); err != nil {
		t.Fatal(err)
	}
}

func assertContentState(
	t *testing.T,
	doc *rom.Document,
	want []byte,
	dirty bool,
) {
	t.Helper()
	state := doc.State()
	if state.Dirty != dirty || state.Size != len(want) ||
		!bytes.Equal(doc.Snapshot().Data, want) {
		t.Fatalf(
			"content = %x, state = %+v; want %x, dirty = %v",
			doc.Snapshot().Data,
			state,
			want,
			dirty,
		)
	}
}
