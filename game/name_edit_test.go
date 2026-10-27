package game

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

const nameTree, nameRoot, nameStream, nameTable, nameRaw = 0x310000, 0x310100, 0x311000, 0x312000, 0x313000

type nameVersion struct {
	id                                  string
	build                               func() *testutil.FixtureBuilder
	text, tree, root                    int
	unitStride, classStride, itemStride int
	japanese                            bool
}

var nameVersions = []nameVersion{
	{"FE8U", testutil.NewMinimalFE8U, 0xA2A0, 0x6E0, 0x6DC, 52, 84, 36, false},
	{"FE8J", testutil.NewMinimalFE8J, 0xA000, 0x6E0, 0x6DC, 52, 84, 36, true},
	{"FE7U", testutil.NewMinimalFE7U, 0x12CB8, 0x6BC, 0x6B8, 52, 84, 36, false},
	{"FE7J", testutil.NewMinimalFE7J, 0x13370, 0x6E0, 0x6DC, 52, 84, 36, true},
	{"FE6", testutil.NewMinimalFE6, 0x13B10, 0x6E0, 0x6DC, 48, 72, 32, true},
}

func nameFixture() *testutil.FixtureBuilder { return nameFixtureFor(nameVersions[0]) }

func nameFixtureFor(version nameVersion) *testutil.FixtureBuilder {
	f := version.build()
	// Fixed codes: A=0, B=10, AB=110, BC=1110, end=1111.
	// ABC requires A+BC; taking AB greedily leaves an unencodable C.
	for i, word := range []uint32{0x00020001, 0x80000041, 0x00040003, 0x80000042, 0x00060005, 0x80004241, 0x00080007, 0x80004342, 0x80000000} {
		f.WriteU32(nameTree+i*4, word)
	}
	f.WritePointer(version.tree, nameTree)
	f.WritePointer(version.root, nameRoot)
	f.WritePointer(nameRoot, nameTree)
	f.WritePointer(version.text, nameTable)
	f.WriteAt(
		nameStream,
		[]byte{0x7B, 0},
	) // AB followed by the padded end symbol.
	f.WritePointer(nameTable+4, nameStream)
	f.WritePointer(
		nameTable+8,
		nameStream,
	) // Another ID aliases the original bytes.
	f.WriteU32(nameTable+12, 0x88000000|nameRaw)
	f.WriteAt(nameRaw, []byte{'B', 0})
	f.WriteU16(f.GetPosition("units"), 1)
	f.WriteU16(f.GetPosition("units")+version.unitStride, 1)
	f.WriteU16(f.GetPosition("classes"), 1)
	f.WriteU16(f.GetPosition("classes")+version.classStride, 2)
	f.WriteU16(f.GetPosition("items"), 1)
	f.WriteU16(f.GetPosition("items")+version.itemStride, 3)
	return f
}

func TestSharedNameEdits(t *testing.T) {
	for _, kind := range []Kind{Units, Classes, Items} {
		t.Run(string(kind), func(t *testing.T) {
			s := openFixture(t, nameFixture())
			before := s.Document.Snapshot()
			id := s.Table(kind).FirstID
			edit, err := s.PlanTextEdit(kind, id, "name_id", "ABC")
			if err != nil {
				t.Fatal(err)
			}
			if len(edit.Uses()) != 4 || edit.TextID() != 1 ||
				edit.Destination() != len(before.Data) {
				t.Fatalf("unexpected shared-name preview: %+v", edit)
			}
			uses := edit.Uses()
			uses[0].ID = 999
			if edit.Uses()[0].ID == 999 || s.Document.State().Dirty ||
				s.Document.State().Revision != before.Revision {
				t.Fatal("preview was mutable or changed the document")
			}
			if err := s.ApplyTextEdit(edit); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			want := []byte{0xEE, 1, 0, 0, 0}
			if edit.Length() != len(want) ||
				edit.Size() != len(before.Data)+len(want) ||
				!bytes.Equal(after.Data[edit.Destination():], want) {
				t.Fatal(
					"encoded name differs from the fixed low-bit-first sample or lacks trailing read space",
				)
			}
			prefix := bytes.Clone(before.Data)
			binary.LittleEndian.PutUint32(
				prefix[nameTable+4:],
				uint32(0x08000000+edit.Destination()),
			)
			if !bytes.Equal(prefix, after.Data[:len(prefix)]) {
				t.Fatal(
					"changed bytes outside the selected text entry, including old text or its alias",
				)
			}
			for _, table := range s.Tables {
				for _, row := range table.Rows {
					if row.NameID == 1 &&
						(row.Name != "ABC" || row.TextError != nil) {
						t.Fatalf(
							"shared record %s/%d was not refreshed: %+v",
							table.Kind,
							row.ID,
							row,
						)
					}
				}
			}
			if s.Table(Classes).Rows[1].Name != "AB" ||
				s.Table(Items).Rows[1].Name != "B" {
				t.Fatal("an independent text ID changed")
			}
			if !s.Document.Undo() || s.Document.State().CanUndo ||
				s.Document.State().Dirty ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal(
					"one undo did not restore text, pointers, history and file size",
				)
			}
			s.Refresh()
			if s.Table(kind).Rows[0].Name != "AB" || !s.Document.Redo() {
				t.Fatal("undo interpretation or redo failed")
			}
			s.Refresh()
			if !bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("redo did not restore the complete name edit")
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			if reopened.Table(kind).Rows[0].Name != "ABC" {
				t.Fatal("saved name did not reopen")
			}
			next, err := reopened.PlanTextEdit(kind, id, "name_id", "B")
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyTextEdit(next); err != nil {
				t.Fatal(err)
			}
			continued := reopened.Document.Snapshot().Data
			if next.Destination()%4 != 0 ||
				reopened.Table(kind).Rows[0].Name != "B" ||
				!bytes.Equal(
					continued[len(after.Data):next.Destination()],
					[]byte{0xFF, 0xFF, 0xFF},
				) ||
				!bytes.Equal(continued[edit.Destination():edit.Size()], want) {
				t.Fatal(
					"continued editing lost the previous text or aligned FF padding",
				)
			}
		})
	}
}

func TestNameEncodingSamples(t *testing.T) {
	s := openFixture(t, nameFixture())
	for _, sample := range []struct {
		name string
		want []byte
	}{
		{"A", []byte{0x1E, 0, 0, 0}},
		{"AB", []byte{0x7B, 0, 0, 0, 0}},
		{"ABA", []byte{0xF3, 0, 0, 0, 0}},
		{"ABC", []byte{0xEE, 1, 0, 0, 0}},
		{"", []byte{0x0F, 0, 0, 0}},
	} {
		t.Run(sample.name, func(t *testing.T) {
			edit, err := s.PlanTextEdit(Units, 1, "name_id", sample.name)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyTextEdit(edit); err != nil {
				t.Fatal(err)
			}
			if got := s.Document.Snapshot().Data[edit.Destination():]; !bytes.Equal(
				got,
				sample.want,
			) {
				t.Fatalf(
					"encoded %q as %X, want %X",
					sample.name,
					got,
					sample.want,
				)
			}
			if s.Table(Units).Rows[0].Name != sample.name ||
				!s.Document.Undo() {
				t.Fatal("encoded name did not round-trip or undo")
			}
			s.Refresh()
		})
	}
}

func TestNameEditRejectsInvalidInput(t *testing.T) {
	for _, sample := range []struct {
		name   string
		input  string
		modify func(*testutil.FixtureBuilder)
		want   error
	}{
		{"missing-symbol", "C", nil, ErrNameEncoding},
		{"unicode", "中文", nil, ErrNameEncoding},
		{"control", "A\x00B", nil, ErrNameEncoding},
		{"spaces", " A", nil, ErrNameEncoding},
		{"too-long", strings.Repeat("A", 4097), nil, ErrNameEncoding},
		{"reserved-id", "A", func(f *testutil.FixtureBuilder) { f.WriteU16(f.GetPosition("units"), 0) }, ErrText},
		{"out-of-range-id", "A", func(f *testutil.FixtureBuilder) { f.WriteU16(f.GetPosition("units"), 0x7FFF) }, ErrText},
		{"recovery-is-read-only", "A", func(f *testutil.FixtureBuilder) {
			f.WriteU32(0xA2A0, 0xFFFFFFFF)
			f.WritePointer(0x15D48C+4, nameStream)
		}, ErrText},
		{"invalid-root", "A", func(f *testutil.FixtureBuilder) { f.WriteU32(nameRoot, 0xFFFFFFFF) }, ErrText},
		{"cycle-in-unused-branch", "A", func(f *testutil.FixtureBuilder) {
			f.WriteAt(nameStream, []byte{0x1E, 0}) // The readable old name A avoids the broken AB branch.
			f.WriteU32(nameTree+20, 0x00050005)
		}, ErrText},
		{"missing-end-symbol", "A", func(f *testutil.FixtureBuilder) {
			f.WriteU32(nameTable+4, 0x88000000|nameRaw)
			f.WriteU32(nameTree+32, 0x80000042)
		}, ErrText},
		{"table-overlap", "A", func(f *testutil.FixtureBuilder) {
			f.WritePointer(0xA2A0, f.GetPosition("units")+0x28)
			f.WriteAt(0x300000, []byte{0x7B, 0})
		}, ErrLayout},
	} {
		t.Run(sample.name, func(t *testing.T) {
			f := nameFixture()
			if sample.modify != nil {
				sample.modify(f)
			}
			s := openFixture(t, f)
			before := s.Document.Snapshot()
			if _, err := s.PlanTextEdit(
				Units,
				1,
				"name_id",
				sample.input,
			); !errors.Is(
				err,
				sample.want,
			) {
				t.Fatalf("preview error %v, want %v", err, sample.want)
			}
			if s.Document.State().Revision != before.Revision ||
				s.Document.State().Dirty ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("failed preview changed bytes, size or history")
			}
		})
	}
}

func TestNameEditBindsDocumentRevisionAndLayouts(t *testing.T) {
	s := openFixture(t, nameFixture())
	edit, err := s.PlanTextEdit(Units, 1, "name_id", "ABC")
	if err != nil {
		t.Fatal(err)
	}
	other := openFixture(t, nameFixture())
	if err := other.ApplyTextEdit(
		edit,
	); !errors.Is(err, rom.ErrStale) ||
		other.Document.State().Dirty {
		t.Fatalf("foreign preview: %v", err)
	}
	if err := s.Set(Units, []int{1}, "base.hp", 17); err != nil {
		t.Fatal(err)
	}
	before := s.Document.Snapshot()
	if err := s.ApplyTextEdit(
		edit,
	); !errors.Is(err, rom.ErrStale) ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
		t.Fatalf("stale preview: %v", err)
	}
	edit, err = s.PlanTextEdit(Units, 1, "name_id", "ABC")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UseLayout(Classes, s.Table(Classes).Layout); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyTextEdit(
		edit,
	); !errors.Is(err, rom.ErrStale) ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
		t.Fatalf("changed interpretation accepted old preview: %v", err)
	}
}

func TestNameEditCapacity(t *testing.T) {
	for _, size := range []int{rom.MaxSize - 5, rom.MaxSize} {
		s := openFixture(t, nameFixture())
		if err := s.Document.ApplyEdit(
			0,
			"fixture capacity",
			rom.Edit{Size: size},
		); err != nil {
			t.Fatal(err)
		}
		before := s.Document.Snapshot()
		edit, err := s.PlanTextEdit(Units, 1, "name_id", "A")
		if size == rom.MaxSize {
			if !errors.Is(err, space.ErrCapacity) ||
				s.Document.State().Revision != before.Revision ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatalf("capacity failure changed the ROM: %v", err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := s.ApplyTextEdit(edit); err != nil {
			t.Fatal(err)
		}
		if s.Document.State().Size != rom.MaxSize ||
			s.Table(Units).Rows[0].Name != "A" {
			t.Fatal("exact capacity was not accepted")
		}
	}
}

func TestNameEditAfterTableRelocation(t *testing.T) {
	s := openFixture(t, nameFixture())
	move, err := s.PlanTableMove(Units, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyTableMove(move); err != nil {
		t.Fatal(err)
	}
	moved := s.Document.Snapshot()
	edit, err := s.PlanTextEdit(Units, 1, "name_id", "ABC")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyTextEdit(edit); err != nil {
		t.Fatal(err)
	}
	if s.Table(Units).Address != move.Destination() ||
		s.Table(Units).Rows[0].Name != "ABC" {
		t.Fatal("relocated record did not use the new name")
	}
	if !s.Document.Undo() ||
		!bytes.Equal(moved.Data, s.Document.Snapshot().Data) {
		t.Fatal("name undo changed the preceding table relocation")
	}
	s.Refresh()
	if s.Table(Units).Address != move.Destination() ||
		s.Table(Units).Rows[0].Name != "AB" {
		t.Fatal("name undo lost the relocated record interpretation")
	}
}

func TestNameEditReplacesAntiHuffmanEntryWithCompressedText(t *testing.T) {
	s := openFixture(t, nameFixture())
	edit, err := s.PlanTextEdit(Items, 1, "name_id", "ABC")
	if err != nil {
		t.Fatal(err)
	}
	if edit.TextID() != 3 || len(edit.Uses()) != 1 {
		t.Fatal("selected the wrong shared text ID")
	}
	if err := s.ApplyTextEdit(edit); err != nil {
		t.Fatal(err)
	}
	data := s.Document.Snapshot().Data
	if u32(data, nameTable+12) != uint32(0x08000000+edit.Destination()) ||
		s.Table(Items).Rows[1].Name != "ABC" || !bytes.Equal(data[nameRaw:nameRaw+2], []byte{'B', 0}) {
		t.Fatal(
			"did not preserve the raw source and write a normal compressed pointer",
		)
	}
}

func TestNativeNamePadding(t *testing.T) {
	for _, word := range []uint32{0x80001F41, 0x8000411F} {
		f := nameFixture()
		f.WriteU32(nameTree+4, word)
		s := openFixture(t, f)
		edit, err := s.PlanTextEdit(Units, 1, "name_id", "A")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.ApplyTextEdit(edit); err != nil {
			t.Fatal(err)
		}
		if s.Table(Units).Rows[0].Name != "A" ||
			!bytes.Equal(
				s.Document.Snapshot().Data[edit.Destination():],
				[]byte{0x1E, 0, 0, 0},
			) {
			t.Fatal("native padding code did not preserve the visible name")
		}
		before := s.Document.Snapshot()
		if _, err := s.PlanTextEdit(
			Units,
			1,
			"name_id",
			strings.Repeat("A", 2049),
		); !errors.Is(err, ErrNameEncoding) ||
			s.Document.State().Revision != before.Revision {
			t.Fatalf("decoded padding exceeded the reader bound: %v", err)
		}
	}
	f := nameFixture()
	f.WriteU32(nameTable+4, 0x88000000|nameRaw)
	f.WriteU32(nameTree+32, 0x80001F1F)
	s := openFixture(t, f)
	if _, err := s.PlanTextEdit(
		Units,
		1,
		"name_id",
		"A",
	); !errors.Is(err, ErrText) ||
		s.Document.State().Dirty {
		t.Fatalf("padding-only leaf was mistaken for an end symbol: %v", err)
	}
}

func TestCompactNameWrites(t *testing.T) {
	for _, sample := range []struct {
		version nameVersion
		glyphs  []uint16
		want    []byte
	}{
		{nameVersions[1], []uint16{0x7084, 0x7184, 0x7284, 0x7384}, []byte{0xA0, 0x1C, 0, 0, 0, 0}},
		{nameVersions[3], []uint16{0x4087, 0x4187, 0x4287, 0x4387, 0x4487, 0x4587, 0x4687}, []byte{0xA0, 0x9C, 0xEE, 0, 0, 0, 0}},
		{nameVersions[4], []uint16{0x4A87, 0x4B87, 0x4C87, 0x4D87, 0x4E87, 0x4F87, 0x5087}, []byte{0xA0, 0x9C, 0xEE, 0, 0, 0, 0}},
	} {
		t.Run(sample.version.id, func(t *testing.T) {
			f := nameFixtureFor(sample.version)
			// A complete three-level tree assigns the glyphs in order, followed
			// by end leaves. Expected streams are fixed independently of the
			// encoder.
			for i := range 7 {
				f.WriteU16(nameTree+i*4, uint16(i*2+1))
				f.WriteU16(nameTree+i*4+2, uint16(i*2+2))
			}
			for i := range 8 {
				word := uint32(0x80000000)
				if i < len(sample.glyphs) {
					word |= uint32(sample.glyphs[i])
				}
				f.WriteU32(nameTree+(i+7)*4, word)
			}
			f.WriteAt(
				nameStream,
				[]byte{7, 0},
			) // An end leaf keeps existing names readable.
			s := openFixture(t, f)
			const name = "(ﾄﾞﾗｺﾞﾝﾏｽﾀｰ)"
			edit, err := s.PlanTextEdit(Classes, 0, "name_id", name)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyTextEdit(edit); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(
				s.Document.Snapshot().Data[edit.Destination():],
				sample.want,
			) ||
				s.Table(Classes).Rows[0].Name != name {
				t.Fatal(
					"compact display name did not encode to the version's original glyph sequence",
				)
			}
		})
	}
}

func TestJapaneseNameCharacters(t *testing.T) {
	for _, version := range nameVersions {
		if !version.japanese {
			continue
		}
		t.Run(version.id, func(t *testing.T) {
			f := nameFixtureFor(version)
			f.WriteU32(nameTree+4, 0x80008A83)  // リ.
			f.WriteU32(nameTree+12, 0x80000041) // A.
			f.WriteU32(nameTree+20, 0x800000B1) // Half-width ｱ.
			s := openFixture(t, f)
			edit, err := s.PlanTextEdit(Units, 1, "name_id", "リAｱ")
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyTextEdit(edit); err != nil {
				t.Fatal(err)
			}
			if s.Table(Units).Rows[0].Name != "リAｱ" ||
				!bytes.Equal(
					s.Document.Snapshot().Data[edit.Destination():],
					[]byte{0xDA, 3, 0, 0, 0},
				) {
				t.Fatal(
					"mixed Shift-JIS, ASCII and half-width characters did not round-trip",
				)
			}
			before := s.Document.Snapshot()
			for _, name := range []string{"🙂", "¥", "リ\x00A", " リ", string([]byte{0xFF}), strings.Repeat("リ", 2049)} {
				if _, err := s.PlanTextEdit(
					Units,
					1,
					"name_id",
					name,
				); !errors.Is(
					err,
					ErrNameEncoding,
				) {
					t.Fatalf(
						"accepted unsupported or lossy name %q: %v",
						name,
						err,
					)
				}
			}
			if s.Document.State().Revision != before.Revision ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("rejected Japanese input changed the document")
			}
			if _, err := s.PlanTextEdit(
				Units,
				1,
				"name_id",
				strings.Repeat("リ", 1400),
			); err != nil {
				t.Fatalf(
					"applied the byte bound to UTF-8 rather than game encoding: %v",
					err,
				)
			}
			f = nameFixtureFor(version)
			f.WriteU32(nameTree+4, 0x80000083)
			f.WriteU32(nameTree+12, 0x8000008A)
			split := openFixture(t, f)
			if _, err := split.PlanTextEdit(
				Units,
				1,
				"name_id",
				"リ",
			); !errors.Is(err, ErrNameEncoding) ||
				split.Document.State().Dirty {
				t.Fatalf(
					"split a Shift-JIS character across single-byte codes: %v",
					err,
				)
			}
		})
	}
}

func TestVersionNameEdits(t *testing.T) {
	for _, version := range nameVersions {
		t.Run(version.id, func(t *testing.T) {
			f := nameFixtureFor(version)
			name, nextName := "ABC", "A"
			want := []byte{0xEE, 1, 0, 0, 0}
			nextWant := []byte{0x1E, 0, 0, 0}
			if version.japanese {
				f.WriteU32(nameTree+4, 0x80008A83)  // リ in Shift-JIS.
				f.WriteU32(nameTree+12, 0x80009383) // ン in Shift-JIS.
				name, nextName = "リン", "リ"
				want = []byte{0x7A, 0, 0, 0, 0}
				nextWant = []byte{
					0x1E,
					0,
					0,
					0,
					0,
				} // Eight end bits require the second stream byte.
			}
			s := openFixture(t, f)
			before := s.Document.Snapshot()
			edit, err := s.PlanTextEdit(Units, 1, "name_id", name)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyTextEdit(edit); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(
				s.Document.Snapshot().Data[edit.Destination():],
				want,
			) ||
				s.Table(Units).Rows[0].Name != name ||
				s.Table(Classes).Rows[1].Name != "AB" {
				t.Fatal(
					"wrong version encoding or changed an independent text ID",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				!s.Document.Redo() {
				t.Fatal("version-specific name history lost bytes")
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(s.Document.State().Path)
			if err != nil || reopened.Table(Units).Rows[0].Name != name {
				t.Fatalf("name did not reopen: %v", err)
			}
			next, err := reopened.PlanTextEdit(Units, 1, "name_id", nextName)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyTextEdit(next); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(
				reopened.Document.Snapshot().Data[next.Destination():],
				nextWant,
			) ||
				reopened.Table(Units).Rows[0].Name != nextName {
				t.Fatal(
					"continued editing did not preserve version-specific end padding",
				)
			}
		})
	}
}
