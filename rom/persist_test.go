package rom_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

// These existing file-workflow checks use the production document save path.
// Session editing and version coverage live beside the game package.
func TestDocument_PersistAfterSaveAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.gba")
	if err := testutil.NewMinimalFE8U().Save(path); err != nil {
		t.Fatal(err)
	}
	doc, err := rom.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Apply(
		doc.State().Revision,
		"edit",
		[]rom.Change{{Offset: 0x200, Data: []byte{0xAA, 0xBB, 0xCC}}},
	); err != nil {
		t.Fatal(err)
	}
	if err := doc.Save(); err != nil {
		t.Fatal(err)
	}
	reopened, err := rom.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(doc.Snapshot().Data, reopened.Snapshot().Data) ||
		doc.State().Dirty {
		t.Fatal("save and reopen lost document contents or saved state")
	}
}

func TestDocument_UndoBeforeSaveDoesNotPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.gba")
	if err := testutil.NewMinimalFE8U().Save(path); err != nil {
		t.Fatal(err)
	}
	doc, err := rom.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	original := doc.Snapshot().Data
	if err := doc.Apply(
		doc.State().Revision,
		"temporary edit",
		[]rom.Change{{Offset: 0x200, Data: []byte{0xFF}}},
	); err != nil {
		t.Fatal(err)
	}
	if !doc.Undo() {
		t.Fatal("cannot undo")
	}
	if err := doc.Save(); err != nil {
		t.Fatal(err)
	}
	reopened, err := rom.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, reopened.Snapshot().Data) {
		t.Fatal("undone bytes persisted")
	}
}

func TestDocument_DetectsExternalChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.gba")
	if err := testutil.NewMinimalFE8U().Save(path); err != nil {
		t.Fatal(err)
	}
	doc, err := rom.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := doc.ExternalChanged(); err != nil || changed {
		t.Fatalf("new document reports external change: %v, %v", changed, err)
	}
	data := doc.Snapshot().Data
	data[0x200] = 0x99
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, err := doc.ExternalChanged(); err != nil || !changed {
		t.Fatalf("external change missed: %v, %v", changed, err)
	}
}
