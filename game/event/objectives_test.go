package event_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func TestObjectiveWorkflow(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewObjectiveFixture(fixture.Name)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			native, err := s.Events.Conditions(0, event.AlwaysEvents)
			if err != nil {
				t.Fatal(err)
			}
			if len(native.Events) != 7 || native.Events[0].TriggerFlag != 2 ||
				native.Events[6].Script != 1 ||
				len(native.Events[6].Warnings) != 0 {
				t.Fatalf("native flag conditions: %+v", native.Events)
			}
			d, err := s.Events.NewConditions(0, event.AlwaysEvents)
			if err != nil {
				t.Fatal(err)
			}
			width := 2
			if fixture.Name == "FE8U" || fixture.Name == "FE8J" {
				width = 4
			}
			if len(d.Fields(0)) != 2 || d.Fields(0)[1].Width != width ||
				len(d.Fields(4)) != 0 ||
				len(d.Fields(5)) != 0 {
				t.Fatal("confirmed fields or opaque conditions")
			}
			if width == 2 && (!native.Events[2].Rout || len(d.Fields(2)) != 1) {
				t.Fatal("native rout callback")
			}
			for _, change := range []struct {
				index int
				field string
				value int64
			}{
				{0, event.EventTrigger, 0x34}, {0, event.EventFlag, 0x35}, {1, event.AreaX1, -2},
			} {
				if err := d.Set(
					change.index,
					change.field,
					change.value,
				); err != nil {
					t.Fatal(err)
				}
			}
			if err := d.SetEventScript(
				s.Events,
				0,
				f.GetPosition("objective_defeat"),
			); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(d.Events()[0].Texts, []uint16{2}) ||
				len(d.Events()[0].Groups) != 0 {
				t.Fatal("draft script navigation remained stale")
			}
			for _, template := range []event.ObjectiveTemplate{event.BossVictory, event.RoutVictory, event.FlagFailure} {
				if err := d.AddObjective(s.Events, template); err != nil {
					t.Fatal(err)
				}
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("draft mutated document")
			}
			plan, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(plan.Uses(), []int{0, 1}) || plan.Before() != 7 ||
				plan.After() != 10 ||
				plan.Length() != 124 {
				t.Fatal("shared complete-list plan")
			}
			if err := d.Set(0, event.EventTrigger, 0x55); err != nil {
				t.Fatal(err)
			}
			if err := d.SetEventScript(s.Events, 0, 1); err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyConditions(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			result, err := s.Events.Conditions(0, event.AlwaysEvents)
			if err != nil {
				t.Fatal(err)
			}
			expected := bytes.Clone(native.Events[0].Raw)
			expected[2], expected[8] = 0x35, 0x34
			binary.LittleEndian.PutUint32(
				expected[4:],
				uint32(0x08000000+f.GetPosition("objective_defeat")),
			)
			if !bytes.Equal(result.Events[0].Raw, expected) {
				t.Fatalf(
					"native width, ignored bytes or immutable plan: % X",
					result.Events[0].Raw,
				)
			}
			for _, index := range []int{2, 3, 4, 5, 6} {
				if !bytes.Equal(
					native.Events[index].Raw,
					result.Events[index].Raw,
				) {
					t.Fatalf("unrelated condition changed: %d", index)
				}
			}
			if result.Events[1].X1 != -2 {
				t.Fatal("shared area edit omitted")
			}
			args := []uint32{2, 6, 0x65}
			kinds := []byte{1, 1, 1}
			if width == 2 {
				kinds[1], args[1] = 14, 0x08079FE9
				if fixture.Name == "FE7J" {
					args[1] = 0x0807AA5D
				}
				if fixture.Name == "FE6" {
					kinds[1], args[1] = 13, 0x0806C2F9
				}
			}
			for i := range 3 {
				e := result.Events[7+i]
				want := []byte{kinds[i], 0, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0}
				script := f.GetPosition("objective_ending")
				if i == 2 {
					want[2], script = 0, f.GetPosition("objective_defeat")
				}
				binary.LittleEndian.PutUint32(
					want[4:],
					uint32(0x08000000+script),
				)
				binary.LittleEndian.PutUint32(want[8:], args[i])
				if !bytes.Equal(e.Raw, want) {
					t.Fatalf("fixed native template %d: % X", i, e.Raw)
				}
			}
			prefix := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				prefix[native.Pointer:native.Pointer+4],
				before.Data[native.Pointer:native.Pointer+4],
			)
			if !bytes.Equal(prefix, before.Data) {
				t.Fatal("existing scripts or resources changed")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(s.Document.Snapshot().Data, before.Data) {
				t.Fatal("atomic undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(s.Document.Snapshot().Data, after.Data) {
				t.Fatal("atomic redo")
			}
			output := filepath.Join(t.TempDir(), "objectives.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			edit, err := reopened.Events.NewConditions(
				0,
				event.AlwaysEvents,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := edit.SetEventScript(reopened.Events, 0, 1); err != nil {
				t.Fatal(err)
			}
			if edit.Events()[0].Script != 1 ||
				len(edit.Events()[0].Texts) != 0 ||
				len(edit.Events()[0].Warnings) != 0 {
				t.Fatal("flag-only navigation")
			}
			if err := edit.Remove(9); err != nil {
				t.Fatal(err)
			}
			next, err := reopened.Events.PlanConditions(edit)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Events.ApplyConditions(next); err != nil {
				t.Fatal(err)
			}
			final, err := reopened.Events.Conditions(0, event.AlwaysEvents)
			if err != nil || len(final.Events) != 9 ||
				binary.LittleEndian.Uint32(final.Events[0].Raw[4:]) != 1 {
				t.Fatal("saved conditions cannot be continued")
			}
		})
	}
}

func TestObjectiveDraftGuards(t *testing.T) {
	for _, name := range []string{"missing ending", "missing failure", "foreign document", "stale draft", "unknown script", "unknown callback", "invalid trigger", "wrong table"} {
		t.Run(name, func(t *testing.T) {
			f := testutil.NewObjectiveFixture("FE8U")
			if name == "missing ending" {
				f.WriteU32(f.GetPosition("objective_ending_site"), 0)
			}
			if name == "missing failure" {
				f.WriteU32(f.GetPosition("objective_table")+40, 0)
				f.WriteAt(f.GetPosition("objective_defeat"), make([]byte, 32))
			}
			s := testutil.Open(t, f)
			table := event.AlwaysEvents
			if name == "wrong table" {
				table = event.TalkEvents
			}
			d, err := s.Events.NewConditions(0, table)
			if err != nil {
				t.Fatal(err)
			}
			if name == "foreign document" {
				s = testutil.Open(t, f)
			}
			if name == "stale draft" {
				if err := s.Document.Apply(
					s.Document.State().Revision,
					"external edit",
					[]rom.Change{{Offset: 0x180, Data: []byte{3}}},
				); err != nil {
					t.Fatal(err)
				}
			}
			before := s.Document.Snapshot()
			switch name {
			case "missing ending":
				err = d.AddObjective(s.Events, event.BossVictory)
			case "missing failure":
				err = d.AddObjective(s.Events, event.FlagFailure)
			case "unknown script":
				err = d.SetEventScript(s.Events, 0, 0x180)
			case "unknown callback":
				err = d.SetEventScript(s.Events, 4, 1)
			case "invalid trigger":
				err = d.Set(0, event.EventTrigger, 0x100000000)
			default:
				err = d.AddObjective(s.Events, event.BossVictory)
			}
			if err == nil ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("invalid operation accepted or changed ROM")
			}
			if (name == "foreign document" || name == "stale draft") &&
				!errors.Is(err, rom.ErrStale) {
				t.Fatal("lost stale identity")
			}
		})
	}
}
