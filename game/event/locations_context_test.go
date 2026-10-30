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

func TestLocationChapterContexts(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewLocationFixture(fixture.Name)
			const alias, manual = 0x253000, 0x253400
			header := f.GetPosition("deploy_header")
			raw := f.Bytes()
			f.WriteAt(alias, raw[header:header+80])
			f.WritePointer(f.GetPosition("deploy_list")+8, alias)
			s := testutil.Open(t, f)
			table := s.Records.Table(records.Chapters)
			f.WriteAt(
				manual,
				raw[table.Rows[1].Address:table.Rows[1].Address+table.Stride],
			)
			s = testutil.Open(t, f)
			d, err := s.Events.NewConditions(0, event.LocationEvents)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Set(0, event.LocationX, 7); err != nil {
				t.Fatal(err)
			}
			if err := d.AddLocation(s.Events, event.House); err != nil {
				t.Fatal(err)
			}
			if err := d.SetEventScript(
				s.Events,
				11,
				f.GetPosition("objective_ending"),
			); err != nil {
				t.Fatal(err)
			}
			if _, owned := d.Dialog(11); owned {
				t.Fatal(
					"existing reference retained generated dialog ownership",
				)
			}
			plan, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(plan.Uses(), []int{0}) {
				t.Fatal("independent header was reported as shared")
			}
			if err := s.Events.ApplyConditions(plan); err != nil {
				t.Fatal(err)
			}
			original, err := s.Events.Conditions(1, event.LocationEvents)
			if err != nil ||
				original.Address != f.GetPosition("location_table") ||
				original.Events[0].Location.Position.X != 1 {
				t.Fatal("independent location header changed", err)
			}
			move, err := s.Records.PlanTableMove(records.Chapters, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Records.ApplyTableMove(move); err != nil {
				t.Fatal(err)
			}
			moved, err := s.Events.Conditions(0, event.LocationEvents)
			if err != nil || moved.Events[0].Location.Position.X != 7 ||
				moved.Events[11].Script != f.GetPosition("objective_ending") {
				t.Fatal("chapter relocation lost locations", err)
			}
			if err := s.Records.UseLayout(
				records.Chapters,
				records.Layout{Address: manual, Stride: table.Stride, Count: 1},
			); err != nil {
				t.Fatal(err)
			}
			manualTable, err := s.Events.Conditions(0, event.LocationEvents)
			if err != nil || manualTable.Pointer != alias+8 ||
				manualTable.Events[0].Location.Position.X != 1 {
				t.Fatal("manual chapter layout ignored", err)
			}
		})
	}
}

func TestLocationResourceBoundaries(t *testing.T) {
	for _, scenario := range []string{"empty", "short terminator", "malformed terminator", "unterminated", "truncated", "location_table", "location_script", "location_inventory", "location_random", "invalid script", "missing ending", "reward word resembles pointer"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewLocationFixture("FE8U")
			site, table := f.GetPosition(
				"deploy_header",
			)+8, f.GetPosition(
				"location_table",
			)
			switch scenario {
			case "empty":
				f.WriteU32(site, 0)
			case "short terminator":
				f.WritePointer(site, len(f.Bytes())-4)
			case "malformed terminator":
				f.WriteU32(f.GetPosition("location_terminator"), 0x12340000)
			case "unterminated":
				for i := range 257 {
					f.WriteAt(
						table+i*12,
						[]byte{7, 0, 1, 0, 2, 0, 0, 0, 1, 0, 0x14, 0},
					)
				}
			case "truncated":
				f.WritePointer(site, len(f.Bytes())-8)
				f.WriteU32(len(f.Bytes())-8, 7)
			case "invalid script":
				f.WriteU32(table+4, 0xDEADBEEF)
			case "missing ending":
				f.WriteU32(f.GetPosition("objective_ending_site"), 0)
			case "reward word resembles pointer":
				f.WritePointer(table+2*12+4, f.GetPosition("location_script"))
			}
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			d, err := s.Events.NewConditions(0, event.LocationEvents)
			if scenario == "malformed terminator" ||
				scenario == "unterminated" ||
				scenario == "truncated" {
				if !errors.Is(err, event.ErrEventConditions) ||
					!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
					t.Fatalf("damaged location table accepted: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "location_table",
				"location_script",
				"location_inventory",
				"location_random":
				start := f.GetPosition(scenario)
				_, err := s.Records.PlanTableMove(
					records.Items,
					[]space.FreeRange{
						{
							Start:  start,
							End:    start + 0x100,
							Source: "location resource conflict",
						},
					},
				)
				if !errors.Is(err, space.ErrRange) ||
					!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
					t.Fatalf("live location resource reused: %v", err)
				}
				return
			case "missing ending":
				if err := d.AddLocation(
					s.Events,
					event.Seize,
				); !errors.Is(err, event.ErrEventConditions) ||
					len(d.Events()) != 11 {
					t.Fatal(
						"missing ending created a partial seize trigger",
						err,
					)
				}
				return
			case "reward word resembles pointer":
				if d.Events()[2].Script != 0 ||
					d.Events()[2].OperandKind != event.RewardTarget {
					t.Fatal("reward promoted to script")
				}
				if _, err := s.Events.NewScriptDraft(
					0,
					table+2*12+4,
				); err == nil {
					t.Fatal("reward accepted by script editor")
				}
			case "invalid script":
				if len(d.Events()[0].Warnings) == 0 {
					t.Fatal("invalid script was not reported")
				}
			}
			if err := d.AddLocation(s.Events, event.Door); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyConditions(plan); err != nil {
				t.Fatal(err)
			}
			result, err := s.Events.Conditions(0, event.LocationEvents)
			if err != nil || len(result.Events) != len(d.Events()) {
				t.Fatal("recovered location table", err)
			}
			if scenario == "invalid script" &&
				result.Events[0].Operand != 0xDEADBEEF {
				t.Fatal("unresolved reference changed")
			}
		})
	}
}
