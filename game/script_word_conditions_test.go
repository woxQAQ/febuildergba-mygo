package game

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestScriptWordConditionWorkflow(t *testing.T) {
	fixtures := map[string]string{
		"FE6": `2100000009000000 240000000900000001000000 01000000 1B00000009000000 1200000000802508 00000000`,
		"FE7J": `4800000009000000 4900000009000000 4C0001000900000001000000 4C0000000900000001000000
   4D00000009000000 4E00000009000000 4F00000009000000 4F00010009000000
   5000000009000000 5100000009000000 5200000009000000 53000000090000000100000007000000
   5400030009000000 550001000900000001000000 550000000900000001000000
   01000000 4200000009000000 3000000000802508 00000000`,
		"FE7U": `4A00000009000000 4B00000009000000 4E0001000900000001000000 4E0000000900000001000000
   4F00000009000000 5000000009000000 5100000009000000 5100010009000000
   5200000009000000 5300000009000000 5400000009000000 55000000090000000100000007000000
   5600030009000000 570001000900000001000000 570000000900000001000000
   01000000 4400000009000000 3200000000802508 00000000`,
	}
	for version, encoded := range fixtures {
		t.Run(version, func(t *testing.T) {
			raw, err := hex.DecodeString(
				strings.Join(strings.Fields(encoded), ""),
			)
			if err != nil {
				t.Fatal(err)
			}
			f := testutil.NewScriptFixture(version)
			root, site := f.GetPosition(
				"script_root",
			), f.GetPosition(
				"script_site",
			)
			units := f.GetPosition("objective_units")
			f.WriteAt(loadUnitsA, f.Bytes()[units:units+36])
			f.WriteAt(root, raw)
			s := openFixture(t, f)
			before := s.Document.Snapshot()
			d, err := s.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			if !d.Complete() || !d.Relocatable() {
				t.Fatal("word condition boundaries")
			}
			active := sceneInstruction(t, d, ScriptBranchActive)
			if err := d.Set(active, ScriptCharacter, 2); err != nil {
				t.Fatal(err)
			}
			changed := bytes.Clone(raw)
			changed[d.Instructions()[active].Address-root+8] = 2
			if version != "FE6" {
				turn := sceneInstruction(t, d, ScriptBranchTurnReached)
				if err := d.Set(turn, ScriptTurn, 4); err != nil {
					t.Fatal(err)
				}
				changed[d.Instructions()[turn].Address-root+2] = 4
				for _, op := range []ScriptOp{ScriptBranchNotYes, ScriptBranchNotYesAlternate, ScriptBranchNormal, ScriptBranchHard, ScriptBranchDeployed, ScriptBranchNotDeployed} {
					sceneInstruction(t, d, op)
				}
			}
			plan, err := s.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			want := bytes.Clone(before.Data)
			copy(want[root:], changed)
			if !bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal(
					"condition operand changed reserved bits or another branch",
				)
			}
			if !s.Document.Undo() {
				t.Fatal("condition undo")
			}
			d, err = s.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Remove(
				sceneInstruction(t, d, ScriptLabel),
			); err != nil {
				t.Fatal(err)
			}
			if _, err := s.PlanScriptEdit(
				d,
			); !errors.Is(err, ErrScript) ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("deleted branch target accepted", err)
			}
			d, err = s.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Duplicate(0); err != nil {
				t.Fatal(err)
			}
			plan, err = s.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			moved, err := s.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			load := sceneInstruction(t, moved, ScriptLoadUnits)
			ref, err := moved.Reference(load, ScriptUnitPointer)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.NewDeploymentDraft(0, ref.Pointer); err != nil {
				t.Fatal("branch hid deployment beyond early return", err)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("condition structural undo")
			}
		})
	}
}
