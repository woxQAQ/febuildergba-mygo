package rom

import (
	"bytes"
	"errors"
	"testing"
)

func TestExpandedEditHistory(t *testing.T) {
	doc := newTestDocument(t, 16)
	before := doc.Snapshot()
	edit := Edit{Size: 24, Changes: []Change{
		{Offset: 4, Data: []byte{0x20, 0, 0, 8}},
		{Offset: 14, Data: []byte{1, 2, 3, 4}}, // Crosses the old end of file.
		{Offset: 20, Data: []byte{5, 6}},
	}}
	if err := doc.ApplyEdit(before.Revision, "move", edit); err != nil {
		t.Fatal(err)
	}
	after := doc.Snapshot()
	if len(after.Data) != 24 || !bytes.Equal(after.Data[14:20], []byte{1, 2, 3, 4, 0xFF, 0xFF}) ||
		!bytes.Equal(after.Data[20:], []byte{5, 6, 0xFF, 0xFF}) {
		t.Fatalf("expanded bytes: %x", after.Data)
	}
	// History owns its bytes, independently of the caller's buffers.
	edit.Changes[0].Data[0] = 0xFF
	if err := doc.Save(); err != nil {
		t.Fatal(err)
	}
	if !doc.Undo() || !bytes.Equal(before.Data, doc.Snapshot().Data) || !doc.State().Dirty {
		t.Fatal("undo lost original bytes, size or saved state")
	}
	if !doc.Redo() || !bytes.Equal(after.Data, doc.Snapshot().Data) || doc.State().Dirty {
		t.Fatal("redo lost expansion or saved state")
	}
	if !doc.Undo() {
		t.Fatal("cannot undo again")
	}
	if err := doc.Apply(doc.State().Revision, "branch", []Change{{Offset: 1, Data: []byte{9}}}); err != nil {
		t.Fatal(err)
	}
	if doc.State().CanRedo || doc.State().Size != len(before.Data) {
		t.Fatal("discarded expansion survived history branch")
	}
}

func TestInvalidExpansionIsAtomic(t *testing.T) {
	doc := newTestDocument(t, 16)
	before := doc.Snapshot()
	for _, edit := range []Edit{
		{Size: 15},
		{Size: MaxSize + 1},
		{Size: 24, Changes: []Change{{Offset: 4, Data: []byte{1}}, {Offset: 24, Data: []byte{2}}}},
		{Size: 24, Changes: []Change{{Offset: 16, Data: []byte{1, 2}}, {Offset: 17, Data: []byte{3}}}},
	} {
		if err := doc.ApplyEdit(before.Revision, "invalid", edit); err == nil {
			t.Fatal("invalid edit accepted")
		}
		if doc.State().Revision != before.Revision || doc.State().CanUndo || doc.State().Dirty || !bytes.Equal(before.Data, doc.Snapshot().Data) {
			t.Fatal("failed edit changed bytes, size or history")
		}
	}
	if err := doc.ApplyEdit(before.Revision+1, "stale", Edit{Size: 24}); !errors.Is(err, ErrStale) {
		t.Fatalf("stale expansion: %v", err)
	}
	if err := doc.ApplyEdit(before.Revision, "resize", Edit{Size: 24}); err != nil {
		t.Fatal(err)
	}
	if !doc.Undo() || !bytes.Equal(before.Data, doc.Snapshot().Data) {
		t.Fatal("expansion without writes did not undo")
	}
}
