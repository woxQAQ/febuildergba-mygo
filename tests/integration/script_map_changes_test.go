//go:build integration

package integration

import (
	"image"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
)

// Follow a confirmed native script operand into the change editor, then verify
// the same operand resolves through the updated resource entry after reopening.
func linkMapChange(t *testing.T, s *game.Session) {
	t.Helper()
	chapters := requireTable(t, s, records.Chapters)
	for _, chapter := range chapters.Rows {
		entries, err := s.Events.ScriptEntries(chapter.ID)
		if err != nil {
			continue
		}
		seen := make(map[int]bool)
		for _, entry := range entries {
			if entry.Err != nil || entry.Address == 0 || seen[entry.Address] {
				continue
			}
			seen[entry.Address] = true
			script, err := s.Events.NewScriptDraft(chapter.ID, entry.Pointer)
			if err != nil {
				continue
			}
			for instruction := range script.Instructions() {
				if !script.IsMapChange(instruction) {
					continue
				}
				links, err := s.Events.MapChanges(script, instruction)
				if err != nil || len(links) == 0 {
					continue
				}
				for _, link := range links {
					draft, err := s.Maps.NewChangesDraft(link.Chapter)
					if err != nil {
						continue
					}
					tiles, err := draft.Tileset()
					if err != nil {
						continue
					}
					for index, change := range draft.Entries() {
						if change.Address != link.Address ||
							change.Bounds.Empty() ||
							!change.Bounds.In(
								image.Rectangle{Max: draft.Size()},
							) {
							continue
						}
						if _, err = draft.Preview(index, true); err != nil {
							continue
						}
						point := change.Bounds.Min
						original, _ := draft.Cell(index, point.X, point.Y)
						replacement := uint16(0)
						if original == 0 {
							for _, tile := range tiles.Cells {
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
						plan, err := s.Maps.PlanChanges(draft)
						checkPlan(t, s, before, err)
						if err = s.Maps.ApplyChanges(plan); err != nil {
							t.Fatal(err)
						}
						reopened := historyAndReopen(t, s, before)
						script, err = reopened.Events.NewScriptDraft(
							chapter.ID,
							entry.Pointer,
						)
						if err != nil {
							t.Fatal(err)
						}
						updated, err := reopened.Events.MapChanges(
							script,
							instruction,
						)
						if err != nil {
							t.Fatal(err)
						}
						found := false
						for _, target := range updated {
							if target.ID == link.ID &&
								target.Address == plan.Destination()+index*12 {
								found = true
							}
						}
						if !found {
							t.Fatal(
								"script target did not follow appended change table",
							)
						}
						continued, err := reopened.Maps.NewChangesDraft(
							link.Chapter,
						)
						if err != nil {
							t.Fatal(err)
						}
						if tile, _ := continued.Cell(
							index,
							point.X,
							point.Y,
						); tile != replacement {
							t.Fatal("linked change lost after reopen")
						}
						t.Logf(
							"script chapter=%02X address=0x%X instruction=%d -> map chapter=%02X change=%02X",
							chapter.ID,
							entry.Address,
							instruction,
							link.Chapter,
							link.ID,
						)
						return
					}
				}
			}
		}
	}
	t.Skip("no statically confirmed script link to an editable map change")
}
