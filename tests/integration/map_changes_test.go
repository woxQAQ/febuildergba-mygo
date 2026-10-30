//go:build integration

package integration

import (
	"bytes"
	"image"
	"testing"
	"time"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
)

func editMapChanges(t *testing.T, s *game.Session) {
	t.Helper()
	chapters := requireTable(t, s, records.Chapters)
	for _, chapter := range chapters.Rows {
		draft, err := s.Maps.NewChangesDraft(chapter.ID)
		if err != nil {
			continue
		}
		tileset, err := draft.Tileset()
		if err != nil {
			continue
		}
		for index, entry := range draft.Entries() {
			if entry.Bounds.Empty() ||
				!entry.Bounds.In(image.Rectangle{Max: draft.Size()}) {
				continue
			}
			if _, err = draft.Preview(index, true); err != nil {
				continue
			}
			point := entry.Bounds.Min
			original, _ := draft.Cell(index, point.X, point.Y)
			replacement := uint16(0)
			if original == 0 {
				for _, tile := range tileset.Cells {
					if tile.Tile != 0 {
						replacement = tile.Tile
						break
					}
				}
			}
			if original == replacement {
				continue
			}
			before := s.Document.Snapshot()
			if err = draft.Stroke(
				index,
				[]image.Point{point},
				replacement,
			); err != nil {
				t.Fatal(err)
			}
			if !draft.Undo() || draft.Modified() || !draft.Redo() {
				t.Fatal("change draft history")
			}
			started := time.Now()
			plan, err := s.Maps.PlanChanges(draft)
			t.Logf(
				"chapter=%02X change=%02X planning=%s",
				chapter.ID,
				entry.ID,
				time.Since(started),
			)
			checkPlan(t, s, before, err)
			if !plan.Applicable() {
				t.Fatal("tile painting removed a script target")
			}
			if err = s.Maps.ApplyChanges(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			preserved := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				preserved[plan.Pointer():plan.Pointer()+4],
				before.Data[plan.Pointer():plan.Pointer()+4],
			)
			if !bytes.Equal(preserved, before.Data) {
				t.Fatal("old change tables, payloads or scripts were altered")
			}
			for _, use := range plan.Uses() {
				if !use.Updated {
					continue
				}
				shared, readErr := s.Maps.NewChangesDraft(use.Chapter)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if tile, _ := shared.Cell(
					index,
					point.X,
					point.Y,
				); tile != replacement {
					t.Fatal("shared change entry not updated")
				}
			}
			reopened := historyAndReopen(t, s, before)
			continued, err := reopened.Maps.NewChangesDraft(chapter.ID)
			if err != nil {
				t.Fatal(err)
			}
			if tile, _ := continued.Cell(
				index,
				point.X,
				point.Y,
			); tile != replacement {
				t.Fatal("saved change lost")
			}
			verified := 0
			for _, ref := range plan.References().References {
				if ref.After < 0 {
					continue
				}
				script, readErr := reopened.Events.NewScriptDraft(
					ref.ScriptChapter,
					ref.Pointer,
				)
				if readErr != nil {
					t.Fatal(readErr)
				}
				links, readErr := reopened.Events.MapChanges(
					script,
					ref.Instruction,
				)
				if readErr != nil {
					t.Fatal(readErr)
				}
				found := false
				for _, link := range links {
					if link.ID == ref.AfterID &&
						link.Address == plan.Destination()+ref.After*12 {
						found = true
					}
				}
				if !found {
					t.Fatal("script link did not follow relocated table")
				}
				verified++
			}
			if err = continued.Stroke(
				index,
				[]image.Point{point},
				original,
			); err != nil {
				t.Fatal(err)
			}
			next, err := reopened.Maps.PlanChanges(continued)
			checkPlan(t, reopened, reopened.Document.Snapshot(), err)
			if err = reopened.Maps.ApplyChanges(next); err != nil {
				t.Fatal(err)
			}
			t.Logf(
				"%d shared/independent uses, %d verified script references, %d incomplete-context warnings",
				len(plan.Uses()),
				verified,
				len(plan.References().Warnings),
			)
			return
		}
	}
	t.Skip("no readable map change with an editable tile")
}
