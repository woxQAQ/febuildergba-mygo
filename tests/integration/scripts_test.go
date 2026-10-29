//go:build integration

package integration

import (
	"bytes"
	"errors"
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
			changed := max(int64(1), value^1)
			if changed == value {
				changed++
			}
			if err := draft.Set(index, game.ScriptFrames, changed); err != nil {
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
			if err != nil || got != changed ||
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

// Select a scene by its available operands and retain all surrounding commands.
// The same workflow accepts native and modified ROMs without fixed IDs or bytes.
func editScriptScene(t *testing.T, s *game.Session) {
	editScriptOperands(
		t,
		s,
		game.ScriptMoveUnit,
		game.ScriptMusic,
		game.ScriptFadeToBlack,
	)
}

func editScriptFocus(t *testing.T, s *game.Session) {
	editScriptOperands(
		t,
		s,
		game.ScriptCameraUnit,
		game.ScriptCameraPosition,
		game.ScriptCursorUnit,
		game.ScriptCursorPosition,
		game.ScriptCursorFlashUnit,
	)
}

func editScriptLoads(t *testing.T, s *game.Session) {
	editScriptOperands(t, s, game.ScriptLoadUnits2, game.ScriptLoadUnits3)
}

func editScriptControl(t *testing.T, s *game.Session) {
	editScriptOperands(
		t,
		s,
		game.ScriptCounterSet,
		game.ScriptIgnoreKeys,
		game.ScriptExecutionMode,
		game.ScriptSlotAdd,
	)
}

func editScriptPresentation(t *testing.T, s *game.Session) {
	editScriptOperands(
		t,
		s,
		game.ScriptMusicFade,
		game.ScriptMusicQuieter,
		game.ScriptTextOpaque,
	)
}

func editScriptRoutes(t *testing.T, s *game.Session) {
	editScriptOperands(
		t,
		s,
		game.ScriptLoadUnitsFiltered,
		game.ScriptLoadPartyFiltered,
	)
}

func editScriptUnitConditions(t *testing.T, s *game.Session) {
	editScriptOperands(
		t,
		s,
		game.ScriptCheckUnitExists,
		game.ScriptCheckUnitAlive,
		game.ScriptCheckUnitDeployed,
		game.ScriptCheckUnitFaction,
		game.ScriptBranchNoAliveAlly,
		game.ScriptBranchNoDeployedAlly,
		game.ScriptBranchActive,
		game.ScriptBranchTurnReached,
	)
}

func editScriptOperands(t *testing.T, s *game.Session, ops ...game.ScriptOp) {
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
			if !draft.Relocatable() {
				continue
			}
			index := slices.IndexFunc(
				draft.Instructions(),
				func(row game.ScriptInstruction) bool {
					return slices.Contains(ops, row.Op)
				},
			)
			if index < 0 {
				continue
			}
			// Known encodings can still contain unresolved RAM/slot references.
			// Establish structural capability before selecting this workflow input.
			probe, err := s.NewScriptDraft(chapter.ID, entry.Pointer)
			if err != nil {
				t.Fatal(err)
			}
			if err := probe.Insert(0, game.ScriptWait); err != nil {
				t.Fatal(err)
			}
			probeBefore := s.Document.Snapshot()
			_, err = s.PlanScriptEdit(probe)
			if errors.Is(err, game.ErrScriptStructure) {
				checkPlan(t, s, probeBefore, nil)
				continue
			}
			checkPlan(t, s, probeBefore, err)
			fields := draft.Fields(index)
			if len(fields) == 0 {
				continue
			}
			// Keep existing branch targets intact while changing the subject.
			fieldIndex := slices.IndexFunc(
				fields,
				func(field game.ScriptField) bool {
					return field.ID != game.ScriptLabelID
				},
			)
			if fieldIndex < 0 {
				continue
			}
			field := fields[fieldIndex]
			value, err := draft.Value(index, field.ID)
			if err != nil {
				t.Fatal(err)
			}
			if value < field.Min || value > field.Max {
				continue
			}
			changed := field.Min
			if changed == value {
				changed++
			}
			t.Logf(
				"chapter=%d, source=%s, instruction=%s, operand=%s",
				chapter.ID,
				entry.Source,
				draft.Instructions()[index].Op,
				field.ID,
			)
			before := s.Document.Snapshot()
			original := draft.Instructions()
			if err := draft.Set(index, field.ID, changed); err != nil {
				t.Fatal(err)
			}
			patch, err := s.PlanScriptEdit(draft)
			checkPlan(t, s, before, err)
			if err := s.ApplyScriptEdit(patch); err != nil {
				t.Fatal(err)
			}
			reopened := historyAndReopen(t, s, before)
			draft, err = reopened.NewScriptDraft(chapter.ID, entry.Pointer)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := draft.Value(
				index,
				field.ID,
			); err != nil ||
				got != changed {
				t.Fatalf("scene operand after reopening: %v", err)
			}
			for row, instruction := range draft.Instructions() {
				if row != index &&
					!bytes.Equal(instruction.Raw, original[row].Raw) {
					t.Fatal(
						"scene parameter edit changed a neighboring instruction",
					)
				}
			}
			before = reopened.Document.Snapshot()
			// A wait at entry does not change label ordering or terminal reachability.
			if err := draft.Insert(0, game.ScriptWait); err != nil {
				t.Fatal(err)
			}
			if err := draft.Set(0, game.ScriptFrames, 1); err != nil {
				t.Fatal(err)
			}
			edit, err := reopened.PlanScriptEdit(draft)
			checkPlan(t, reopened, before, err)
			if !edit.Relocated() {
				t.Fatal("scene insertion must append")
			}
			if err := reopened.ApplyScriptEdit(edit); err != nil {
				t.Fatal(err)
			}
			reopened = historyAndReopen(t, reopened, before)
			result, err := reopened.NewScriptDraft(chapter.ID, entry.Pointer)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Instructions()) != len(original)+1 ||
				result.Instructions()[0].Op != game.ScriptWait {
				t.Fatal("scene structure after reopening")
			}
			if got, err := result.Value(
				index+1,
				field.ID,
			); err != nil ||
				got != changed {
				t.Fatalf("relocated scene operand: %v", err)
			}
			return
		}
	}
	t.Skip("no relocatable scene with a supported operand")
}
