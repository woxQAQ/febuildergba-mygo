//go:build integration

package integration

import (
	"bytes"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
)

func editShops(t *testing.T, s *game.Session) {
	chapters := requireTable(t, s, records.Chapters)
	for _, chapter := range chapters.Rows {
		draft, err := s.Events.NewConditions(chapter.ID, event.LocationEvents)
		if err != nil {
			continue
		}
		for index, condition := range draft.Events() {
			if condition.OperandKind != event.ShopTarget {
				continue
			}
			inventory, err := draft.Shop(index)
			if err != nil || inventory.Count() == 0 {
				continue
			}
			before := s.Document.Snapshot()
			expected := inventory.Items()
			if len(expected) < shop.MaxItems {
				if err := inventory.Add(int(expected[0].ID)); err != nil {
					t.Fatal(err)
				}
				expected = append(expected, shop.Item{ID: expected[0].ID})
			} else {
				if err := inventory.Remove(len(expected) - 1); err != nil {
					t.Fatal(err)
				}
				expected = expected[:len(expected)-1]
			}
			t.Logf(
				"chapter=%d, location=%d, inventory=0x%08X",
				chapter.ID,
				index,
				inventory.Address(),
			)
			plan, err := s.Events.PlanConditions(draft)
			checkPlan(t, s, before, err)
			if err := s.Events.ApplyConditions(plan); err != nil {
				t.Fatal(err)
			}
			reopened := historyAndReopen(t, s, before)
			result, err := reopened.Events.NewConditions(
				chapter.ID,
				event.LocationEvents,
			)
			if err != nil {
				t.Fatal(err)
			}
			saved, err := result.Shop(index)
			if err != nil || !slices.Equal(saved.Items(), expected) {
				t.Fatal("inventory changed after reopening", err)
			}
			for i, original := range draft.Events() {
				got := result.Events()[i].Raw
				want := bytes.Clone(original.Raw)
				if i == index {
					copy(want[4:8], got[4:8])
				}
				if !bytes.Equal(want, got) {
					t.Fatal("shop edit changed unrelated location bytes")
				}
			}
			return
		}
	}
	t.Skip("no readable nonempty chapter shop inventory")
}
