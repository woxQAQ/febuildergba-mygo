package event_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func TestTalkEditingWorkflow(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewTalkFixture(fixture.Name)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			native, err := s.Events.Conditions(0, event.TalkEvents)
			if err != nil {
				t.Fatal(err)
			}
			stride, kind := 16, 3
			if fixture.Name == "FE6" {
				stride, kind = 12, 4
			}
			condition := native.Events[1]
			if native.Pointer != f.GetPosition("deploy_header")+4 ||
				len(native.Events) != 3 ||
				len(condition.Raw) != stride ||
				condition.Kind != kind ||
				condition.Initiator != 1 ||
				condition.Target != 2 ||
				!slices.Equal(condition.Texts, []uint16{1}) ||
				len(condition.Groups) != 1 ||
				len(condition.Warnings) != 0 {
				t.Fatalf("fixed talk condition: %+v", native)
			}
			if stride == 16 &&
				(condition.TalkMode != 3 || condition.TalkFlag != 0x2A) {
				t.Fatalf("native additional condition: %+v", condition)
			}
			d, err := s.Events.NewConditions(0, event.TalkEvents)
			if err != nil {
				t.Fatal(err)
			}
			if d.Modified() || len(d.Fields(0)) != 0 || len(d.Fields(2)) != 0 {
				t.Fatal("opaque and ASM conditions became editable")
			}
			for field, value := range map[string]int64{event.TalkInitiator: 2, event.TalkTarget: 1, event.EventFlag: 0x61} {
				if err := d.Set(1, field, value); err != nil {
					t.Fatal(err)
				}
			}
			if stride == 16 {
				if err := d.Set(1, event.TalkMode, 0); err != nil {
					t.Fatal(err)
				}
				if err := d.Set(1, event.TalkFlag, 0x5E); err != nil {
					t.Fatal(err)
				}
			}
			if err := d.Add(s.Events); err != nil {
				t.Fatal(err)
			}
			added := len(d.Events()) - 1
			if d.Deployment(added) != nil {
				t.Fatal("ordinary talk created a reinforcement")
			}
			if err := d.SetDialogue(added, 2); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(added, event.EventFlag, 0x62); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("draft wrote ROM bytes")
			}
			plan, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Before() != 3 || plan.After() != 4 ||
				!slices.Equal(plan.Uses(), []int{0, 1}) {
				t.Fatal("shared talk preview")
			}
			if err := d.SetDialogue(added, 1); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(1, event.TalkTarget, 9); err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyConditions(plan); err != nil {
				t.Fatal(err)
			}
			result, err := s.Events.Conditions(0, event.TalkEvents)
			if err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			want := bytes.Clone(condition.Raw)
			want[2], want[3], want[8], want[9] = 0x61, 0, 2, 1
			if stride == 16 {
				want[12], want[14], want[15] = 0, 0x5E, 0
				if fixture.Name == "FE8U" || fixture.Name == "FE8J" {
					want[13] = 0
				}
			}
			if !bytes.Equal(result.Events[1].Raw, want) ||
				result.Events[1].TalkMode != 0 {
				t.Fatalf(
					"typed edits changed ignored bytes: %X",
					result.Events[1].Raw,
				)
			}
			for _, i := range []int{0, 2} {
				if !bytes.Equal(native.Events[i].Raw, result.Events[i].Raw) {
					t.Fatal("unrelated condition changed")
				}
			}
			created := result.Events[added]
			if created.Kind != kind || created.Flag != 0x62 ||
				!slices.Equal(created.Texts, []uint16{2}) ||
				len(created.Groups) != 0 ||
				len(created.Warnings) != 0 {
				t.Fatalf("new ordinary dialog: %+v", created)
			}
			literal := "0D00000002000000090000000A00000000000000"
			if fixture.Name == "FE8U" || fixture.Name == "FE8J" {
				literal = "201A0000201B0200201D0000221B000020010000"
			}
			if fixture.Name == "FE6" {
				literal = "0700000002000000050000000600000000000000"
			}
			script, err := hex.DecodeString(literal)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(
				after.Data[created.Script:created.Script+len(script)],
				script,
			) {
				t.Fatal(
					"generated script differs from independent native literal",
				)
			}
			prefix := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				prefix[native.Pointer:native.Pointer+4],
				before.Data[native.Pointer:native.Pointer+4],
			)
			if !bytes.Equal(prefix, before.Data) {
				t.Fatal(
					"existing scripts, other condition tables or text bytes changed",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(s.Document.Snapshot().Data, before.Data) ||
				!s.Document.Redo() ||
				!bytes.Equal(s.Document.Snapshot().Data, after.Data) {
				t.Fatal("talk edit was not one atomic history operation")
			}
			output := filepath.Join(t.TempDir(), "talks.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			again, err := reopened.Events.Conditions(0, event.TalkEvents)
			if err != nil ||
				!bytes.Equal(again.Events[added].Raw, created.Raw) ||
				!slices.Equal(again.Events[added].Texts, []uint16{2}) {
				t.Fatal("talk did not survive save/reopen")
			}
			textEdit, err := reopened.Texts.PlanID(1, "AA")
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Texts.Apply(textEdit); err != nil {
				t.Fatal(err)
			}
			text, err := reopened.Texts.Read(again.Events[1].Texts[0])
			if err != nil || text.Content != "AA" {
				t.Fatal("existing script did not retain its shared text ID")
			}
			units, err := reopened.Deployments.NewDraft(
				0,
				again.Events[1].Groups[0].Pointer,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := units.Set(0, "deploy_level", 7); err != nil {
				t.Fatal(err)
			}
			unitEdit, err := reopened.Deployments.Plan(units)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Deployments.Apply(unitEdit); err != nil {
				t.Fatal(err)
			}
			removal, err := reopened.Events.NewConditions(
				0,
				event.TalkEvents,
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
				event.TalkEvents,
			)
			if err != nil || len(remaining.Events) != 3 ||
				remaining.Events[1].Groups[0].Units[0].Level != 7 {
				t.Fatal("reopened talk could not continue editing")
			}
		})
	}
}

func TestTalkDraftGuards(t *testing.T) {
	for _, name := range []string{"foreign document", "revision", "layout", "no text", "RAM text", "missing text", "zero target", "zero FE6 initiator", "opaque condition", "invalid values", "recovered text table"} {
		t.Run(name, func(t *testing.T) {
			version := "FE8U"
			if name == "zero FE6 initiator" {
				version = "FE6"
			}
			f := testutil.NewTalkFixture(version)
			if name == "recovered text table" {
				f.WriteU32(f.GetPosition("talk_text_root"), 0)
				raw := f.Bytes()
				f.WriteAt(
					0x15D48C,
					raw[f.GetPosition("talk_text_table"):f.GetPosition("talk_text_table")+20],
				)
			}
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			d, err := s.Events.NewConditions(0, event.TalkEvents)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Add(s.Events); err != nil {
				t.Fatal(err)
			}
			if err := d.SetDialogue(3, 1); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "no text":
				err = d.SetDialogue(3, 0)
			case "RAM text":
				err = d.SetDialogue(3, 3)
			case "missing text":
				err = d.SetDialogue(3, 4)
			case "zero target":
				err = d.Set(3, event.TalkTarget, 0)
			case "zero FE6 initiator":
				err = d.Set(3, event.TalkInitiator, 0)
			case "opaque condition", "invalid values":
				index, field, value := 0, event.TalkTarget, int64(1)
				if name == "invalid values" {
					index, value = 1, 256
				}
				original := d.Events()[index].Raw
				if d.Set(index, field, value) == nil ||
					!bytes.Equal(original, d.Events()[index].Raw) ||
					d.SetDialogue(1, 2) == nil ||
					d.SetDialogue(3, 0x7FFF) == nil {
					t.Fatal("rejected values changed the draft")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if name == "foreign document" || name == "revision" ||
				name == "layout" {
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
				}
				current := s.Document.Snapshot()
				if !errors.Is(s.Events.ApplyConditions(plan), rom.ErrStale) ||
					!errors.Is(
						func() error { _, err := s.Events.PlanConditions(d); return err }(),
						rom.ErrStale,
					) ||
					!bytes.Equal(current.Data, s.Document.Snapshot().Data) {
					t.Fatal("stale talk edit changed document")
				}
				return
			}
			if _, err := s.Events.PlanConditions(
				d,
			); err == nil ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("invalid new dialog wrote document")
			}
		})
	}
}
