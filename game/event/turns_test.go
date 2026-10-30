package event_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestTurnReinforcementWorkflow(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewTurnFixture(fixture.Name)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			native, err := s.Events.Conditions(0, event.TurnEvents)
			if err != nil {
				t.Fatal(err)
			}
			index := 0
			if fixture.Name == "FE7U" || fixture.Name == "FE7J" {
				index = 1
			}
			condition := native.Events[index]
			if condition.Address != f.GetPosition("turn_condition") ||
				condition.Kind != 2 ||
				condition.Flag != 0x1234 ||
				condition.Start != 3 ||
				condition.End != 5 ||
				condition.Phase != 0x80 ||
				len(condition.Groups) != 1 {
				t.Fatalf("fixed condition: %+v", condition)
			}
			if index == 1 &&
				(len(native.Events[0].Raw) != 12 || len(condition.Raw) != 16 || condition.Route != 4) {
				t.Fatal("mixed FE7 record lengths or route")
			}
			d, err := s.Events.NewConditions(0, event.TurnEvents)
			if err != nil {
				t.Fatal(err)
			}
			for field, value := range map[string]int64{event.TurnStart: 7, event.TurnEnd: 0, event.TurnPhase: 0x40, event.EventFlag: 0x4321} {
				if err := d.Set(index, field, value); err != nil {
					t.Fatal(err)
				}
			}
			if err := d.Add(s.Events); err != nil {
				t.Fatal(err)
			}
			added := len(d.Events()) - 1
			units := d.Deployment(added)
			if units == nil || units.Count() != 1 {
				t.Fatal("new row must own an editable deployment")
			}
			for field, value := range map[string]int64{"deploy_character": 2, "deploy_class": 3, "deploy_level": 9, "deploy_start_x": 4, "deploy_start_y": 5, "deploy_end_x": 6, "deploy_end_y": 7, "deploy_item_1": 0x12, "deploy_ai_4": 0xCC} {
				if err := units.Set(0, field, value); err != nil {
					t.Fatal(err)
				}
			}
			if err := units.Duplicate(0); err != nil {
				t.Fatal(err)
			}
			if err := units.Add(); err != nil {
				t.Fatal(err)
			}
			if err := units.Remove(2); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("draft changed document")
			}
			plan, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Before() != len(native.Events) ||
				plan.After() != len(native.Events)+1 ||
				len(plan.Uses()) != 2 {
				t.Fatalf("incorrect plan metadata: %+v", plan)
			}
			// Later draft changes cannot mutate the approved immutable payload.
			if err := d.Set(index, event.TurnStart, 99); err != nil {
				t.Fatal(err)
			}
			if err := units.Set(0, "deploy_level", 12); err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyConditions(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			r, err := s.Events.Conditions(0, event.TurnEvents)
			if err != nil {
				t.Fatal(err)
			}
			if r.Address != plan.Destination() ||
				len(r.Events) != plan.After() {
				t.Fatal("new condition table")
			}
			got := r.Events[index]
			expected := bytes.Clone(condition.Raw)
			expected[2], expected[3], expected[8], expected[9], expected[10], expected[11] = 0x21, 0x43, 7, 0, 0x40, 0
			if !bytes.Equal(got.Raw, expected) {
				t.Fatalf("target fields: %X != %X", got.Raw, expected)
			}
			for i := range native.Events {
				if i != index &&
					!bytes.Equal(native.Events[i].Raw, r.Events[i].Raw) {
					t.Fatal("opaque or short native record changed")
				}
			}
			conditionEntry := r.Events[added]
			if conditionEntry.Start != 3 || conditionEntry.End != 0 ||
				conditionEntry.Phase != 0x80 ||
				len(conditionEntry.Groups) != 1 ||
				len(conditionEntry.Warnings) != 0 {
				t.Fatalf("generated event: %+v", conditionEntry)
			}
			group := conditionEntry.Groups[0]
			if group.Err != nil || len(group.Units) != 2 ||
				group.Units[0].Level != 9 ||
				group.Units[0].Faction != 2 ||
				group.Units[0].Start.X != 4 ||
				group.Units[0].End.Y != 7 {
				t.Fatalf("new deployment: %+v", group)
			}
			script := after.Data[conditionEntry.Script:group.Address]
			var expectedScript []byte
			switch fixture.Name {
			case "FE8U", "FE8J":
				expectedScript = []byte{
					0x40,
					0x2C,
					1,
					0,
					0,
					0,
					0,
					0,
					0x20,
					0x30,
					0,
					0,
					0x20,
					1,
					0,
					0,
				}
			case "FE7U":
				expectedScript = []byte{
					0x32,
					0,
					0,
					0,
					0,
					0,
					0,
					0,
					0x3B,
					0,
					0,
					0,
					0x0A,
					0,
					0,
					0,
					0,
					0,
					0,
					0,
				}
			case "FE7J":
				expectedScript = []byte{
					0x30,
					0,
					0,
					0,
					0,
					0,
					0,
					0,
					0x39,
					0,
					0,
					0,
					0x0A,
					0,
					0,
					0,
					0,
					0,
					0,
					0,
				}
			default:
				expectedScript = []byte{
					0x12,
					0,
					0,
					0,
					0,
					0,
					0,
					0,
					0x14,
					0,
					0,
					0,
					0x06,
					0,
					0,
					0,
					0,
					0,
					0,
					0,
				}
			}
			binary.LittleEndian.PutUint32(
				expectedScript[4:],
				uint32(0x08000000+group.Address),
			)
			if !bytes.Equal(script, expectedScript) {
				t.Fatalf(
					"literal load/wait/end script: %X != %X",
					script,
					expectedScript,
				)
			}
			preserved := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				preserved[r.Pointer:r.Pointer+4],
				before.Data[r.Pointer:r.Pointer+4],
			)
			if !bytes.Equal(preserved, before.Data) {
				t.Fatal("old resources changed")
			}
			shared, err := s.Events.Conditions(1, event.TurnEvents)
			if err != nil || shared.Address != r.Address {
				t.Fatal("shared header did not follow selected pointer")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(s.Document.Snapshot().Data, before.Data) {
				t.Fatal("atomic undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(s.Document.Snapshot().Data, after.Data) {
				t.Fatal("atomic redo")
			}
			s.Refresh()
			output := filepath.Join(t.TempDir(), "turns.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			reopenedTurns, err := reopened.Events.Conditions(
				0,
				event.TurnEvents,
			)
			if err != nil ||
				!bytes.Equal(
					reopenedTurns.Events[added].Raw,
					conditionEntry.Raw,
				) {
				t.Fatal("save/reopen lost event")
			}
			// The generated group's operand participates in ordinary deployment editing.
			unitDraft, err := reopened.Deployments.NewDraft(0, group.Pointer)
			if err != nil {
				t.Fatal(err)
			}
			if err := unitDraft.Set(0, "deploy_level", 10); err != nil {
				t.Fatal(err)
			}
			unitPlan, err := reopened.Deployments.Plan(unitDraft)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Deployments.Apply(unitPlan); err != nil {
				t.Fatal(err)
			}
			deleteDraft, err := reopened.Events.NewConditions(
				0,
				event.TurnEvents,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := deleteDraft.Remove(added); err != nil {
				t.Fatal(err)
			}
			deletion, err := reopened.Events.PlanConditions(deleteDraft)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Events.ApplyConditions(deletion); err != nil {
				t.Fatal(err)
			}
			remaining, err := reopened.Events.Conditions(
				0,
				event.TurnEvents,
			)
			if err != nil || len(remaining.Events) != len(native.Events) {
				t.Fatal("remove reinforcement")
			}
		})
	}
}

func TestTurnDraftIsolation(t *testing.T) {
	for _, name := range []string{"foreign document", "revision", "layout", "capacity", "bad value", "opaque record"} {
		t.Run(name, func(t *testing.T) {
			f := testutil.NewTurnFixture("FE8U")
			s := testutil.Open(t, f)
			d, err := s.Events.NewConditions(0, event.TurnEvents)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Add(s.Events); err != nil {
				t.Fatal(err)
			}
			p, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
			expected := error(rom.ErrStale)
			switch name {
			case "foreign document":
				s = testutil.Open(t, f)
			case "revision":
				if err := s.Records.Set(
					records.Units,
					[]int{1},
					"base.hp",
					9,
				); err != nil {
					t.Fatal(err)
				}
			case "layout":
				table := s.Records.Table(records.Chapters)
				if err := s.Records.UseLayout(
					records.Chapters,
					table.Layout,
				); err != nil {
					t.Fatal(err)
				}
			case "capacity":
				snapshot := s.Document.Snapshot()
				if err := s.Document.ApplyEdit(
					snapshot.Revision,
					"capacity",
					rom.Edit{Size: rom.MaxSize},
				); err != nil {
					t.Fatal(err)
				}
				s.Refresh()
				d, err = s.Events.NewConditions(0, event.TurnEvents)
				if err != nil {
					t.Fatal(err)
				}
				if err := d.Add(s.Events); err != nil {
					t.Fatal(err)
				}
				before := s.Document.Snapshot()
				_, err = s.Events.PlanConditions(d)
				if !errors.Is(err, space.ErrCapacity) ||
					!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
					t.Fatalf("capacity failure: %v", err)
				}
				return
			case "bad value", "opaque record":
				index, field, value := 0, event.TurnStart, int64(256)
				if name == "opaque record" {
					index, value = 1, 3
				}
				raw := d.Events()[index].Raw
				if err := d.Set(
					index,
					field,
					value,
				); err == nil ||
					!bytes.Equal(raw, d.Events()[index].Raw) {
					t.Fatal("invalid field mutated draft")
				}
				return
			}
			before := s.Document.Snapshot()
			if err := s.Events.ApplyConditions(
				p,
			); !errors.Is(err, expected) ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatalf("stale edit: %v", err)
			}
		})
	}
}
