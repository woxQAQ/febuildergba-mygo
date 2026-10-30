package chapter_test

import (
	"bytes"
	"errors"
	"image"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestMapSharedTilesetValidation(t *testing.T) {
	f := testutil.NewMapFixture("FE8U")
	// Both chapters can display tile zero; only the first has graphics for tile four.
	f.WriteAt(
		f.GetPosition("map_layout"),
		[]byte{0x10, 6, 0, 0, 0, 2, 1, 0, 0, 0, 0},
	)
	f.WriteU8(f.GetPosition("chapters")+148+8, 4)
	f.WriteU8(f.GetPosition("chapters")+148+4, 5)
	s := testutil.Open(t, f)
	before := s.Document.Snapshot()
	if _, err := s.Maps.Read(1); err != nil {
		t.Fatal("shared chapter must initially be readable", err)
	}
	draft, err := s.Maps.NewDraft(0)
	if err != nil {
		t.Fatal(err)
	}
	if err = draft.Stroke([]image.Point{{}}, 4); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Maps.Plan(draft); !errors.Is(err, maps.ErrMap) {
		t.Fatal("incompatible shared tileset accepted", err)
	}
	if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
		s.Document.State().Dirty {
		t.Fatal("shared validation wrote ROM")
	}
	limited, err := s.Maps.NewDraft(1)
	if err != nil {
		t.Fatal(err)
	}
	tiles, err := limited.Tileset()
	if err != nil {
		t.Fatal(err)
	}
	for _, cell := range tiles.Cells {
		if cell.Tile == 4 {
			t.Fatal("unreadable tile offered as a brush")
		}
	}
	if err = limited.Stroke(
		[]image.Point{{}},
		4,
	); !errors.Is(err, edit.ErrValue) ||
		limited.Modified() {
		t.Fatal("unreadable tile changed draft", err)
	}
}

func TestMapRelocatedAndManualChapters(t *testing.T) {
	for _, version := range []string{"FE6", "FE7J", "FE7U", "FE8J", "FE8U"} {
		t.Run(version, func(t *testing.T) {
			s := testutil.Open(t, testutil.NewMapFixture(version))
			move, err := s.Records.PlanTableMove(records.Chapters, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Records.ApplyTableMove(move); err != nil {
				t.Fatal(err)
			}
			table := s.Records.Table(records.Chapters)
			if err = s.Records.UseLayout(
				records.Chapters,
				records.Layout{
					Address: table.Rows[1].Address,
					Stride:  table.Stride,
					Count:   1,
				},
			); err != nil {
				t.Fatal(err)
			}
			draft, err := s.Maps.NewDraft(0)
			if err != nil || draft.Width() != 1 || draft.Height() != 2 {
				t.Fatal("manual chapter was ignored", err)
			}
			if err = draft.Stroke([]image.Point{{}}, 0); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Maps.Plan(draft)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Maps.Apply(plan); err != nil {
				t.Fatal(err)
			}
			if err = s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			changed, err := reopened.Maps.Read(1)
			if err != nil || changed.Cells[0].Tile != 0 || changed.Width != 1 {
				t.Fatal("wrong relocated chapter map", err)
			}
			original, err := reopened.Maps.Read(0)
			if err != nil || original.Width != 2 ||
				original.Cells[1].Tile != 4 {
				t.Fatal("unselected map changed", err)
			}
		})
	}
}
