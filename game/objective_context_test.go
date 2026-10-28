package game

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestObjectiveChapterContexts(t *testing.T) {
	for _, fixture := range gameFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			f := testutil.NewObjectiveFixture(fixture.name)
			const alias, manual = 0x244400, 0x244000
			header := f.GetPosition("deploy_header")
			raw := f.Bytes()
			f.WriteAt(alias, raw[header:header+80])
			f.WritePointer(f.GetPosition("deploy_list")+8, alias)
			f.WritePointer(f.GetPosition("deploy_list")+12, alias)
			s := openFixture(t, f)
			table := s.Table(Chapters)
			f.WriteAt(
				manual,
				raw[table.Rows[0].Address:table.Rows[0].Address+table.Stride],
			)
			offset := 116
			if fixture.name == "FE7U" {
				offset = 120
			}
			if fixture.name == "FE6" {
				offset = 58
			}
			f.WriteU8(manual+offset, 3)
			s = openFixture(t, f)
			d, err := s.NewEventDraft(0, AlwaysEvents)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Set(0, EventTrigger, 0x39); err != nil {
				t.Fatal(err)
			}
			if err := d.AddObjective(s, RoutVictory); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanEventEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(plan.Uses(), []int{0}) {
				t.Fatal("independent pointer alias reported as shared")
			}
			if err := s.ApplyEventEdit(plan); err != nil {
				t.Fatal(err)
			}
			other, err := s.ChapterEvents(1, AlwaysEvents)
			if err != nil || other.Events[0].TriggerFlag != 2 ||
				len(other.Events) != 7 {
				t.Fatal("independent header changed")
			}
			move, err := s.PlanTableMove(Chapters, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyTableMove(move); err != nil {
				t.Fatal(err)
			}
			moved, err := s.ChapterEvents(0, AlwaysEvents)
			if err != nil || moved.Events[0].TriggerFlag != 0x39 ||
				len(moved.Events) != 8 {
				t.Fatal("chapter relocation lost conditions")
			}
			if err := s.UseLayout(
				Chapters,
				Layout{Address: manual, Stride: table.Stride, Count: 1},
			); err != nil {
				t.Fatal(err)
			}
			actual, err := s.ChapterEvents(0, AlwaysEvents)
			if err != nil || actual.Pointer != alias+12 ||
				actual.Events[0].TriggerFlag != 2 {
				t.Fatal("manual chapter layout ignored")
			}
			draft, err := s.NewEventDraft(0, AlwaysEvents)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.ContainsFunc(
				draft.Scripts(),
				func(ref EventScriptRef) bool {
					return ref.Kind == ChapterEndingScript &&
						ref.Address == f.GetPosition("objective_ending")
				},
			) {
				t.Fatal("manual ending lookup ignored")
			}
		})
	}
}

func TestObjectiveResourceOwnership(t *testing.T) {
	for _, fixture := range gameFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			for _, resource := range []string{"objective_table", "objective_ending", "objective_units", "objective_path", "objective_ending_site"} {
				t.Run(resource, func(t *testing.T) {
					if resource == "objective_path" && fixture.name != "FE8U" &&
						fixture.name != "FE8J" {
						return
					}
					f := testutil.NewObjectiveFixture(fixture.name)
					// Keep the ending as its sole owner: no condition or talk refers to it.
					table := f.GetPosition("objective_table")
					f.WritePointer(table+4, f.GetPosition("objective_defeat"))
					f.WritePointer(table+28, f.GetPosition("objective_defeat"))
					f.WritePointer(table+52, f.GetPosition("objective_defeat"))
					s := openFixture(t, f)
					before := s.Document.Snapshot()
					at := f.GetPosition(resource)
					_, err := s.PlanTableMove(
						Items,
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
