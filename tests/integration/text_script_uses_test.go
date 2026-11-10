//go:build integration

package integration

import (
	"slices"
	"testing"
	"time"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
)

func editScriptTextWithUses(t *testing.T, s *game.Session) {
	entries, err := s.Texts.Entries()
	if err != nil {
		t.Skipf("text table unavailable: %v", err)
	}
	started := time.Now()
	for _, entry := range entries {
		if entry.Err != nil || !entry.Editable || entry.Content == "" ||
			len(entry.Content) > 2048 {
			continue
		}
		uses, complete := s.Events.TextUses(entry.ID)
		if len(uses) == 0 {
			continue
		}
		t.Logf(
			"text ID=%04X, script uses=%d, complete=%v, discovery=%s",
			entry.ID,
			len(uses),
			complete,
			time.Since(started),
		)
		for _, use := range uses {
			program, err := s.Events.Script(use.ID, use.Pointer)
			if err != nil ||
				!slices.ContainsFunc(
					program.Instructions,
					func(row event.ScriptInstruction) bool { return row.Address == use.Address },
				) {
				t.Fatal(
					"text use does not navigate to its declared instruction",
					use,
					err,
				)
			}
		}
		before := s.Document.Snapshot()
		content := entry.Content + entry.Content
		plan, err := s.Texts.PlanID(entry.ID, content)
		checkPlan(t, s, before, err)
		known := slices.DeleteFunc(
			plan.Uses(),
			func(use text.Use) bool { return use.Kind != event.TextUse },
		)
		if !slices.Equal(known, uses) || plan.UsesComplete() != complete {
			t.Fatal("text preview changed the script usage scope")
		}
		if err := s.Texts.Apply(plan); err != nil {
			t.Fatal(err)
		}
		reopened := historyAndReopen(t, s, before)
		value, err := reopened.Texts.Read(entry.ID)
		if err != nil || value.Content != content {
			t.Fatal("edited script text did not survive save and reopen", err)
		}
		saved, savedComplete := reopened.Events.TextUses(entry.ID)
		if !slices.Equal(saved, uses) || savedComplete != complete {
			t.Fatal("text replacement changed referencing script operands")
		}
		return
	}
	t.Skip("no editable bounded text with a confirmed static script use")
}
