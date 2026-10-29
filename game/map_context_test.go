package game

import (
	"bytes"
	"errors"
	"image"
	"testing"

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
	s := openFixture(t, f)
	before := s.Document.Snapshot()
	if _, err := s.ChapterMap(1); err != nil {
		t.Fatal("shared chapter must initially be readable", err)
	}
	draft, err := s.NewMapDraft(0)
	if err != nil {
		t.Fatal(err)
	}
	if err = draft.Stroke([]image.Point{{}}, 4); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PlanMapEdit(draft); !errors.Is(err, ErrMap) {
		t.Fatal("incompatible shared tileset accepted", err)
	}
	if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
		s.Document.State().Dirty {
		t.Fatal("shared validation wrote ROM")
	}
	limited, err := s.NewMapDraft(1)
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
	); !errors.Is(err, ErrValue) ||
		limited.Modified() {
		t.Fatal("unreadable tile changed draft", err)
	}
}

func TestMapRelocatedAndManualChapters(t *testing.T) {
	for _, version := range []string{"FE6", "FE7J", "FE7U", "FE8J", "FE8U"} {
		t.Run(version, func(t *testing.T) {
			s := openFixture(t, testutil.NewMapFixture(version))
			move, err := s.PlanTableMove(Chapters, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.ApplyTableMove(move); err != nil {
				t.Fatal(err)
			}
			table := s.Table(Chapters)
			if err = s.UseLayout(
				Chapters,
				Layout{
					Address: table.Rows[1].Address,
					Stride:  table.Stride,
					Count:   1,
				},
			); err != nil {
				t.Fatal(err)
			}
			draft, err := s.NewMapDraft(0)
			if err != nil || draft.Width() != 1 || draft.Height() != 2 {
				t.Fatal("manual chapter was ignored", err)
			}
			if err = draft.Stroke([]image.Point{{}}, 0); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanMapEdit(draft)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.ApplyMapEdit(plan); err != nil {
				t.Fatal(err)
			}
			if err = s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			changed, err := reopened.ChapterMap(1)
			if err != nil || changed.Cells[0].Tile != 0 || changed.Width != 1 {
				t.Fatal("wrong relocated chapter map", err)
			}
			original, err := reopened.ChapterMap(0)
			if err != nil || original.Width != 2 ||
				original.Cells[1].Tile != 4 {
				t.Fatal("unselected map changed", err)
			}
		})
	}
}
