package game

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// Fixed offsets and widths follow the four MapSetting forms and ROMFE*.
var chapterVersions = []struct {
	version                                  nameVersion
	stride, name, objective, events, weather int
	music, musicWidth, boundary              int
}{
	{nameVersions[0], 148, 112, 136, 116, 18, 22, 2, 12},
	{nameVersions[1], 148, 112, 136, 116, 18, 22, 2, 12},
	{nameVersions[2], 152, 116, 140, 120, 18, 22, 2, 12},
	{nameVersions[3], 148, 112, 136, 116, 18, 22, 2, 12},
	{nameVersions[4], 68, 56, 48, 58, 17, 20, 1, 11},
}

func chapterFixture(index int) *testutil.FixtureBuilder {
	version := chapterVersions[index]
	f := textFixtureFor(version.version)
	base := f.GetPosition("chapters")
	for i := range 2 {
		f.WriteU16(base+i*version.stride+version.name, uint16(i+1))
		f.WriteU16(base+i*version.stride+version.objective, 4)
		f.WriteU8(base+i*version.stride+16, 0xA7) // An uninterpreted byte survives all edits.
	}
	return f
}

func TestChapterEditingWorkflow(t *testing.T) {
	for index, version := range chapterVersions {
		t.Run(version.version.id, func(t *testing.T) {
			f := chapterFixture(index)
			s := openFixture(t, f)
			table := s.Table(Chapters)
			if table.Err != nil || table.Stride != version.stride || table.FirstID != 0 || table.Count != 2 ||
				table.Rows[0].NameID != 1 || table.Rows[1].NameID != 2 || table.Rows[0].Name != "AB" {
				t.Fatalf("wrong native chapter layout or title location: %+v", table)
			}
			before := s.Document.Snapshot()
			if err := s.Set(Chapters, []int{0, 1}, "fog_level", 4); err != nil {
				t.Fatal(err)
			}
			expected := bytes.Clone(before.Data)
			base := f.GetPosition("chapters")
			expected[base+12], expected[base+version.stride+12] = 4, 4
			if !bytes.Equal(expected, s.Document.Snapshot().Data) || !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) || !s.Document.Redo() {
				t.Fatal("batch editing changed unrelated bytes or was not one history operation")
			}
			s.Refresh()
			music := int64(0x1234)
			if version.musicWidth == 1 {
				music = 0x34
			}
			for _, edit := range []struct {
				field         string
				offset, width int
				value         int64
			}{
				{"weather", version.weather, 1, 3},
				{"player_music_id", version.music, version.musicWidth, music},
				{"chapter_events_id", version.events, 1, 8},
				{"map_id", 8, 1, 9},
			} {
				if err := s.Set(Chapters, []int{1}, edit.field, edit.value); err != nil {
					t.Fatal(err)
				}
				at := base + version.stride + edit.offset
				for i := range edit.width {
					expected[at+i] = byte(edit.value >> (8 * i))
				}
			}
			if !bytes.Equal(expected, s.Document.Snapshot().Data) {
				t.Fatal("chapter offsets, field widths or uninterpreted bytes changed")
			}
			stable := s.Document.Snapshot()
			for _, edit := range []struct {
				ids   []int
				field string
				value int64
			}{
				{[]int{0, 256}, "fog_level", 6},
				{[]int{0}, "weather", 256},
				{[]int{0}, "player_music_id", 1 << (version.musicWidth * 8)},
			} {
				if err := s.Set(Chapters, edit.ids, edit.field, edit.value); err == nil {
					t.Fatal("invalid chapter edit was accepted")
				}
			}
			if stable.Revision != s.Document.State().Revision || !bytes.Equal(stable.Data, s.Document.Snapshot().Data) {
				t.Fatal("rejected edits changed the document")
			}
			text, err := s.Text(Chapters, 1, "objective_text_id")
			if err != nil || text.ID != 4 || text.Content != originalDescription {
				t.Fatalf("chapter objective did not resolve to the linked text: %+v, %v", text, err)
			}
			preview, err := s.PlanTextIDEdit(4, editedDescription)
			if err != nil || !slices.Contains(preview.Uses(), TextUse{Chapters, 0, "objective_text_id"}) ||
				!slices.Contains(preview.Uses(), TextUse{Chapters, 1, "objective_text_id"}) {
				t.Fatalf("shared text preview missed chapter references: %v", err)
			}
			if err := s.ApplyTextEdit(preview); err != nil {
				t.Fatal(err)
			}
			if !s.Document.Undo() || !bytes.Equal(stable.Data, s.Document.Snapshot().Data) || !s.Document.Redo() {
				t.Fatal("linked text did not retain complete history")
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			text, err = reopened.Text(Chapters, 0, "objective_text_id")
			if err != nil || text.Content != editedDescription {
				t.Fatalf("linked text did not survive saving: %v", err)
			}
			if value, err := reopened.Value(Chapters, 1, "player_music_id"); err != nil || value != music {
				t.Fatalf("saved chapter music: %d, %v", value, err)
			}
			if err := reopened.Set(Chapters, []int{1}, "fog_level", 5); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestChapterFallbackAndIsolation(t *testing.T) {
	for index, version := range chapterVersions {
		t.Run(version.version.id, func(t *testing.T) {
			f := chapterFixture(index)
			base := f.GetPosition("chapters")
			for i := range 2 {
				f.WriteU32(base+i*version.stride, 0) // Native fallback without CP.
				f.WriteU8(base+i*version.stride+4, 1)
			}
			s := openFixture(t, f)
			if table := s.Table(Chapters); table.Err != nil || table.Count != 2 {
				t.Fatalf("valid records without CP were lost: %+v", table)
			}
			for _, corrupt := range []struct {
				offset int
				data   []byte
			}{
				{version.boundary, []byte{0xE}},
				{4, []byte{0, 0, 0, 0}},
				{version.objective, []byte{0xFF, 0x7F}},
			} {
				broken := chapterFixture(index)
				for i := range 2 {
					broken.WriteU32(base+i*version.stride, 0)
					broken.WriteU8(base+i*version.stride+4, 1)
				}
				broken.WriteAt(base+version.stride+corrupt.offset, corrupt.data)
				bad := openFixture(t, broken)
				if table := bad.Table(Chapters); table.Err != nil || table.Count != 1 {
					t.Fatalf("legacy stopping rule ignored at offset %X: %+v", corrupt.offset, table)
				}
			}
			// An unreadable chapter table leaves the three other editors usable.
			for _, pointer := range s.Adapter.(gameFormat).chapters.pointers {
				f.WriteU32(pointer, 0)
			}
			bad := openFixture(t, f)
			if bad.Table(Chapters).Err == nil || bad.Table(Units).Err != nil || bad.Table(Items).Err != nil {
				t.Fatal("a broken chapter table disabled unrelated editors")
			}
			before := bad.Document.Snapshot()
			if err := bad.Set(Chapters, []int{0}, "fog_level", 2); !errors.Is(err, ErrLayout) ||
				!bytes.Equal(before.Data, bad.Document.Snapshot().Data) {
				t.Fatal("an invalid chapter layout was written")
			}
		})
	}
}

func TestFE6ChapterStrideAndManualLayout(t *testing.T) {
	f := chapterFixture(4)
	base := f.GetPosition("chapters")
	first := bytes.Clone(f.Bytes()[base : base+68])
	second := bytes.Clone(f.Bytes()[base+68 : base+136])
	f.WriteAt(base, bytes.Repeat([]byte{0}, 216))
	f.WriteAt(base, first)
	f.WriteAt(base+68, []byte{0xDE, 0xAD, 0xBE, 0xEF})
	f.WriteAt(base+72, second)
	f.WriteU16(0x2BB12, 0x2048)
	s := openFixture(t, f)
	if table := s.Table(Chapters); table.Err != nil || table.Stride != 72 || table.Count != 2 || table.Rows[1].NameID != 2 {
		t.Fatalf("FE6's 72-byte instruction was not honored: %+v", table)
	}
	before := s.Document.Snapshot()
	if err := s.Set(Chapters, []int{1}, "player_music_id", 0x45); err != nil {
		t.Fatal(err)
	}
	expected := bytes.Clone(before.Data)
	expected[base+72+20] = 0x45
	if !bytes.Equal(expected, s.Document.Snapshot().Data) {
		t.Fatal("FE6's stride tail or adjacent music bytes changed")
	}
	const manual = 0x400000
	region, err := s.ReadRegion(manual, 216)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyRegion(region, s.Document.Snapshot().Data[base:base+216]); err != nil {
		t.Fatal(err)
	}
	preview, err := s.PlanTextIDEdit(4, editedDescription)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UseLayout(Chapters, Layout{Address: manual, Stride: 72, Count: 2}); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyTextEdit(preview); !errors.Is(err, rom.ErrStale) {
		t.Fatal("a chapter-layout change accepted the old shared-text preview")
	}
	if err := s.Set(Chapters, []int{1}, "fog_level", 7); err != nil {
		t.Fatal(err)
	}
	data := s.Document.Snapshot().Data
	if data[manual+72+12] != 7 || data[base+72+12] != before.Data[base+72+12] {
		t.Fatal("manual chapter editing wrote the automatic table")
	}
	s.UseAutomaticLayout(Chapters)
	if s.Table(Chapters).Manual || s.Table(Chapters).Address != base || s.Table(Chapters).Stride != 72 {
		t.Fatal("automatic FE6 chapter layout did not recover")
	}
}

func TestChapterProtectedRanges(t *testing.T) {
	f := chapterFixture(0)
	s := openFixture(t, f)
	base := f.GetPosition("chapters")
	interval := space.Range{Start: base, End: base + 0x1000}
	free := []space.FreeRange{{Range: interval, Source: "declared interval overlapping chapters"}}
	if _, err := s.PlanTableMove(Items, free); !errors.Is(err, space.ErrRange) {
		t.Fatalf("another table could overwrite known chapters: %v", err)
	}
	f.WritePointer(0xA2A0, base-4) // Text ID 1 would update the first chapter's CP.
	bad := openFixture(t, f)
	before := bad.Document.Snapshot()
	if _, err := bad.PlanTextEdit(Units, 1, "name_id", "AB"); !errors.Is(err, ErrLayout) ||
		!bytes.Equal(before.Data, bad.Document.Snapshot().Data) {
		t.Fatalf("a text entry could overwrite a known chapter: %v", err)
	}
	// A bounded table at the ROM end can be read, but has no complete
	// stopping record to include in an automatic relocation.
	f = chapterFixture(0)
	at := len(f.Bytes()) - 148
	f.WritePointer(at, 0x300000)
	for _, pointer := range s.Adapter.(gameFormat).chapters.pointers {
		f.WritePointer(pointer, at)
	}
	truncated := openFixture(t, f)
	if truncated.Table(Chapters).Count != 1 {
		t.Fatal("the last bounded chapter record was not readable")
	}
	if _, err := truncated.PlanTableMove(Chapters, nil); !errors.Is(err, ErrLayout) {
		t.Fatal("an incomplete stopping record was moved")
	}
}
