package event_test

import (
	"image"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestReinforcementPathPlan(t *testing.T) {
	for _, version := range []string{"FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			s := testutil.Open(t, testutil.NewAreaFixture(version))
			d, err := s.Events.NewConditions(0, event.TurnEvents)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Add(s.Events); err != nil {
				t.Fatal(err)
			}
			index := len(d.Events()) - 1
			units := d.Deployment(index)
			for range 2 {
				if err := units.AddPathStep(0); err != nil {
					t.Fatal(err)
				}
			}
			for field, value := range map[string]int64{deployment.MoveX: 4, deployment.MoveY: 5, deployment.MoveSyncUnit: 2, deployment.MoveDelay: 30} {
				if err := units.SetPathField(0, 1, field, value); err != nil {
					t.Fatal(err)
				}
			}
			generation := units.Generation()
			plan, err := s.Events.PlanConditions(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := units.SetPathField(
				0,
				1,
				deployment.MoveX,
				7,
			); err != nil {
				t.Fatal(err)
			}
			if generation == units.Generation() {
				t.Fatal("path edit did not change deployment generation")
			}
			if err := s.Events.ApplyConditions(plan); err != nil {
				t.Fatal(err)
			}
			events, err := s.Events.Conditions(0, event.TurnEvents)
			if err != nil {
				t.Fatal(err)
			}
			group := events.Events[index].Groups[0]
			if group.Units[0].End != image.Pt(4, 5) ||
				group.Units[0].MovementCount != 2 {
				t.Fatal("event plan path was mutable")
			}
			edit, err := s.Deployments.NewDraft(0, group.Pointer)
			if err != nil {
				t.Fatal(err)
			}
			if step := edit.Path(0)[1]; step.SyncUnit != 2 || step.Delay != 30 {
				t.Fatal("reinforcement REDA fields changed")
			}
		})
	}
}
