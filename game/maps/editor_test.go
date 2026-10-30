package maps_test

import (
	"bytes"
	"errors"
	"image"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestMapEditingWorkflow(t *testing.T) {
	for _, version := range []string{"FE6", "FE7J", "FE7U", "FE8J", "FE8U"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewMapFixture(version)
			stride := 148
			if version == "FE6" {
				stride = 68
			}
			if version == "FE7U" {
				stride = 152
			}
			base, source := f.GetPosition(
				"chapters",
			), f.GetPosition(
				"map_layout",
			)
			f.WriteAt(base+stride*2, f.Bytes()[base:base+stride])
			f.WritePointer(f.GetPosition("map_layout_list")+6*4, source)
			// Independent literal stream: two cells and four uninterpreted bytes.
			f.WriteAt(
				source,
				[]byte{
					0x10,
					10,
					0,
					0,
					0,
					2,
					1,
					0,
					0,
					4,
					0,
					0xA5,
					0x5A,
					0,
					0xC3,
					0x7E,
				},
			)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			d, err := s.Maps.NewDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			set, err := d.Tileset()
			if err != nil || len(set.Cells) < 2 ||
				set.Cells[1] != (maps.Cell{Tile: 4, Terrain: 0x21}) {
				t.Fatal("native tileset choices", err)
			}
			if err = d.Stroke([]image.Point{{X: 0}, {X: 1}}, 4); err != nil {
				t.Fatal(err)
			}
			if !d.Undo() || d.Modified() || !d.Redo() {
				t.Fatal("one stroke must be one draft operation")
			}
			plan, err := s.Maps.Plan(d)
			if err != nil {
				t.Fatal(err)
			}
			if plan.ChangedCells() != 1 || plan.Source() != source ||
				plan.Destination() != len(before.Data) {
				t.Fatal("wrong map allocation or changed cell count")
			}
			uses := plan.Uses()
			if len(uses) != 3 || !uses[0].Updated || uses[1].Updated ||
				!uses[2].Updated {
				t.Fatal("shared and independent entries", uses)
			}
			uses[0].Updated = false
			if !plan.Uses()[0].Updated {
				t.Fatal("mutable preview ownership")
			}
			if err = d.Fill(image.Rect(0, 0, 2, 1), 0); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				s.Document.State().Dirty {
				t.Fatal("draft or preview wrote ROM")
			}
			if err = s.Maps.Apply(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			at, valid := gba.Pointer(after.Data, plan.Pointer())
			if !valid || at != plan.Destination() ||
				len(after.Data) != at+plan.Length() {
				t.Fatal("pointer and file length were not committed together")
			}
			decoded, err := gba.DecompressLZ77(after.Data[at:], 3074)
			if err != nil ||
				!bytes.Equal(
					decoded,
					[]byte{2, 1, 4, 0, 4, 0, 0xA5, 0x5A, 0xC3, 0x7E},
				) {
				t.Fatal("mutable plan or lost unknown tail", decoded, err)
			}
			preserved := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				preserved[plan.Pointer():plan.Pointer()+4],
				before.Data[plan.Pointer():plan.Pointer()+4],
			)
			if !bytes.Equal(preserved, before.Data) {
				t.Fatal(
					"map edit touched old resources or independent references",
				)
			}
			for _, chapter := range []int{0, 1, 2} {
				preview, readErr := s.Maps.Read(chapter)
				want := uint16(4)
				if chapter == 1 {
					want = 0
				}
				if readErr != nil || preview.Cells[0].Tile != want {
					t.Fatal("wrong shared chapter result", chapter, readErr)
				}
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				!s.Document.Redo() ||
				!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("map document history")
			}
			if err = s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			next, err := reopened.Maps.NewDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			if err = next.Stroke([]image.Point{{X: 1}}, 0); err != nil {
				t.Fatal(err)
			}
			nextPlan, err := reopened.Maps.Plan(next)
			if err != nil {
				t.Fatal(err)
			}
			if err = reopened.Maps.Apply(nextPlan); err != nil {
				t.Fatal(err)
			}
			preview, err := reopened.Maps.Read(0)
			if err != nil ||
				!slices.Equal(
					preview.Cells,
					[]maps.Cell{
						{Tile: 4, Terrain: 0x21},
						{Tile: 0, Terrain: 1},
					},
				) {
				t.Fatal("cannot continue drawing after reopen", err)
			}
		})
	}
}

func TestMapDraftGesturesAndFailures(t *testing.T) {
	f := testutil.NewMapFixture("FE8U")
	// 4 x 3 with twelve initial tile-zero cells, independently encoded.
	f.WriteAt(
		f.GetPosition("map_layout"),
		[]byte{
			0x10,
			26,
			0,
			0,
			0,
			4,
			3,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
		},
	)
	s := testutil.Open(t, f)
	d, err := s.Maps.NewDraft(0)
	if err != nil {
		t.Fatal(err)
	}
	if err = d.Stroke([]image.Point{{}, {X: 3}}, 4); err != nil {
		t.Fatal(err)
	}
	for x := range 4 {
		if cell, _ := d.Cell(x, 0); cell.Tile != 4 {
			t.Fatal("sparse stroke left a hole")
		}
	}
	if !d.Undo() || d.Modified() || !d.Redo() {
		t.Fatal("stroke history")
	}
	if err = d.Fill(image.Rect(1, 1, 4, 3), 4); err != nil {
		t.Fatal(err)
	}
	before, _ := d.Preview()
	for _, operation := range []func() error{
		func() error { return d.Stroke([]image.Point{{}, {X: 4}}, 0) },
		func() error { return d.Stroke([]image.Point{{}}, 0xFFFF) },
		func() error { return d.Fill(image.Rect(-1, 0, 1, 1), 0) },
		func() error { return d.Fill(image.Rectangle{}, 0) },
	} {
		if err = operation(); !errors.Is(err, edit.ErrValue) {
			t.Fatal("invalid painting accepted", err)
		}
		after, _ := d.Preview()
		if !slices.Equal(before.Cells, after.Cells) {
			t.Fatal("failed gesture partially changed draft")
		}
	}
	if !d.Undo() || !d.CanRedo() {
		t.Fatal("rectangle history")
	}
	if err = d.Stroke([]image.Point{{Y: 2}}, 4); err != nil || d.CanRedo() {
		t.Fatal("new stroke did not replace redo branch", err)
	}
	if s.Document.State().Dirty {
		t.Fatal("draft history changed document")
	}
}

func TestMapWriteGuards(t *testing.T) {
	for _, sample := range []struct {
		name   string
		change func(*testutil.FixtureBuilder)
	}{
		{"record table", func(f *testutil.FixtureBuilder) {
			f.WritePointer(f.GetPosition("map_layout_pointer"), f.GetPosition("chapters")-16)
			f.WritePointer(f.GetPosition("chapters"), f.GetPosition("map_layout"))
		}},
		{"palette bytes", func(f *testutil.FixtureBuilder) {
			f.WritePointer(f.GetPosition("map_layout_pointer"), f.GetPosition("map_palette")-16)
			f.WritePointer(f.GetPosition("map_palette"), f.GetPosition("map_layout"))
		}},
		{"compressed tileset bytes", func(f *testutil.FixtureBuilder) {
			f.WritePointer(f.GetPosition("map_layout_pointer"), f.GetPosition("map_config")+16)
			f.WritePointer(f.GetPosition("map_config")+32, f.GetPosition("map_layout"))
		}},
		{"animation entry", func(f *testutil.FixtureBuilder) {
			f.WritePointer(0x30134, f.GetPosition("map_layout_list"))
			f.WriteU8(f.GetPosition("chapters")+9, 4)
		}},
	} {
		t.Run(sample.name, func(t *testing.T) {
			f := testutil.NewMapFixture("FE8U")
			sample.change(f)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			d, err := s.Maps.NewDraft(0)
			if err != nil {
				t.Fatal("readable but unsafe graph", err)
			}
			if err = d.Stroke([]image.Point{{}}, 4); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Maps.Plan(d); !errors.Is(err, edit.ErrLayout) {
				t.Fatal("unsafe pointer accepted", err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				s.Document.State().Dirty {
				t.Fatal("unsafe write changed ROM")
			}
		})
	}
}

func TestMapPreviewConflicts(t *testing.T) {
	for _, mode := range []string{"revision", "layout", "foreign document", "capacity"} {
		t.Run(mode, func(t *testing.T) {
			s := testutil.Open(t, testutil.NewMapFixture("FE8U"))
			if mode == "capacity" {
				if err := s.Document.ApplyEdit(
					s.Document.State().Revision,
					"grow",
					rom.Edit{Size: rom.MaxSize},
				); err != nil {
					t.Fatal(err)
				}
			}
			d, err := s.Maps.NewDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			if err = d.Stroke([]image.Point{{}}, 4); err != nil {
				t.Fatal(err)
			}
			if err = d.Resize(3, 2, maps.MapCenter, 4); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Maps.Plan(d)
			if mode == "capacity" {
				if !errors.Is(err, space.ErrCapacity) {
					t.Fatal("full ROM accepted", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "revision":
				err = s.Records.Set(records.Chapters, []int{0}, "weather", 2)
			case "layout":
				table := s.Records.Table(records.Chapters)
				err = s.Records.UseLayout(
					records.Chapters,
					records.Layout{
						Address: table.Rows[1].Address,
						Stride:  table.Stride,
						Count:   1,
					},
				)
			case "foreign document":
				s = testutil.Open(t, testutil.NewMapFixture("FE8U"))
			}
			if err != nil {
				t.Fatal(err)
			}
			before := s.Document.Snapshot()
			if d.Current(s.Maps) {
				t.Fatal("obsolete draft accepted")
			}
			if _, err = s.Maps.Plan(d); !errors.Is(err, rom.ErrStale) {
				t.Fatal("stale draft preview", err)
			}
			if err = s.Maps.Apply(plan); !errors.Is(err, rom.ErrStale) {
				t.Fatal("stale plan applied", err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("stale map edit changed ROM")
			}
		})
	}
}
