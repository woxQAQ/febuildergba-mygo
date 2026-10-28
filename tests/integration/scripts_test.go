//go:build integration

package integration

import (
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
)

func editScript(t *testing.T, s *game.Session) {
	table := requireTable(t, s, game.Chapters)
	for _, chapter := range table.Rows {
		entries, err := s.ScriptEntries(chapter.ID)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.Err != nil || entry.Address == 0 {
				continue
			}
			draft, err := s.NewScriptDraft(chapter.ID, entry.Pointer)
			if err != nil {
				t.Fatal(err)
			}
			index := slices.IndexFunc(
				draft.Instructions(),
				func(row game.ScriptInstruction) bool {
					return row.Op == game.ScriptWait && row.Editable
				},
			)
			if !draft.Complete() || index < 0 {
				continue
			}
			value, err := draft.Value(index, game.ScriptFrames)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf(
				"chapter=%d, source=%s, instruction=%d",
				chapter.ID,
				entry.Source,
				index,
			)
			before := s.Document.Snapshot()
			if err := draft.Set(index, game.ScriptFrames, value^1); err != nil {
				t.Fatal(err)
			}
			edit, err := s.PlanScriptEdit(draft)
			checkPlan(t, s, before, err)
			if err := s.ApplyScriptEdit(edit); err != nil {
				t.Fatal(err)
			}
			reopened := historyAndReopen(t, s, before)
			result, err := reopened.NewScriptDraft(chapter.ID, entry.Pointer)
			if err != nil {
				t.Fatal(err)
			}
			got, err := result.Value(index, game.ScriptFrames)
			if err != nil || got != value^1 ||
				len(result.Instructions()) != len(draft.Instructions()) {
				t.Fatalf(
					"script operand or structure changed after reopening: %v",
					err,
				)
			}
			return
		}
	}
	t.Skip("no complete script with an editable wait instruction")
}
