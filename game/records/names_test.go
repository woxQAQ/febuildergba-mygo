package records_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestSharedNameEdits(t *testing.T) {
	for _, kind := range []records.Kind{records.Units, records.Classes, records.Items} {
		t.Run(string(kind), func(t *testing.T) {
			s := testutil.Open(t, testutil.NameFixture())
			before := s.Document.Snapshot()
			id := s.Records.Table(kind).FirstID
			edit, err := s.Records.PlanTextEdit(kind, id, "name_id", "ABC")
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
			if err := s.Texts.Apply(edit); err != nil {
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
				prefix[testutil.NameTable+4:],
				uint32(0x08000000+edit.Destination()),
			)
			if !bytes.Equal(prefix, after.Data[:len(prefix)]) {
				t.Fatal(
					"changed bytes outside the selected text entry, including old text or its alias",
				)
			}
			for _, table := range s.Records.Tables() {
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
			if s.Records.Table(records.Classes).Rows[1].Name != "AB" ||
				s.Records.Table(records.Items).Rows[1].Name != "B" {
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
			if s.Records.Table(kind).Rows[0].Name != "AB" ||
				!s.Document.Redo() {
				t.Fatal("undo interpretation or redo failed")
			}
			s.Refresh()
			if !bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("redo did not restore the complete name edit")
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			if reopened.Records.Table(kind).Rows[0].Name != "ABC" {
				t.Fatal("saved name did not reopen")
			}
			next, err := reopened.Records.PlanTextEdit(kind, id, "name_id", "B")
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Texts.Apply(next); err != nil {
				t.Fatal(err)
			}
			continued := reopened.Document.Snapshot().Data
			if next.Destination()%4 != 0 ||
				reopened.Records.Table(kind).Rows[0].Name != "B" ||
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
	s := testutil.Open(t, testutil.NameFixture())
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
			edit, err := s.Records.PlanTextEdit(
				records.Units,
				1,
				"name_id",
				sample.name,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Texts.Apply(edit); err != nil {
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
			if s.Records.Table(records.Units).Rows[0].Name != sample.name ||
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
		{"missing-symbol", "C", nil, text.ErrNameEncoding},
		{"unicode", "中文", nil, text.ErrNameEncoding},
		{"control", "A\x00B", nil, text.ErrNameEncoding},
		{"spaces", " A", nil, text.ErrNameEncoding},
		{"too-long", strings.Repeat("A", 4097), nil, text.ErrNameEncoding},
		{"reserved-id", "A", func(f *testutil.FixtureBuilder) { f.WriteU16(f.GetPosition("units"), 0) }, text.ErrText},
		{"out-of-range-id", "A", func(f *testutil.FixtureBuilder) { f.WriteU16(f.GetPosition("units"), 0x7FFF) }, text.ErrText},
		{"recovery-is-read-only", "A", func(f *testutil.FixtureBuilder) {
			f.WriteU32(0xA2A0, 0xFFFFFFFF)
			f.WritePointer(0x15D48C+4, testutil.NameStream)
		}, text.ErrText},
		{"invalid-root", "A", func(f *testutil.FixtureBuilder) { f.WriteU32(testutil.NameRoot, 0xFFFFFFFF) }, text.ErrText},
		{"cycle-in-unused-branch", "A", func(f *testutil.FixtureBuilder) {
			f.WriteAt(testutil.NameStream, []byte{0x1E, 0}) // The readable old name A avoids the broken AB branch.
			f.WriteU32(testutil.NameTree+20, 0x00050005)
		}, text.ErrText},
		{"missing-end-symbol", "A", func(f *testutil.FixtureBuilder) {
			f.WriteU32(testutil.NameTable+4, 0x88000000|testutil.NameRaw)
			f.WriteU32(testutil.NameTree+32, 0x80000042)
		}, text.ErrText},
		{"table-overlap", "A", func(f *testutil.FixtureBuilder) {
			f.WritePointer(0xA2A0, f.GetPosition("units")+0x28)
			f.WriteAt(0x300000, []byte{0x7B, 0})
		}, edit.ErrLayout},
	} {
		t.Run(sample.name, func(t *testing.T) {
			f := testutil.NameFixture()
			if sample.modify != nil {
				sample.modify(f)
			}
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			if _, err := s.Records.PlanTextEdit(
				records.Units,
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
	s := testutil.Open(t, testutil.NameFixture())
	edit, err := s.Records.PlanTextEdit(records.Units, 1, "name_id", "ABC")
	if err != nil {
		t.Fatal(err)
	}
	other := testutil.Open(t, testutil.NameFixture())
	if err := other.Texts.Apply(
		edit,
	); !errors.Is(err, rom.ErrStale) ||
		other.Document.State().Dirty {
		t.Fatalf("foreign preview: %v", err)
	}
	if err := s.Records.Set(
		records.Units,
		[]int{1},
		"base.hp",
		17,
	); err != nil {
		t.Fatal(err)
	}
	before := s.Document.Snapshot()
	if err := s.Texts.Apply(
		edit,
	); !errors.Is(err, rom.ErrStale) ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
		t.Fatalf("stale preview: %v", err)
	}
	edit, err = s.Records.PlanTextEdit(records.Units, 1, "name_id", "ABC")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Records.UseLayout(
		records.Classes,
		s.Records.Table(records.Classes).Layout,
	); err != nil {
		t.Fatal(err)
	}
	if err := s.Texts.Apply(
		edit,
	); !errors.Is(err, rom.ErrStale) ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
		t.Fatalf("changed interpretation accepted old preview: %v", err)
	}
}

func TestNameEditCapacity(t *testing.T) {
	for _, size := range []int{rom.MaxSize - 5, rom.MaxSize} {
		s := testutil.Open(t, testutil.NameFixture())
		if err := s.Document.ApplyEdit(
			0,
			"fixture capacity",
			rom.Edit{Size: size},
		); err != nil {
			t.Fatal(err)
		}
		before := s.Document.Snapshot()
		edit, err := s.Records.PlanTextEdit(records.Units, 1, "name_id", "A")
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
		if err := s.Texts.Apply(edit); err != nil {
			t.Fatal(err)
		}
		if s.Document.State().Size != rom.MaxSize ||
			s.Records.Table(records.Units).Rows[0].Name != "A" {
			t.Fatal("exact capacity was not accepted")
		}
	}
}

func TestNameEditAfterTableRelocation(t *testing.T) {
	s := testutil.Open(t, testutil.NameFixture())
	move, err := s.Records.PlanTableMove(records.Units, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Records.ApplyTableMove(move); err != nil {
		t.Fatal(err)
	}
	moved := s.Document.Snapshot()
	edit, err := s.Records.PlanTextEdit(records.Units, 1, "name_id", "ABC")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Texts.Apply(edit); err != nil {
		t.Fatal(err)
	}
	if s.Records.Table(records.Units).Address != move.Destination() ||
		s.Records.Table(records.Units).Rows[0].Name != "ABC" {
		t.Fatal("relocated record did not use the new name")
	}
	if !s.Document.Undo() ||
		!bytes.Equal(moved.Data, s.Document.Snapshot().Data) {
		t.Fatal("name undo changed the preceding table relocation")
	}
	s.Refresh()
	if s.Records.Table(records.Units).Address != move.Destination() ||
		s.Records.Table(records.Units).Rows[0].Name != "AB" {
		t.Fatal("name undo lost the relocated record interpretation")
	}
}

func TestNameEditReplacesAntiHuffmanEntryWithCompressedText(t *testing.T) {
	s := testutil.Open(t, testutil.NameFixture())
	edit, err := s.Records.PlanTextEdit(records.Items, 1, "name_id", "ABC")
	if err != nil {
		t.Fatal(err)
	}
	if edit.TextID() != 3 || len(edit.Uses()) != 1 {
		t.Fatal("selected the wrong shared text ID")
	}
	if err := s.Texts.Apply(edit); err != nil {
		t.Fatal(err)
	}
	data := s.Document.Snapshot().Data
	if gba.Uint32(
		data,
		testutil.NameTable+12,
	) != uint32(
		0x08000000+edit.Destination(),
	) ||
		s.Records.Table(records.Items).Rows[1].Name != "ABC" ||
		!bytes.Equal(
			data[testutil.NameRaw:testutil.NameRaw+2],
			[]byte{'B', 0},
		) {
		t.Fatal(
			"did not preserve the raw source and write a normal compressed pointer",
		)
	}
}

func TestNativeNamePadding(t *testing.T) {
	for _, word := range []uint32{0x80001F41, 0x8000411F} {
		f := testutil.NameFixture()
		f.WriteU32(testutil.NameTree+4, word)
		s := testutil.Open(t, f)
		edit, err := s.Records.PlanTextEdit(records.Units, 1, "name_id", "A")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Texts.Apply(edit); err != nil {
			t.Fatal(err)
		}
		if s.Records.Table(records.Units).Rows[0].Name != "A" ||
			!bytes.Equal(
				s.Document.Snapshot().Data[edit.Destination():],
				[]byte{0x1E, 0, 0, 0},
			) {
			t.Fatal("native padding code did not preserve the visible name")
		}
		before := s.Document.Snapshot()
		if _, err := s.Records.PlanTextEdit(
			records.Units,
			1,
			"name_id",
			strings.Repeat("A", 2049),
		); !errors.Is(err, text.ErrNameEncoding) ||
			s.Document.State().Revision != before.Revision {
			t.Fatalf("decoded padding exceeded the reader bound: %v", err)
		}
	}
	f := testutil.NameFixture()
	f.WriteU32(testutil.NameTable+4, 0x88000000|testutil.NameRaw)
	f.WriteU32(testutil.NameTree+32, 0x80001F1F)
	s := testutil.Open(t, f)
	if _, err := s.Records.PlanTextEdit(
		records.Units,
		1,
		"name_id",
		"A",
	); !errors.Is(err, text.ErrText) ||
		s.Document.State().Dirty {
		t.Fatalf("padding-only leaf was mistaken for an end symbol: %v", err)
	}
}

func TestCompactNameWrites(t *testing.T) {
	for _, sample := range []struct {
		version testutil.TextVersion
		glyphs  []uint16
		want    []byte
	}{
		{testutil.TextVersions[1], []uint16{0x7084, 0x7184, 0x7284, 0x7384}, []byte{0xA0, 0x1C, 0, 0, 0, 0}},
		{testutil.TextVersions[3], []uint16{0x4087, 0x4187, 0x4287, 0x4387, 0x4487, 0x4587, 0x4687}, []byte{0xA0, 0x9C, 0xEE, 0, 0, 0, 0}},
		{testutil.TextVersions[4], []uint16{0x4A87, 0x4B87, 0x4C87, 0x4D87, 0x4E87, 0x4F87, 0x5087}, []byte{0xA0, 0x9C, 0xEE, 0, 0, 0, 0}},
	} {
		t.Run(sample.version.ID, func(t *testing.T) {
			f := testutil.NameFixtureFor(sample.version)
			// A complete three-level tree assigns the glyphs in order, followed
			// by end leaves. Expected streams are fixed independently of the
			// encoder.
			for i := range 7 {
				f.WriteU16(testutil.NameTree+i*4, uint16(i*2+1))
				f.WriteU16(testutil.NameTree+i*4+2, uint16(i*2+2))
			}
			for i := range 8 {
				word := uint32(0x80000000)
				if i < len(sample.glyphs) {
					word |= uint32(sample.glyphs[i])
				}
				f.WriteU32(testutil.NameTree+(i+7)*4, word)
			}
			f.WriteAt(
				testutil.NameStream,
				[]byte{7, 0},
			) // An end leaf keeps existing names readable.
			s := testutil.Open(t, f)
			const name = "(ﾄﾞﾗｺﾞﾝﾏｽﾀｰ)"
			edit, err := s.Records.PlanTextEdit(
				records.Classes,
				0,
				"name_id",
				name,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Texts.Apply(edit); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(
				s.Document.Snapshot().Data[edit.Destination():],
				sample.want,
			) ||
				s.Records.Table(records.Classes).Rows[0].Name != name {
				t.Fatal(
					"compact display name did not encode to the version's original glyph sequence",
				)
			}
		})
	}
}

func TestJapaneseNameCharacters(t *testing.T) {
	for _, version := range testutil.TextVersions {
		if !version.Japanese {
			continue
		}
		t.Run(version.ID, func(t *testing.T) {
			f := testutil.NameFixtureFor(version)
			f.WriteU32(testutil.NameTree+4, 0x80008A83)  // リ.
			f.WriteU32(testutil.NameTree+12, 0x80000041) // A.
			f.WriteU32(testutil.NameTree+20, 0x800000B1) // Half-width ｱ.
			s := testutil.Open(t, f)
			edit, err := s.Records.PlanTextEdit(
				records.Units,
				1,
				"name_id",
				"リAｱ",
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Texts.Apply(edit); err != nil {
				t.Fatal(err)
			}
			if s.Records.Table(records.Units).Rows[0].Name != "リAｱ" ||
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
				if _, err := s.Records.PlanTextEdit(
					records.Units,
					1,
					"name_id",
					name,
				); !errors.Is(
					err,
					text.ErrNameEncoding,
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
			if _, err := s.Records.PlanTextEdit(
				records.Units,
				1,
				"name_id",
				strings.Repeat("リ", 1400),
			); err != nil {
				t.Fatalf(
					"applied the byte bound to UTF-8 rather than game encoding: %v",
					err,
				)
			}
			f = testutil.NameFixtureFor(version)
			f.WriteU32(testutil.NameTree+4, 0x80000083)
			f.WriteU32(testutil.NameTree+12, 0x8000008A)
			split := testutil.Open(t, f)
			if _, err := split.Records.PlanTextEdit(
				records.Units,
				1,
				"name_id",
				"リ",
			); !errors.Is(err, text.ErrNameEncoding) ||
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
	for _, version := range testutil.TextVersions {
		t.Run(version.ID, func(t *testing.T) {
			f := testutil.NameFixtureFor(version)
			name, nextName := "ABC", "A"
			want := []byte{0xEE, 1, 0, 0, 0}
			nextWant := []byte{0x1E, 0, 0, 0}
			if version.Japanese {
				f.WriteU32(testutil.NameTree+4, 0x80008A83)  // リ in Shift-JIS.
				f.WriteU32(testutil.NameTree+12, 0x80009383) // ン in Shift-JIS.
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
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			edit, err := s.Records.PlanTextEdit(
				records.Units,
				1,
				"name_id",
				name,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Texts.Apply(edit); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(
				s.Document.Snapshot().Data[edit.Destination():],
				want,
			) ||
				s.Records.Table(records.Units).Rows[0].Name != name ||
				s.Records.Table(records.Classes).Rows[1].Name != "AB" {
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
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil ||
				reopened.Records.Table(records.Units).Rows[0].Name != name {
				t.Fatalf("name did not reopen: %v", err)
			}
			next, err := reopened.Records.PlanTextEdit(
				records.Units,
				1,
				"name_id",
				nextName,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Texts.Apply(next); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(
				reopened.Document.Snapshot().Data[next.Destination():],
				nextWant,
			) ||
				reopened.Records.Table(records.Units).Rows[0].Name != nextName {
				t.Fatal(
					"continued editing did not preserve version-specific end padding",
				)
			}
		})
	}
}
