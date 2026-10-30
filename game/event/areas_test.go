package event_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"image"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func TestAreaReinforcementWorkflow(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewAreaFixture(fixture.Name)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			native, err := s.Events.Conditions(0, event.AlwaysEvents)
			if err != nil {
				t.Fatal(err)
			}
			if native.Pointer != f.GetPosition("deploy_header")+12 ||
				len(native.Events) != 3 ||
				len(native.Events[1].Raw) != 12 ||
				native.Events[1].Area() != image.Rect(2, 3, 5, 7) ||
				len(native.Events[1].Groups) != 1 {
				t.Fatalf("fixed native area conditions: %+v", native)
			}
			d, err := s.Events.NewConditions(0, event.AlwaysEvents)
			if err != nil {
				t.Fatal(err)
			}
			for field, value := range map[string]int64{event.AreaX1: 7, event.AreaY1: 8, event.AreaX2: 2, event.AreaY2: 3} {
				if err := d.Set(1, field, value); err != nil {
					t.Fatal(err)
				}
			}
			reversed := d.Events()[1].Area()
			if fixture.Name == "FE8U" || fixture.Name == "FE8J" {
				if reversed != image.Rect(2, 3, 8, 9) {
					t.Fatal("FE8 reversed endpoints lost")
				}
			} else if !reversed.Empty() {
				t.Fatal("reversed FE6/7 range must not trigger")
			}
			for field, value := range map[string]int64{event.AreaX1: -3, event.AreaY1: 4, event.AreaX2: 6, event.AreaY2: 8, event.EventFlag: 0x29} {
				if err := d.Set(1, field, value); err != nil {
					t.Fatal(err)
				}
			}
			if err := d.Add(s.Events); err != nil {
				t.Fatal(err)
			}
			added := len(d.Events()) - 1
			if err := d.Set(added, event.EventFlag, 0x2B); err != nil {
				t.Fatal(err)
			}
			for field, value := range map[string]int64{event.AreaX1: 1, event.AreaY1: 2, event.AreaX2: 3, event.AreaY2: 4} {
				if err := d.Set(added, field, value); err != nil {
					t.Fatal(err)
				}
			}
			units := d.Deployment(added)
			for field, value := range map[string]int64{"deploy_character": 2, "deploy_class": 3, "deploy_level": 9, "deploy_start_x": 4, "deploy_start_y": 5, "deploy_end_x": 6, "deploy_end_y": 7, "deploy_item_1": 0x12} {
				if err := units.Set(0, field, value); err != nil {
					t.Fatal(err)
				}
			}
			if err := units.Duplicate(0); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("draft mutated document")
			}
			plan, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Before() != 3 || plan.After() != 4 ||
				len(plan.Uses()) != 2 {
				t.Fatal("shared header preview")
			}
			if err := d.Set(1, event.AreaX1, 99); err != nil {
				t.Fatal(err)
			}
			if err := units.Set(0, "deploy_level", 12); err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyConditions(plan); err != nil {
				t.Fatal(err)
			}
			result, err := s.Events.Conditions(0, event.AlwaysEvents)
			if err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			expected := bytes.Clone(native.Events[1].Raw)
			expected[2], expected[3], expected[8], expected[9], expected[10], expected[11] = 0x29, 0, 0xFD, 4, 6, 8
			if !bytes.Equal(expected, result.Events[1].Raw) ||
				result.Events[1].Area() != image.Rect(-3, 4, 7, 9) {
				t.Fatal("signed bounds or immutable preview")
			}
			for _, i := range []int{0, 2} {
				if !bytes.Equal(native.Events[i].Raw, result.Events[i].Raw) {
					t.Fatal("non-area condition changed")
				}
			}
			condition := result.Events[added]
			if condition.Kind != 11 || condition.Flag != 0x2B ||
				condition.Area() != image.Rect(1, 2, 4, 5) ||
				len(condition.Groups) != 1 ||
				len(condition.Warnings) != 0 {
				t.Fatalf("new area event: %+v", condition)
			}
			group := condition.Groups[0]
			if group.Err != nil || len(group.Units) != 2 ||
				group.Units[0].Level != 9 ||
				group.Units[0].Faction != 2 ||
				group.Units[0].Start != image.Pt(4, 5) ||
				group.Units[0].End != image.Pt(6, 7) {
				t.Fatalf("area deployment: %+v", group)
			}
			literals := map[string]string{
				"FE8U": "402C0100000000002030000020010000",
				"FE8J": "402C0100000000002030000020010000",
				"FE7U": "32000000000000003B0000000A00000000000000",
				"FE7J": "3000000000000000390000000A00000000000000",
				"FE6":  "1200000000000000140000000600000000000000",
			}
			script, err := hex.DecodeString(literals[fixture.Name])
			if err != nil {
				t.Fatal(err)
			}
			actual := bytes.Clone(
				after.Data[condition.Script : condition.Script+len(script)],
			)
			clear(actual[4:8])
			if !bytes.Equal(actual, script) {
				t.Fatalf("native script literal: %X", actual)
			}
			prefix := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				prefix[native.Pointer:native.Pointer+4],
				before.Data[native.Pointer:native.Pointer+4],
			)
			if !bytes.Equal(prefix, before.Data) {
				t.Fatal("unrelated pointer, turn table or old resource changed")
			}
			shared, err := s.Events.Conditions(1, event.AlwaysEvents)
			if err != nil || shared.Address != result.Address {
				t.Fatal("shared pointer not updated")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(s.Document.Snapshot().Data, before.Data) {
				t.Fatal("atomic undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(s.Document.Snapshot().Data, after.Data) {
				t.Fatal("atomic redo")
			}
			path := filepath.Join(t.TempDir(), "areas.gba")
			if err := s.Document.SaveAs(path, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			deployed, err := reopened.Deployments.NewDraft(0, group.Pointer)
			if err != nil {
				t.Fatal(err)
			}
			if err := deployed.Set(0, "deploy_level", 10); err != nil {
				t.Fatal(err)
			}
			deployment, err := reopened.Deployments.Plan(deployed)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Deployments.Apply(deployment); err != nil {
				t.Fatal(err)
			}
			removal, err := reopened.Events.NewConditions(
				0,
				event.AlwaysEvents,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := removal.Remove(added); err != nil {
				t.Fatal(err)
			}
			deletion, err := reopened.Events.PlanConditions(removal)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Events.ApplyConditions(deletion); err != nil {
				t.Fatal(err)
			}
			remaining, err := reopened.Events.Conditions(
				0,
				event.AlwaysEvents,
			)
			if err != nil || len(remaining.Events) != 3 ||
				remaining.Events[1].X1 != -3 {
				t.Fatal("reopened conditions or deletion")
			}
		})
	}
}

func TestAreaDraftIsolation(t *testing.T) {
	for _, name := range []string{"foreign document", "revision", "layout", "opaque", "signed bounds", "unknown table"} {
		t.Run(name, func(t *testing.T) {
			f := testutil.NewAreaFixture("FE8U")
			s := testutil.Open(t, f)
			d, err := s.Events.NewConditions(0, event.AlwaysEvents)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Add(s.Events); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
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
				if err := s.Records.UseLayout(
					records.Chapters,
					s.Records.Table(records.Chapters).Layout,
				); err != nil {
					t.Fatal(err)
				}
			case "opaque", "signed bounds":
				index, field, value := 1, event.AreaX1, int64(128)
				if name == "opaque" {
					index, value = 0, 1
				}
				raw := d.Events()[index].Raw
				if err := d.Set(
					index,
					field,
					value,
				); err == nil ||
					!bytes.Equal(raw, d.Events()[index].Raw) {
					t.Fatal("rejected edit mutated draft")
				}
				if err := d.Set(1, event.AreaY1, -129); err == nil {
					t.Fatal("signed underflow")
				}
				return
			case "unknown table":
				if _, err := s.Events.NewConditions(
					0,
					event.Table(255),
				); !errors.Is(
					err,
					event.ErrEventConditions,
				) {
					t.Fatal("unknown header slot accepted")
				}
				return
			}
			before := s.Document.Snapshot()
			if err := s.Events.ApplyConditions(
				plan,
			); !errors.Is(err, rom.ErrStale) ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("stale area edit changed document")
			}
		})
	}
}
