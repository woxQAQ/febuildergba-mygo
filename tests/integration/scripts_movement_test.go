//go:build integration

package integration

import (
	"errors"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
)

func editScriptMovement(t *testing.T, s *game.Session) {
	table := requireTable(t, s, records.Chapters)
	for _, chapter := range table.Rows {
		entries, err := s.Events.ScriptEntries(chapter.ID)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.Err != nil || entry.Address == 0 {
				continue
			}
			d, err := s.Events.NewScriptDraft(chapter.ID, entry.Pointer)
			if err != nil {
				t.Fatal(err)
			}
			if !d.Complete() {
				continue
			}
			for index := range d.Instructions() {
				for _, field := range d.Fields(index) {
					if field.Target != event.ScriptMovementTarget {
						continue
					}
					steps, err := d.Movement(index, field.ID)
					if err != nil {
						continue
					}
					at := slices.IndexFunc(
						steps,
						func(step event.ScriptMovementStep) bool { return step.Command < 4 },
					)
					if at < 0 {
						continue
					}
					steps[at].Command ^= 1
					if err := d.SetMovement(
						index,
						field.ID,
						steps,
					); errors.Is(
						err,
						edit.ErrValue,
					) {
						continue
					} else if err != nil {
						t.Fatal(err)
					}
					before := s.Document.Snapshot()
					edit, err := s.Events.PlanScriptEdit(d)
					checkPlan(t, s, before, err)
					if edit.MovementCount() != 1 {
						t.Fatal("movement preview count")
					}
					if err := s.Events.ApplyScriptEdit(edit); err != nil {
						t.Fatal(err)
					}
					reopened := historyAndReopen(t, s, before)
					again, err := reopened.Events.NewScriptDraft(
						chapter.ID,
						entry.Pointer,
					)
					if err != nil {
						t.Fatal(err)
					}
					got, err := again.Movement(index, field.ID)
					if err != nil || !slices.Equal(got, steps) {
						t.Fatal("movement save/reopen", err)
					}
					t.Logf(
						"chapter=%d source=%s instruction=%d operand=%s",
						chapter.ID,
						entry.Source,
						index,
						field.ID,
					)
					return
				}
			}
		}
	}
	t.Skip("no complete script with an editable native movement path")
}
