package event_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"image"
	"path/filepath"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestLocationEditingWorkflow(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewLocationFixture(fixture.Name)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			native, err := s.Events.Conditions(0, event.LocationEvents)
			if err != nil {
				t.Fatal(err)
			}
			if native.Pointer != f.GetPosition("deploy_header")+8 ||
				len(native.Events) != 11 {
				t.Fatalf("native location table: %+v", native)
			}
			wantKinds := []event.LocationKind{
				event.House,
				event.Village,
				event.Chest,
				event.Chest,
				event.Door,
				event.Seize,
				event.Shop,
				event.ScriptChest,
				"",
				event.VillageCenter,
				"",
			}
			if fixture.Name == "FE8U" || fixture.Name == "FE8J" {
				wantKinds[7] = event.RandomChest
			}
			for i, condition := range native.Events {
				if condition.Location.Kind != wantKinds[i] {
					t.Fatalf("location %d: %+v", i, condition)
				}
				if condition.OperandKind != event.ConditionScript &&
					(condition.Script != 0 || len(condition.Texts) != 0 || len(condition.Groups) != 0) {
					t.Fatalf("non-script operand scanned: %+v", condition)
				}
			}
			if !slices.Equal(native.Events[0].Texts, []uint16{1}) ||
				len(native.Events[0].Groups) != 1 ||
				native.Events[2].Location.Money != 0xBEEF ||
				native.Events[4].Script != 1 {
				t.Fatal("script, reward and native marker were conflated")
			}
			entries, err := s.Events.ScriptEntries(0)
			if err != nil {
				t.Fatal(err)
			}
			for _, index := range []int{2, 3, 6, 8, 9, 10} {
				if slices.ContainsFunc(
					entries,
					func(entry event.ScriptEntry) bool { return entry.Pointer == native.Events[index].Address+4 },
				) {
					t.Fatalf(
						"non-script location %d entered script index",
						index,
					)
				}
			}
			d, err := s.Events.NewConditions(0, event.LocationEvents)
			if err != nil {
				t.Fatal(err)
			}
			if len(d.Fields(6)) != 3 || len(d.Fields(8)) != 0 || d.Modified() {
				t.Fatal("shop coordinates or opaque location editability")
			}
			if err := d.SetLocationPosition(0, image.Pt(3, 4)); err != nil {
				t.Fatal(err)
			}
			if err := d.SetChestReward(
				3,
				native.Events[3].Location.Item,
				1234,
			); err != nil {
				t.Fatal(err)
			}
			for _, kind := range []event.LocationKind{event.House, event.Village, event.Chest, event.Door, event.Seize} {
				if err := d.AddLocation(s.Events, kind); err != nil {
					t.Fatal(err)
				}
			}
			if err := d.SetDialogue(11, 1); err != nil {
				t.Fatal(err)
			}
			if err := d.SetDialogue(12, 2); err != nil {
				t.Fatal(err)
			}
			if err := d.CopyLocation(12); err != nil {
				t.Fatal(err)
			}
			if err := d.SetDialogue(16, 1); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(plan.Uses(), []int{0, 1}) || plan.Before() != 11 ||
				plan.After() != 17 {
				t.Fatal("shared location preview")
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("preview wrote ROM")
			}
			if err := d.SetLocationPosition(0, image.Pt(20, 30)); err != nil {
				t.Fatal(err)
			}
			if err := d.SetDialogue(12, 1); err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyConditions(plan); err != nil {
				t.Fatal(err)
			}
			result, err := s.Events.Conditions(0, event.LocationEvents)
			if err != nil {
				t.Fatal(err)
			}
			want := bytes.Clone(native.Events[0].Raw)
			want[8], want[9] = 3, 4
			if !bytes.Equal(result.Events[0].Raw, want) ||
				result.Events[3].Location.Money != 1234 {
				t.Fatal("plan changed ignored bytes or followed mutable draft")
			}
			for _, i := range []int{1, 2, 4, 5, 6, 7, 8, 9, 10} {
				if !bytes.Equal(result.Events[i].Raw, native.Events[i].Raw) {
					t.Fatalf("unselected row %d changed", i)
				}
			}
			if !slices.Equal(result.Events[12].Texts, []uint16{2}) ||
				!slices.Equal(result.Events[16].Texts, []uint16{1}) {
				t.Fatal("copied dialog drafts share ownership")
			}
			if result.Events[13].Location.Item != 1 ||
				result.Events[14].Script != 1 ||
				result.Events[15].Flag != 3 ||
				result.Events[15].Script != f.GetPosition("objective_ending") {
				t.Fatal("native chest, door or seize template")
			}
			literal := "0D00000002000000090000000A00000000000000"
			if fixture.Name == "FE6" {
				literal = "0700000002000000050000000600000000000000"
			}
			if fixture.Name == "FE8U" || fixture.Name == "FE8J" {
				literal = "201A0000201B0200201D0000221B000020010000"
			}
			script, err := hex.DecodeString(literal)
			if err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			at := result.Events[12].Script
			if !bytes.Equal(after.Data[at:at+len(script)], script) {
				t.Fatal("new visit differs from native literal")
			}
			prefix := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				prefix[native.Pointer:native.Pointer+4],
				before.Data[native.Pointer:native.Pointer+4],
			)
			if !bytes.Equal(prefix, before.Data) {
				t.Fatal("old resources or unrelated references changed")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				!s.Document.Redo() ||
				!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("locations did not form one atomic history operation")
			}
			path := filepath.Join(t.TempDir(), "locations.gba")
			if err := s.Document.SaveAs(path, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			again, err := reopened.Events.NewConditions(0, event.LocationEvents)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.EqualFunc(
				result.Events,
				again.Events(),
				func(a, b event.Condition) bool { return bytes.Equal(a.Raw, b.Raw) },
			) {
				t.Fatal("save/reopen changed locations")
			}
			if err := again.Remove(16); err != nil {
				t.Fatal(err)
			}
			deletion, err := reopened.Events.PlanConditions(again)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Events.ApplyConditions(deletion); err != nil {
				t.Fatal(err)
			}
			remaining, err := reopened.Events.Conditions(
				0,
				event.LocationEvents,
			)
			if err != nil || len(remaining.Events) != 16 {
				t.Fatal("delete after reopen", err)
			}
			program, err := reopened.Events.NewScriptDraft(
				0,
				remaining.Events[0].Address+4,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := program.Insert(0, event.ScriptWait); err != nil {
				t.Fatal(err)
			}
			programPlan, err := reopened.Events.PlanScriptEdit(program)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Events.ApplyScriptEdit(programPlan); err != nil {
				t.Fatal(err)
			}
			linked, err := reopened.Events.Conditions(0, event.LocationEvents)
			if err != nil ||
				linked.Events[0].Script == remaining.Events[0].Script ||
				linked.Events[1].Script != remaining.Events[1].Script ||
				!bytes.Equal(linked.Events[8].Raw, remaining.Events[8].Raw) {
				t.Fatal(
					"location script relocation changed independent references",
					err,
				)
			}
		})
	}
}

func TestLocationRewardVersionWorkflow(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewLocationFixture(fixture.Name)
			s := testutil.Open(t, f)
			d, err := s.Events.NewConditions(0, event.LocationEvents)
			if err != nil {
				t.Fatal(err)
			}
			before := s.Document.Snapshot()
			gold := d.Events()[3].Location.Item
			if fixture.Name == "FE8U" || fixture.Name == "FE8J" {
				for field, value := range map[string]int64{event.ChestItem: 256, event.ChestMoney: 100} {
					if err := d.Set(
						3,
						field,
						value,
					); !errors.Is(
						err,
						edit.ErrValue,
					) {
						t.Fatal("ambiguous FE8 reward accepted", err)
					}
				}
				if err := d.SetChestReward(
					2,
					256,
					0,
				); !errors.Is(err, edit.ErrValue) ||
					d.Modified() {
					t.Fatal("FE8 combined reward partially applied", err)
				}
				if err := d.SetChestReward(3, gold, 256); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := d.SetChestReward(3, gold, 100); err != nil {
					t.Fatal("native low gold rejected", err)
				}
				if err := d.Set(2, event.ChestItem, 0x101); err != nil {
					t.Fatal("native item halfword rejected", err)
				}
			}
			plan, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyConditions(plan); err != nil {
				t.Fatal(err)
			}
			result, err := s.Events.Conditions(0, event.LocationEvents)
			if err != nil {
				t.Fatal(err)
			}
			want := []byte{0x64, 0}
			if fixture.Name == "FE8U" || fixture.Name == "FE8J" {
				want = []byte{0, 1}
			}
			if !bytes.Equal(result.Events[3].Raw[6:8], want) {
				t.Fatal("native reward bytes")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("reward edit was not atomic")
			}
		})
	}
}

func TestLocationConflictAndFailureAtomicity(t *testing.T) {
	for _, scenario := range []string{"foreign document", "revision", "layout", "capacity", "invalid position", "invalid reward", "missing text", "opaque", "shop pointer"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewLocationFixture("FE8U")
			s := testutil.Open(t, f)
			d, err := s.Events.NewConditions(0, event.LocationEvents)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Set(0, event.EventFlag, 0x31); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
			want := error(rom.ErrStale)
			switch scenario {
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
			case "capacity":
				if err := s.Document.ApplyEdit(
					s.Document.State().Revision,
					"capacity",
					rom.Edit{Size: rom.MaxSize},
				); err != nil {
					t.Fatal(err)
				}
				d, err = s.Events.NewConditions(0, event.LocationEvents)
				if err != nil {
					t.Fatal(err)
				}
				if err := d.Set(0, event.EventFlag, 0x31); err != nil {
					t.Fatal(err)
				}
				want = space.ErrCapacity
			}
			before := s.Document.Snapshot()
			raw := d.Events()[2].Raw
			switch scenario {
			case "invalid position":
				err = d.SetLocationPosition(2, image.Pt(4, 256))
				want = edit.ErrValue
			case "invalid reward":
				err = d.SetChestReward(2, 0x77, 255)
				want = edit.ErrValue
			case "missing text":
				if err := d.AddLocation(s.Events, event.Village); err != nil {
					t.Fatal(err)
				}
				_, err = s.Events.PlanConditions(d)
				if err == nil {
					t.Fatal("incomplete visit was writable")
				}
				return
			case "opaque":
				err = d.Set(8, event.LocationX, 4)
				want = edit.ErrField
			case "shop pointer":
				err = d.SetEventScript(
					s.Events,
					6,
					f.GetPosition("location_script"),
				)
				want = edit.ErrField
			case "capacity":
				_, err = s.Events.PlanConditions(d)
			default:
				err = s.Events.ApplyConditions(plan)
			}
			if !errors.Is(err, want) ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				before.Revision != s.Document.State().Revision ||
				!bytes.Equal(raw, d.Events()[2].Raw) {
				t.Fatalf("failure left partial edits: %v", err)
			}
		})
	}
}
