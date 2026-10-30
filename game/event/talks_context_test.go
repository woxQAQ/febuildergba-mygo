package event_test

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestTalkChapterContexts(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewTalkFixture(fixture.Name)
			const alias, manual = 0x244400, 0x244000
			header := f.GetPosition("deploy_header")
			raw := f.Bytes()
			f.WriteAt(alias, raw[header:header+80])
			f.WritePointer(f.GetPosition("deploy_list")+8, alias)
			f.WritePointer(f.GetPosition("deploy_list")+12, alias)
			s := testutil.Open(t, f)
			table := s.Records.Table(records.Chapters)
			f.WriteAt(
				manual,
				raw[table.Rows[0].Address:table.Rows[0].Address+table.Stride],
			)
			offset := 116
			if fixture.Name == "FE7U" {
				offset = 120
			}
			if fixture.Name == "FE6" {
				offset = 58
			}
			f.WriteU8(manual+offset, 3)
			s = testutil.Open(t, f)
			d, err := s.Events.NewConditions(0, event.TalkEvents)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Set(1, event.TalkTarget, 7); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(plan.Uses(), []int{0}) {
				t.Fatal("independent talk pointer included in shared preview")
			}
			if err := s.Events.ApplyConditions(plan); err != nil {
				t.Fatal(err)
			}
			old, err := s.Events.Conditions(1, event.TalkEvents)
			if err != nil || old.Address != f.GetPosition("talk_table") ||
				old.Events[1].Target != 2 {
				t.Fatal("independent header alias changed")
			}
			move, err := s.Records.PlanTableMove(records.Chapters, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Records.ApplyTableMove(move); err != nil {
				t.Fatal(err)
			}
			moved, err := s.Events.Conditions(0, event.TalkEvents)
			if err != nil || moved.Events[1].Target != 7 {
				t.Fatal("chapter relocation lost talk edits")
			}
			if err := s.Records.UseLayout(
				records.Chapters,
				records.Layout{Address: manual, Stride: table.Stride, Count: 1},
			); err != nil {
				t.Fatal(err)
			}
			actual, err := s.Events.Conditions(0, event.TalkEvents)
			if err != nil || actual.Pointer != alias+4 ||
				actual.Events[1].Target != 2 {
				t.Fatal("manual chapter layout ignored")
			}
		})
	}
}

func TestTalkRecoveryAndOwnership(t *testing.T) {
	for _, name := range []string{"empty pointer", "last word terminator", "malformed terminator", "unterminated", "unresolved script", "talk_table", "talk_script", "talk_units", "talk_path"} {
		t.Run(name, func(t *testing.T) {
			f := testutil.NewTalkFixture("FE8U")
			site, at := f.GetPosition(
				"deploy_header",
			)+4, f.GetPosition(
				"talk_table",
			)
			switch name {
			case "empty pointer":
				f.WriteU32(site, 0)
			case "last word terminator":
				f.WritePointer(site, len(f.Bytes())-4)
			case "malformed terminator":
				f.WriteU32(f.GetPosition("talk_terminator"), 0x12340000)
			case "unterminated":
				for i := range 257 {
					f.WriteAt(
						at+i*16,
						[]byte{3, 0, 0, 0, 0, 0, 0, 0, 1, 2, 0, 0, 0, 0, 0, 0},
					)
				}
			case "unresolved script":
				f.WriteU32(at+20, 0xDEADBEEF)
			}
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			d, err := s.Events.NewConditions(0, event.TalkEvents)
			if name == "malformed terminator" || name == "unterminated" {
				if !errors.Is(err, event.ErrEventConditions) ||
					!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
					t.Fatalf("damaged talk list accepted: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if name == "talk_table" || name == "talk_script" ||
				name == "talk_units" ||
				name == "talk_path" {
				start := f.GetPosition(name)
				if _, err := s.Records.PlanTableMove(
					records.Items,
					[]space.FreeRange{
						{
							Start:  start,
							End:    start + 0x100,
							Source: "talk resource conflict",
						},
					},
				); !errors.Is(err, space.ErrRange) ||
					!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
					t.Fatalf("live talk resource reused: %v", err)
				}
				return
			}
			if name == "unresolved script" && len(d.Events()[1].Warnings) == 0 {
				t.Fatal("unresolved talk script not reported")
			}
			if err := d.Add(s.Events); err != nil {
				t.Fatal(err)
			}
			added := len(d.Events()) - 1
			if err := d.SetDialogue(added, 1); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyConditions(plan); err != nil {
				t.Fatal(err)
			}
			result, err := s.Events.Conditions(0, event.TalkEvents)
			if err != nil || len(result.Events) != added+1 {
				t.Fatal("new talk could not be applied")
			}
			if name == "unresolved script" &&
				!bytes.Equal(
					result.Events[1].Raw[4:8],
					[]byte{0xEF, 0xBE, 0xAD, 0xDE},
				) {
				t.Fatal("unresolved script pointer changed")
			}
		})
	}
}

func TestTalkTextDiscovery(t *testing.T) {
	for _, name := range []string{"known slot and call", "unknown slot", "slot changed by ASM", "route text alternatives"} {
		t.Run(name, func(t *testing.T) {
			version := "FE8U"
			if name == "route text alternatives" {
				version = "FE7U"
			}
			f := testutil.NewTalkFixture(version)
			script := f.GetPosition("talk_script")
			const child = 0x247800
			switch name {
			case "known slot and call", "unknown slot", "slot changed by ASM":
				f.WriteAt(
					script,
					[]byte{
						0x40,
						0x05,
						2,
						0,
						2,
						0,
						0,
						0,
						0x40,
						0x0A,
						0,
						0,
						0,
						0,
						0,
						0,
						0x20,
						1,
						0,
						0,
					},
				)
				f.WritePointer(script+12, child)
				f.WriteAt(
					child,
					[]byte{
						0x20,
						0x1A,
						0,
						0,
						0x20,
						0x1B,
						0xFF,
						0xFF,
						0x20,
						0x1D,
						0,
						0,
						0x22,
						0x1B,
						0,
						0,
						0x20,
						1,
						0,
						0,
					},
				)
				if name == "slot changed by ASM" {
					f.WriteAt(
						script,
						[]byte{
							0x40,
							0x05,
							2,
							0,
							2,
							0,
							0,
							0,
							0x40,
							0x0D,
							0,
							0,
							1,
							1,
							0,
							8,
							0x40,
							0x0A,
							0,
							0,
							0,
							0,
							0,
							0,
							0x20,
							1,
							0,
							0,
						},
					)
					f.WritePointer(script+20, child)
				}
				if name == "unknown slot" {
					f.WriteU16(script+2, 3)
				}
			case "route text alternatives":
				f.WriteAt(
					script,
					[]byte{
						0x0F,
						0,
						0,
						0,
						1,
						0,
						0,
						0,
						2,
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
					},
				)
			}
			s := testutil.Open(t, f)
			events, err := s.Events.Conditions(0, event.TalkEvents)
			if err != nil {
				t.Fatal(err)
			}
			event := events.Events[1]
			want := []uint16{2}
			if name == "route text alternatives" {
				want = []uint16{1, 2}
			}
			if name == "unknown slot" || name == "slot changed by ASM" {
				want = nil
				if len(event.Warnings) == 0 {
					t.Fatal("dynamic text presented as a confirmed reference")
				}
			}
			if !slices.Equal(event.Texts, want) {
				t.Fatalf("typed text operands: %X", event.Texts)
			}
		})
	}
}
