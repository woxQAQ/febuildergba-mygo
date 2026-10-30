//go:build integration

package integration

import (
	"bytes"
	"image"
	"testing"
	"time"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
)

func editMap(t *testing.T, s *game.Session) {
	t.Helper()
	chapters := requireTable(t, s, records.Chapters)
	for _, row := range chapters.Rows {
		draft, err := s.Maps.NewDraft(row.ID)
		if err != nil {
			continue
		}
		tileset, err := draft.Tileset()
		if err != nil {
			continue
		}
		original, _ := draft.Cell(0, 0)
		var replacement *maps.Cell
		for _, tile := range tileset.Cells {
			if tile.Tile != original.Tile {
				replacement = &tile
				break
			}
		}
		if replacement == nil {
			continue
		}
		before := s.Document.Snapshot()
		if err = draft.Stroke(
			[]image.Point{{}, {X: min(2, draft.Width()-1)}},
			replacement.Tile,
		); err != nil {
			t.Fatal(err)
		}
		if !draft.Undo() || draft.Modified() || !draft.Redo() {
			t.Fatal("map stroke history")
		}
		if err = draft.Fill(
			image.Rect(0, 0, min(2, draft.Width()), min(2, draft.Height())),
			replacement.Tile,
		); err != nil {
			t.Fatal(err)
		}
		started := time.Now()
		plan, err := s.Maps.Plan(draft)
		t.Logf(
			"chapter=%02X, map=%dx%d, planning=%s",
			row.ID,
			draft.Width(),
			draft.Height(),
			time.Since(started),
		)
		checkPlan(t, s, before, err)
		if err = s.Maps.Apply(plan); err != nil {
			t.Fatal(err)
		}
		after := s.Document.Snapshot()
		preserved := bytes.Clone(after.Data[:len(before.Data)])
		copy(
			preserved[plan.Pointer():plan.Pointer()+4],
			before.Data[plan.Pointer():plan.Pointer()+4],
		)
		if !bytes.Equal(preserved, before.Data) {
			t.Fatal("map write altered old resources or independent references")
		}
		for _, use := range plan.Uses() {
			if !use.Updated {
				continue
			}
			view, readErr := s.Maps.Read(use.Chapter)
			if readErr != nil || view.Cells[0].Tile != replacement.Tile {
				t.Fatal("shared map entry did not follow the edit", readErr)
			}
		}
		reopened := historyAndReopen(t, s, before)
		continued, err := reopened.Maps.NewDraft(row.ID)
		if err != nil {
			t.Fatal(err)
		}
		if cell, _ := continued.Cell(0, 0); cell.Tile != replacement.Tile {
			t.Fatal("map changed after reopen")
		}
		if err = continued.Stroke(
			[]image.Point{{}},
			original.Tile,
		); err != nil {
			t.Fatal(err)
		}
		next, err := reopened.Maps.Plan(continued)
		checkPlan(t, reopened, reopened.Document.Snapshot(), err)
		if err = reopened.Maps.Apply(next); err != nil {
			t.Fatal(err)
		}
		t.Logf(
			"edited %d cells, %d confirmed chapter uses",
			plan.ChangedCells(),
			len(plan.Uses()),
		)
		return
	}
	t.Skip("no readable map with an alternative tile")
}
