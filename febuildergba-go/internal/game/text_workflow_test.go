package game

import (
	"bytes"
	"encoding/binary"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/rom"
	"github.com/woxQAQ/febuildergba-mygo/tests/testutil"
)

func textFixtureFor(version nameVersion) *testutil.FixtureBuilder {
	f := descriptionFixtureFor(version)
	f.WritePointer(nameTable, nameRaw) // ID 0 participates in the native table extent.
	return f
}

func TestTextIDWorkflow(t *testing.T) {
	for _, version := range nameVersions {
		t.Run(version.id, func(t *testing.T) {
			s := openFixture(t, textFixtureFor(version))
			entries, err := s.TextEntries()
			if err != nil || len(entries) != 6 || entries[3].ID != 4 || entries[3].Content != originalDescription {
				t.Fatalf("existing IDs were not indexed: %+v, %v", entries, err)
			}
			entries[3].Content = "changed copy"
			entries, _ = s.TextEntries()
			if entries[3].Content != originalDescription {
				t.Fatal("a caller changed the cached index")
			}
			before := s.Document.Snapshot()
			const content = "AB[ToggleRed][LoadFace][0x0041]\n[.]"
			edit, err := s.PlanTextIDEdit(4, content)
			if err != nil {
				t.Fatal(err)
			}
			if edit.TextID() != 4 || len(edit.Uses()) != 6 || s.Document.State().Revision != before.Revision {
				t.Fatal("preview missed shared fields or changed the document")
			}
			if err := s.ApplyTextEdit(edit); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			// Independent four-bit paths for leaves 3,6,7,8,1,4,5,end.
			want := []byte{0x6C, 0x1E, 0x28, 0x0A, 0, 0, 0, 0}
			if !bytes.Equal(after.Data[edit.Destination():], want) {
				t.Fatalf("control edit differs from the fixed bitstream: % X", after.Data[edit.Destination():])
			}
			prefix := bytes.Clone(before.Data)
			binary.LittleEndian.PutUint32(prefix[nameTable+16:], uint32(0x08000000+edit.Destination()))
			if !bytes.Equal(prefix, after.Data[:len(prefix)]) {
				t.Fatal("the edit changed old resources, aliases or unrelated bytes")
			}
			value, err := s.TextByID(4)
			if err != nil || value.Content != content || value.Raw != "AB[0x0080][0x0021][0x0010][0x0041]\n[0x001F]" ||
				!slices.Equal(value.ControlCodes(), []uint16{0x80, 0x21, 0x10, 0x41, 0x1F}) {
				t.Fatalf("readable commands, raw units or parameters changed: %+v, %v", value, err)
			}
			entries, _ = s.TextEntries()
			if entries[3].Content != content || entries[4].Content != originalDescription {
				t.Fatal("the index was stale or an independent ID alias changed")
			}
			if !s.Document.Undo() || !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("one undo did not restore bytes, pointer and length")
			}
			entries, _ = s.TextEntries()
			if entries[3].Content != originalDescription || !s.Document.Redo() {
				t.Fatal("history did not refresh the index or restore the edit")
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			value, err = reopened.TextByID(4)
			if err != nil || value.Content != content {
				t.Fatalf("saved text did not reopen: %v", err)
			}
			// Remove old instructions and insert a different complete extension.
			next, err := reopened.PlanTextIDEdit(4, "AB[Wink]")
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyTextEdit(next); err != nil {
				t.Fatal(err)
			}
			value, err = reopened.TextByID(4)
			if err != nil || value.Content != "AB[Wink]" || !slices.Equal(value.ControlCodes(), []uint16{0x80, 0x1F}) {
				t.Fatalf("could not add and remove instructions after reopening: %v", err)
			}
		})
	}
}

func TestTextIDRejectsInvalidOperations(t *testing.T) {
	s := openFixture(t, textFixtureFor(nameVersions[0]))
	before := s.Document.Snapshot()
	for _, id := range []uint16{0, 7, 0x7FFF, 0xFFFF} {
		if _, err := s.TextByID(id); !errors.Is(err, ErrText) {
			t.Fatalf("read outside the existing text table: %04X, %v", id, err)
		}
		if _, err := s.PlanTextIDEdit(id, "AB"); !errors.Is(err, ErrText) {
			t.Fatalf("wrote outside the existing text table: %04X, %v", id, err)
		}
	}
	for _, content := range []string{
		"[LoadFace]", "[LoadFace]A", "[LoadFace][Wink]", "[LoadFace][0x0000]", "[LoadFace][0x0100]",
		"[LoadFace][0x10000]", "[LoadFace][0xFFFF]", "[0x0080]", "[0x0080][0x0026]",
		"[0x001E]", "[0x0041]", "[Unknown]", "[NL", "[Wink]🙂", "\\", strings.Repeat("A", 4097),
	} {
		if _, err := s.PlanTextIDEdit(4, content); !errors.Is(err, ErrTextEncoding) {
			t.Fatalf("accepted invalid or unencodable text %q: %v", content, err)
		}
	}
	for _, content := range []string{originalDescription, "A[NL][0x2180][LoadFace][0x0141]B[.]"} {
		if _, err := s.PlanTextIDEdit(4, content); !errors.Is(err, ErrValue) {
			t.Fatalf("equivalent notation unnecessarily rewrote the text: %v", err)
		}
	}
	if !bytes.Equal(before.Data, s.Document.Snapshot().Data) || before.Revision != s.Document.State().Revision {
		t.Fatal("a rejected operation changed the document")
	}
	edit, err := s.PlanTextIDEdit(4, "AB[Wink]")
	if err != nil {
		t.Fatal(err)
	}
	other := openFixture(t, textFixtureFor(nameVersions[0]))
	if err := other.ApplyTextEdit(edit); !errors.Is(err, rom.ErrStale) || other.Document.State().Dirty {
		t.Fatal("another document accepted a text preview")
	}
	if err := s.Set(Units, []int{1}, "base.hp", 25); err != nil {
		t.Fatal(err)
	}
	latest := s.Document.Snapshot()
	if err := s.ApplyTextEdit(edit); !errors.Is(err, rom.ErrStale) || !bytes.Equal(latest.Data, s.Document.Snapshot().Data) {
		t.Fatal("a stale preview applied changes")
	}
}

func TestTextRecoveryIsReadOnly(t *testing.T) {
	f := textFixtureFor(nameVersions[0])
	const recovery = 0x15D48C // FE8U's TextForm recovery address.
	f.WriteAt(recovery, f.Bytes()[nameTable:nameTable+28])
	f.WriteU32(0xA2A0, 0)
	s := openFixture(t, f)
	before := s.Document.Snapshot()
	entries, err := s.TextEntries()
	if err != nil || len(entries) != 6 || entries[3].Editable {
		t.Fatalf("recovery table could not be browsed read-only: %+v, %v", entries, err)
	}
	value, err := s.TextByID(4)
	if err != nil || value.Content != originalDescription || value.Editable || !errors.Is(value.ReadOnlyReason, ErrText) {
		t.Fatalf("recovery data was not readable with its write restriction: %+v, %v", value, err)
	}
	if _, err := s.PlanTextIDEdit(4, "AB"); !errors.Is(err, ErrText) || !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
		t.Fatal("a missing primary pointer was treated as a writable recovery table")
	}
}

func TestTextIndexRetainsUnreadableEntries(t *testing.T) {
	for _, ram := range []bool{false, true} {
		f := textFixtureFor(nameVersions[0])
		if ram {
			f.WriteU32(nameTable+12, 0x02000001)
		} else {
			f.WriteAt(nameRaw, bytes.Repeat([]byte{0x1F}, 4096)) // Unterminated anti-Huffman text.
		}
		s := openFixture(t, f)
		entries, err := s.TextEntries()
		if err != nil || len(entries) != 6 || entries[2].Err == nil || entries[3].Content != originalDescription {
			t.Fatalf("a broken stream hid subsequent text IDs: %+v, %v", entries, err)
		}
		if _, err := s.PlanTextIDEdit(3, "AB"); !errors.Is(err, ErrText) || s.Document.State().Dirty {
			t.Fatal("an unreadable or RAM entry was replaced")
		}
	}
	f := textFixtureFor(nameVersions[0])
	f.WriteU32(nameTree+22*4, 0x80000026) // Unknown extended instruction.
	s := openFixture(t, f)
	value, err := s.TextByID(4)
	if err != nil || value.Editable || !errors.Is(value.ReadOnlyReason, ErrTextControl) ||
		!strings.Contains(value.Raw, "[0x0080][0x0026]") {
		t.Fatalf("unknown controls were not preserved with a reason: %+v, %v", value, err)
	}
	if _, err := s.PlanTextIDEdit(4, "AB"); !errors.Is(err, ErrTextControl) || s.Document.State().Dirty {
		t.Fatal("unknown source controls were discarded")
	}
}
