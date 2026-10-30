package chapter_test

import (
	"bytes"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestMapResizeLocationImpacts(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewLocationFixture(fixture.Name)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			draft, err := s.Maps.NewDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			if err := draft.Resize(1, 1, maps.MapTopLeft, 0); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Maps.Plan(draft)
			if err != nil {
				t.Fatal(err)
			}
			seen := make(map[int]bool)
			for _, impact := range plan.Impacts() {
				if impact.Kind != maps.TriggerLocation {
					continue
				}
				if impact.Chapter != 0 || impact.BeforeOutside ||
					!impact.AfterOutside ||
					impact.LostCells != 1 ||
					impact.Reference != f.GetPosition("deploy_header")+8 {
					t.Fatalf("location crop impact: %+v", impact)
				}
				seen[impact.Index] = true
			}
			for _, index := range []int{0, 1, 2, 3, 4, 5, 6, 7, 9} {
				if !seen[index] {
					t.Fatalf("location %d missing from crop preview", index)
				}
			}
			if len(seen) != 9 {
				t.Fatal("unknown coordinates were interpreted")
			}
			warned := false
			for _, warning := range plan.ImpactWarnings() {
				if warning.Kind == maps.TriggerLocation &&
					warning.Chapter == 0 {
					warned = true
				}
			}
			if !warned {
				t.Fatal("unknown location context was not reported")
			}
			if err := s.Maps.Apply(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			at, end := f.GetPosition(
				"location_table",
			), f.GetPosition(
				"location_terminator",
			)+4
			if !bytes.Equal(before.Data[at:end], after.Data[at:end]) {
				t.Fatal("map resize moved location coordinates")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("map resize did not undo atomically")
			}
		})
	}
}
