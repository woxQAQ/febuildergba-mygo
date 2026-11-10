//go:build integration

package integration

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
)

func browseQuoteScripts(t *testing.T, s *game.Session) {
	if !s.Events.QuoteScriptsAvailable() {
		t.Skip("native dialogue tables have no script fields")
	}
	before := s.Document.Snapshot()
	entries, err := s.Events.ScriptEntries(event.QuoteScripts)
	if err != nil {
		t.Fatal(err)
	}
	browsed := 0
	var textID uint16
	var content string
	var replacement *text.Plan
	unencodable := 0
	for _, entry := range entries {
		if entry.Err != nil || entry.Address == 0 {
			continue
		}
		program, err := s.Events.Script(event.QuoteScripts, entry.Pointer)
		if err != nil || !program.ReadOnly ||
			program.Entry.Chapter != event.QuoteScripts ||
			program.Entry.Address != entry.Address {
			t.Fatal(
				"declared global script could not be browsed independently",
				entry,
				err,
			)
		}
		for _, row := range program.Instructions {
			if row.Editable {
				t.Fatal("read-only script exposed instruction edits", entry)
			}
		}
		draft, err := s.Events.NewScriptDraft(event.QuoteScripts, entry.Pointer)
		if err != nil || !draft.ReadOnly() || draft.Relocatable() {
			t.Fatal("global draft permissions", err)
		}
		browsed++
		if textID != 0 {
			continue
		}
		for index, instruction := range draft.Instructions() {
			for _, operand := range draft.TextOperands(index) {
				value, err := s.Texts.Value(operand.ID)
				if err != nil || !value.Editable || value.Content == "" ||
					len(value.Content) > 2048 {
					continue
				}
				uses, _ := s.Events.TextUses(operand.ID)
				if !slices.ContainsFunc(uses, func(use text.Use) bool {
					return use.ID == event.QuoteScripts && use.Pointer == entry.Pointer &&
						use.Source == entry.Source &&
						use.Address == instruction.Address &&
						use.Field == operand.Field
				}) {
					continue
				}
				plan, err := s.Texts.PlanID(
					operand.ID,
					value.Content+value.Content,
				)
				if errors.Is(err, text.ErrTextEncoding) {
					unencodable++
					continue
				}
				checkPlan(t, s, before, err)
				textID, content, replacement = operand.ID, value.Content, plan
				break
			}
			if textID != 0 {
				break
			}
		}
	}
	if browsed == 0 {
		t.Skip("no readable native battle or defeat scripts")
	}
	if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
		before.Revision != s.Document.Snapshot().Revision {
		t.Fatal("browsing changed ROM bytes or revision")
	}
	t.Logf(
		"native entries=%d, readable scripts=%d, unencodable text candidates=%d",
		len(entries),
		browsed,
		unencodable,
	)
	t.Run("text", func(t *testing.T) {
		if textID == 0 {
			t.Skip("no encodable existing text in a confirmed quote script")
		}
		uses, complete := s.Events.TextUses(textID)
		if replacement.UsesComplete() != complete {
			t.Fatal("text preview changed discovery completeness")
		}
		if err := s.Texts.Apply(replacement); err != nil {
			t.Fatal(err)
		}
		reopened := historyAndReopen(t, s, before)
		saved, savedComplete := reopened.Events.TextUses(textID)
		if !slices.Equal(saved, uses) || savedComplete != complete {
			t.Fatal("text edit changed script references after save and reopen")
		}
		value, err := reopened.Texts.Value(textID)
		if err != nil || value.Content != content+content {
			t.Fatal("quote script text replacement did not persist", err)
		}
		t.Logf(
			"text=%04X, confirmed script uses=%d, complete=%v",
			textID,
			len(uses),
			complete,
		)
	})
}
