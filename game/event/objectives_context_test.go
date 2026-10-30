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

func TestObjectiveChapterContexts(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewObjectiveFixture(fixture.Name)
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
			d, err := s.Events.NewConditions(0, event.AlwaysEvents)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Set(0, event.EventTrigger, 0x39); err != nil {
				t.Fatal(err)
			}
			if err := d.AddObjective(
				s.Events,
				event.RoutVictory,
			); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(plan.Uses(), []int{0}) {
				t.Fatal("independent pointer alias reported as shared")
			}
			if err := s.Events.ApplyConditions(plan); err != nil {
				t.Fatal(err)
			}
			other, err := s.Events.Conditions(1, event.AlwaysEvents)
			if err != nil || other.Events[0].TriggerFlag != 2 ||
				len(other.Events) != 7 {
				t.Fatal("independent header changed")
			}
			move, err := s.Records.PlanTableMove(records.Chapters, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Records.ApplyTableMove(move); err != nil {
				t.Fatal(err)
			}
			moved, err := s.Events.Conditions(0, event.AlwaysEvents)
			if err != nil || moved.Events[0].TriggerFlag != 0x39 ||
				len(moved.Events) != 8 {
				t.Fatal("chapter relocation lost conditions")
			}
			if err := s.Records.UseLayout(
				records.Chapters,
				records.Layout{Address: manual, Stride: table.Stride, Count: 1},
			); err != nil {
				t.Fatal(err)
			}
			actual, err := s.Events.Conditions(0, event.AlwaysEvents)
			if err != nil || actual.Pointer != alias+12 ||
				actual.Events[0].TriggerFlag != 2 {
				t.Fatal("manual chapter layout ignored")
			}
			draft, err := s.Events.NewConditions(0, event.AlwaysEvents)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.ContainsFunc(
				draft.Scripts(),
				func(ref event.EventScriptRef) bool {
					return ref.Kind == event.ChapterEndingScript &&
						ref.Address == f.GetPosition("objective_ending")
				},
			) {
				t.Fatal("manual ending lookup ignored")
			}
		})
	}
}

func TestObjectiveResourceOwnership(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			for _, resource := range []string{"objective_table", "objective_ending", "objective_units", "objective_path", "objective_ending_site"} {
				t.Run(resource, func(t *testing.T) {
					if resource == "objective_path" && fixture.Name != "FE8U" &&
						fixture.Name != "FE8J" {
						return
					}
					f := testutil.NewObjectiveFixture(fixture.Name)
					// Keep the ending as its sole owner: no condition or talk refers to it.
					table := f.GetPosition("objective_table")
					f.WritePointer(table+4, f.GetPosition("objective_defeat"))
					f.WritePointer(table+28, f.GetPosition("objective_defeat"))
					f.WritePointer(table+52, f.GetPosition("objective_defeat"))
					s := testutil.Open(t, f)
					before := s.Document.Snapshot()
					at := f.GetPosition(resource)
					_, err := s.Records.PlanTableMove(
						records.Items,
						[]space.FreeRange{
							{
								Start:  at,
								End:    at + 0x100,
								Source: "live objective resource",
							},
						},
					)
					if !errors.Is(err, space.ErrRange) ||
						!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
						t.Fatalf("resource reused: %v", err)
					}
				})
			}
		})
	}
}
