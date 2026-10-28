//go:build integration

package integration

import (
	"bytes"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
)

func editDeployment(t *testing.T, s *game.Session) {
	table := requireTable(t, s, game.Chapters)
	for _, chapter := range table.Rows {
		resource, err := s.ChapterDeployments(chapter.ID)
		if err != nil {
			continue
		}
		for _, group := range resource.Groups {
			if group.Err != nil || len(group.Units) == 0 {
				continue
			}
			draft, err := s.NewDeploymentDraft(chapter.ID, group.Pointer)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("chapter=%d, deployment=%s", chapter.ID, group.Label)
			before := s.Document.Snapshot()
			expected := draft.Units()
			expected[0].Level ^= 1
			if err := draft.Set(
				0,
				"deploy_level",
				int64(expected[0].Level),
			); err != nil {
				t.Fatal(err)
			}
			edit, err := s.PlanDeploymentEdit(draft)
			checkPlan(t, s, before, err)
			if err := s.ApplyDeploymentEdit(edit); err != nil {
				t.Fatal(err)
			}
			reopened := historyAndReopen(t, s, before)
			result, err := reopened.NewDeploymentDraft(
				chapter.ID,
				group.Pointer,
			)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(result.Units(), expected) {
				t.Fatal("deployment values or order changed after reopening")
			}
			return
		}
	}
	t.Skip("no readable nonempty deployment group")
}

func editEvents(t *testing.T, s *game.Session, kind game.EventTable) {
	table := requireTable(t, s, game.Chapters)
	for _, chapter := range table.Rows {
		draft, err := s.NewEventDraft(chapter.ID, kind)
		if err != nil {
			continue
		}
		for index := range draft.Events() {
			fields := draft.Fields(index)
			position := slices.IndexFunc(
				fields,
				func(field game.Field) bool { return field.ID == game.EventFlag },
			)
			if position < 0 {
				continue
			}
			field := fields[position]
			before := s.Document.Snapshot()
			expected := draft.Events()
			expected[index].Raw[field.Offset] ^= 1
			value, err := draft.Value(index, game.EventFlag)
			if err != nil {
				t.Fatal(err)
			}
			if err := draft.Set(index, game.EventFlag, value^1); err != nil {
				t.Fatal(err)
			}
			t.Logf("chapter=%d, condition=%d", chapter.ID, index)
			edit, err := s.PlanEventEdit(draft)
			checkPlan(t, s, before, err)
			if err := s.ApplyEventEdit(edit); err != nil {
				t.Fatal(err)
			}
			reopened := historyAndReopen(t, s, before)
			result, err := reopened.NewEventDraft(chapter.ID, kind)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.EqualFunc(
				result.Events(),
				expected,
				func(a, b game.EventCondition) bool {
					return bytes.Equal(a.Raw, b.Raw)
				},
			) {
				t.Fatal("event edit changed unrelated condition bytes")
			}
			return
		}
	}
	t.Skip("no readable condition with an editable completion flag")
}
