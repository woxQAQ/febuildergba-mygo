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
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

func editNativeShops(t *testing.T, s *game.Session) {
	initial := s.Document.Snapshot()
	for _, family := range []struct {
		name   string
		origin shop.Origin
	}{
		{"default", shop.Default}, {"preparation", shop.Preparation}, {"world-map", shop.WorldMap},
	} {
		t.Run(family.name, func(t *testing.T) {
			current := openCopy(t, initial.Data)
			entries, warnings := current.Shops.Entries()
			for _, entry := range entries {
				if entry.Key.Origin != family.origin || entry.Err != nil {
					continue
				}
				draft, err := current.Shops.NewDraft(entry.Key)
				if err != nil {
					t.Fatal(err)
				}
				inventory := draft.Inventory()
				want := inventory.Items()
				if len(want) == shop.MaxItems {
					if err := inventory.Remove(len(want) - 1); err != nil {
						t.Fatal(err)
					}
					want = want[:len(want)-1]
				} else {
					id := byte(1)
					if len(want) != 0 {
						id = want[0].ID
					}
					if err := inventory.Add(int(id)); err != nil {
						t.Fatal(err)
					}
					want = append(want, shop.Item{ID: id})
				}
				before := current.Document.Snapshot()
				plan, err := current.Shops.Plan(draft)
				checkPlan(t, current, before, err)
				t.Logf(
					"entry=%+v, pointer=0x%08X, inventory=0x%08X, uses-default=%t, known-uses=%d",
					entry.Key,
					entry.Pointer,
					entry.Address,
					entry.UsesDefault,
					len(plan.Uses()),
				)
				if err := current.Shops.Apply(plan); err != nil {
					t.Fatal(err)
				}
				after := current.Document.Snapshot()
				expected := bytes.Clone(before.Data)
				copy(
					expected[entry.Pointer:entry.Pointer+4],
					after.Data[entry.Pointer:entry.Pointer+4],
				)
				if !bytes.Equal(expected, after.Data[:len(expected)]) {
					t.Fatal(
						"changed unrelated node bytes, old inventory or independent references",
					)
				}
				reopened := historyAndReopen(t, current, before)
				saved, err := reopened.Shops.NewDraft(entry.Key)
				if err != nil ||
					!slices.Equal(saved.Inventory().Items(), want) {
					t.Fatal(
						"native shop inventory changed after reopening",
						err,
					)
				}
				if family.origin == shop.WorldMap {
					saved.Clear()
					closing, err := reopened.Shops.Plan(saved)
					if err != nil {
						t.Fatal(err)
					}
					if err := reopened.Shops.Apply(closing); err != nil {
						t.Fatal(err)
					}
					image := reopened.Document.Snapshot()
					address, valid := gba.Pointer(
						image.Data,
						saved.Entry().Pointer,
					)
					if !valid || gba.Uint16(image.Data, address) != 0 {
						t.Fatal(
							"closing did not install a valid empty inventory",
						)
					}
				}
				return
			}
			t.Skipf(
				"no readable native entry for this family (%d unresolved sources)",
				len(warnings),
			)
		})
	}
}

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
