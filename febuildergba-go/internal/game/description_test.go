package game

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/tests/testutil"
)

const (
	descriptionStream   = nameStream + 0x100
	originalDescription = "A\n[0x0080][0x0021][0x0010][0x0141]B[0x001F]"
	editedDescription   = "AB\n[0x0080][0x0021][0x0010][0x0141]A[0x001F]"
)

func descriptionFixture() *testutil.FixtureBuilder { return descriptionFixtureFor(nameVersions[0]) }

func descriptionFixtureFor(version nameVersion) *testutil.FixtureBuilder {
	f := nameFixtureFor(version)
	// Complete four-level tree. The fixed stream uses leaf indices
	// 1,4,6,7,8,9,2,5,0. In particular 0x0141 is one portrait argument,
	// not the visible character A followed by a newline.
	for i := range 15 {
		f.WriteU16(nameTree+i*4, uint16(i*2+1))
		f.WriteU16(nameTree+i*4+2, uint16(i*2+2))
	}
	for i, word := range []uint16{0, 'A', 'B', 0x4241, 1, 0x1F, 0x80, 0x21, 0x10, 0x0141, '[', ']', '\\', 0x1E, 0x26, ' '} {
		f.WriteU32(nameTree+(i+15)*4, 0x80000000|uint32(word))
	}
	f.WriteAt(descriptionStream, []byte{0x28, 0xE6, 0x91, 0xA4, 0, 0, 0, 0})
	f.WritePointer(nameTable+16, descriptionStream)
	f.WritePointer(nameTable+20, descriptionStream) // Independent ID aliases the original resource.
	f.WriteAt(nameRaw+16, []byte{'B', 1, 0x1F, 0})
	f.WriteU32(nameTable+24, 0x88000000|uint32(nameRaw+16))
	f.WriteAt(nameRaw+32, []byte{'A', 'B', 0})
	f.WriteU32(nameTable+4, 0x88000000|uint32(nameRaw+32))
	f.WriteU32(nameTable+8, 0x88000000|uint32(nameRaw+32))
	f.WriteU16(f.GetPosition("units")+2, 4)
	f.WriteU16(f.GetPosition("units")+version.unitStride, 4) // A name also shares the description's ID.
	f.WriteU16(f.GetPosition("units")+version.unitStride+2, 4)
	f.WriteU16(f.GetPosition("classes")+2, 4)
	f.WriteU16(f.GetPosition("classes")+version.classStride+2, 5)
	f.WriteU16(f.GetPosition("items")+2, 4)
	f.WriteU16(f.GetPosition("items")+4, 4)
	f.WriteU16(f.GetPosition("items")+version.itemStride+4, 6)
	return f
}

func TestDescriptionEdits(t *testing.T) {
	for _, sample := range []struct {
		kind  Kind
		id    int
		field string
	}{
		{Units, 1, "description_id"},
		{Classes, 0, "description_id"},
		{Items, 0, "description_id"},
		{Items, 0, "use_description_id"},
	} {
		t.Run(string(sample.kind)+"/"+sample.field, func(t *testing.T) {
			s := openFixture(t, descriptionFixture())
			original, err := s.Text(sample.kind, sample.id, sample.field)
			if err != nil || original.Content != originalDescription || !original.Editable ||
				!slices.Equal(original.ControlCodes(), []uint16{0x80, 0x21, 0x10, 0x0141, 0x1F}) {
				t.Fatalf("control boundaries were lost while reading: %+v, %v", original, err)
			}
			before := s.Document.Snapshot()
			edit, err := s.PlanTextEdit(sample.kind, sample.id, sample.field, editedDescription)
			if err != nil {
				t.Fatal(err)
			}
			wantUses := []TextUse{
				{Units, 1, "description_id"},
				{Units, 2, "name_id"},
				{Units, 2, "description_id"},
				{Classes, 0, "description_id"},
				{Items, 0, "description_id"},
				{Items, 0, "use_description_id"},
			}
			if edit.TextID() != 4 || !slices.Equal(edit.Uses(), wantUses) || s.Document.State().Revision != before.Revision {
				t.Fatal("preview missed shared fields or modified the document")
			}
			if err := s.ApplyTextEdit(edit); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(s.Document.Snapshot().Data[edit.Destination():], []byte{0x2C, 0xE6, 0x91, 0xA8, 0, 0, 0, 0}) {
				t.Fatal("description did not match the independent fixed bitstream")
			}
			value, err := s.Text(Items, 0, "use_description_id")
			if err != nil || value.Content != editedDescription || !slices.Equal(value.ControlCodes(), original.ControlCodes()) {
				t.Fatalf("shared content or 16-bit arguments changed: %v", err)
			}
			alias, err := s.Text(Classes, 1, "description_id")
			if err != nil || alias.Content != originalDescription ||
				!bytes.Equal(before.Data[descriptionStream:descriptionStream+8], s.Document.Snapshot().Data[descriptionStream:descriptionStream+8]) {
				t.Fatal("independent alias or original resource was overwritten")
			}
			if !s.Document.Undo() || !bytes.Equal(before.Data, s.Document.Snapshot().Data) || !s.Document.Redo() {
				t.Fatal("single undo/redo did not restore bytes, pointer and file length")
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			value, err = reopened.Text(sample.kind, sample.id, sample.field)
			if err != nil || value.Content != editedDescription || !slices.Equal(value.ControlCodes(), original.ControlCodes()) {
				t.Fatalf("saved description did not reopen losslessly: %v", err)
			}
			next, err := reopened.PlanTextEdit(sample.kind, sample.id, sample.field, value.Content+"\n")
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyTextEdit(next); err != nil {
				t.Fatal(err)
			}
			value, err = reopened.Text(sample.kind, sample.id, sample.field)
			if err != nil || value.Content != editedDescription+"\n" {
				t.Fatalf("could not continue editing after reopening: %v", err)
			}
		})
	}
}

func TestDescriptionRejectsLossyEdits(t *testing.T) {
	s := openFixture(t, descriptionFixture())
	before := s.Document.Snapshot()
	for _, content := range []string{
		"AB",
		strings.Replace(editedDescription, "[0x0141]", "[0x0041]", 1),
		strings.Replace(editedDescription, "[0x0021]", "!", 1),
		strings.Replace(editedDescription, "[0x0010][0x0141]", "[0x0141][0x0010]", 1),
		editedDescription + "[0x0000]",
		editedDescription + "[0x0100]",
		editedDescription + "[0x001F]",
		editedDescription + "[0x01]",
		editedDescription + "\\n",
		editedDescription + "é",
		editedDescription + "\x00",
		editedDescription + "\t",
		editedDescription + strings.Repeat("A", 4097),
	} {
		if _, err := s.PlanTextEdit(Units, 1, "description_id", content); !errors.Is(err, ErrTextEncoding) {
			t.Fatalf("accepted a lossy or invalid edit: %v", err)
		}
	}
	for _, content := range []string{originalDescription, strings.ReplaceAll(originalDescription, "\n", "\r\n")} {
		if _, err := s.PlanTextEdit(Units, 1, "description_id", content); !errors.Is(err, ErrValue) {
			t.Fatalf("unchanged text was unnecessarily reencoded: %v", err)
		}
	}
	if _, err := s.PlanTextEdit(Units, 1, "base.hp", "AB"); !errors.Is(err, ErrField) {
		t.Fatalf("numeric field accepted as a text ID: %v", err)
	}
	if _, err := s.PlanTextEdit(Items, 1, "description_id", "AB"); !errors.Is(err, ErrText) {
		t.Fatalf("reserved text ID zero accepted: %v", err)
	}
	if s.Document.State().Revision != before.Revision || s.Document.State().Dirty || !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
		t.Fatal("rejected input left document changes")
	}
}

func TestUnsupportedDescriptionControls(t *testing.T) {
	for _, sample := range []struct {
		label string
		leaf  int
		word  uint32
	}{
		{"unknown-command", 1, 0x1E},
		{"unknown-extension", 7, 0x26},
		{"missing-argument", 9, 0},
		{"unsupported-packed-portrait", 1, 0x4110},
	} {
		t.Run(sample.label, func(t *testing.T) {
			f := descriptionFixture()
			f.WriteU32(nameTree+(sample.leaf+15)*4, 0x80000000|sample.word)
			s := openFixture(t, f)
			value, err := s.Text(Units, 1, "description_id")
			if err != nil || value.Editable || value.Content == "" {
				t.Fatalf("unsupported controls were not retained for read-only display: %v", err)
			}
			if _, err := s.PlanTextEdit(Units, 1, "description_id", value.Content+"\n"); !errors.Is(err, ErrTextEncoding) || s.Document.State().Dirty {
				t.Fatalf("unsupported control semantics were rewritten: %v", err)
			}
		})
	}
}

func TestRawDescriptionAndLiteralEscapes(t *testing.T) {
	s := openFixture(t, descriptionFixture())
	value, err := s.Text(Items, 1, "use_description_id")
	if err != nil || value.Content != "B\n[0x001F]" || !value.Editable {
		t.Fatalf("anti-Huffman text did not preserve controls: %v", err)
	}
	const content = "\\[AB]\\\\\nB[0x001F]"
	edit, err := s.PlanTextEdit(Items, 1, "use_description_id", content)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyTextEdit(edit); err != nil {
		t.Fatal(err)
	}
	value, err = s.Text(Items, 1, "use_description_id")
	if err != nil || value.Content != content || !slices.Equal(value.ControlCodes(), []uint16{0x1F}) || u32(s.Document.Snapshot().Data, nameTable+24) >= 0x88000000 {
		t.Fatalf("literal escapes or raw-input conversion changed the text: %v", err)
	}
	f := descriptionFixture()
	f.WriteU32(nameTree+16*4, 0x80002180)
	s = openFixture(t, f)
	value, err = s.Text(Units, 1, "description_id")
	if err != nil || !value.Editable || !strings.HasPrefix(value.Content, "[0x2180]") {
		t.Fatalf("packed extended command was not retained: %v", err)
	}
	edit, err = s.PlanTextEdit(Units, 1, "description_id", value.Content+"\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyTextEdit(edit); err != nil {
		t.Fatal(err)
	}
	changed, err := s.Text(Units, 1, "description_id")
	if err != nil || changed.Content != value.Content+"\n" || !slices.Equal(changed.ControlCodes(), value.ControlCodes()) {
		t.Fatalf("packed command was split or changed: %v", err)
	}
}

func TestDescriptionVersions(t *testing.T) {
	for _, version := range nameVersions {
		t.Run(version.id, func(t *testing.T) {
			f := descriptionFixtureFor(version)
			originalContent, content := originalDescription, editedDescription
			want := []byte{0x2C, 0xE6, 0x91, 0xA8, 0, 0, 0, 0}
			if version.japanese {
				f.WriteU32(nameTree+16*4, 0x80008A83) // リ.
				f.WriteU32(nameTree+17*4, 0x80008083) // ム; the trail byte must not start an extended command.
				f.WriteU32(nameTree+24*4, 0x80004183) // A portrait argument that also forms the glyph ア.
				originalContent = "リ\n[0x0080][0x0021][0x0010][0x4183]ム[0x001F]"
				content = "リリム\n[0x0080][0x0021][0x0010][0x4183]リ[0x001F]"
				want = []byte{0x88, 0x24, 0xE6, 0x91, 0xA8, 0, 0, 0, 0}
			}
			s := openFixture(t, f)
			original, err := s.Text(Units, 1, "description_id")
			if err != nil || !original.Editable || original.Content != originalContent {
				t.Fatalf("version-specific text or parameters did not decode: %v", err)
			}
			before := s.Document.Snapshot()
			edit, err := s.PlanTextEdit(Units, 1, "description_id", content)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyTextEdit(edit); err != nil {
				t.Fatal(err)
			}
			value, err := s.Text(Units, 1, "description_id")
			if err != nil || value.Content != content || !slices.Equal(value.ControlCodes(), original.ControlCodes()) ||
				!bytes.Equal(s.Document.Snapshot().Data[edit.Destination():], want) {
				t.Fatalf("character boundary, parameter or fixed stream changed: %v", err)
			}
			if !s.Document.Undo() || !bytes.Equal(before.Data, s.Document.Snapshot().Data) || !s.Document.Redo() {
				t.Fatal("version-specific history lost bytes")
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			value, err = reopened.Text(Items, 0, "use_description_id")
			if err != nil || value.Content != content || !slices.Equal(value.ControlCodes(), original.ControlCodes()) {
				t.Fatalf("shared description did not survive saving: %v", err)
			}
			next, err := reopened.PlanTextEdit(Items, 0, "use_description_id", content+"\n")
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyTextEdit(next); err != nil {
				t.Fatal(err)
			}
		})
	}
}
