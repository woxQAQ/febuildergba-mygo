//go:build integration

package integration

import (
	"bytes"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/support"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func editSupports(t *testing.T, session *game.Session) {
	initial := session.Document.Snapshot()
	t.Run("relationships", func(t *testing.T) {
		s := openCopy(t, initial.Data)
		rows, err := s.Supports.Records()
		if err != nil {
			t.Skipf("support characters unavailable: %v", err)
		}
		for _, row := range rows {
			draft, err := s.Supports.NewDraft(row.ID)
			if err != nil {
				continue
			}
			relations := draft.Relationships()
			if len(relations) == 0 {
				continue
			}
			seen := map[uint8]bool{}
			valid := true
			for _, relation := range relations {
				found := false
				for _, other := range rows {
					if other.ID == int(relation.Unit) {
						found = true
						break
					}
				}
				if !found || seen[relation.Unit] ||
					int(relation.Unit) == row.ID {
					valid = false
				}
				seen[relation.Unit] = true
			}
			if !valid {
				continue
			}
			expected := draft.Raw()
			relations[0].Initial ^= 1
			expected[draft.Capacity()] = relations[0].Initial
			if err := draft.Set(0, relations[0]); err != nil {
				t.Fatal(err)
			}
			before := s.Document.Snapshot()
			plan, err := s.Supports.Plan(draft, false)
			checkPlan(t, s, before, err)
			reopened := applySupports(t, s, plan, before)
			saved, err := reopened.Supports.NewDraft(row.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(expected, saved.Raw()) {
				t.Fatal("relationship values or opaque bytes changed")
			}
			t.Logf(
				"character=%02X, resource moves=%d",
				row.ID,
				len(plan.Moves()),
			)
			return
		}
		t.Skip("no editable support relationship")
	})
	t.Run("dialogue", func(t *testing.T) {
		s := openCopy(t, initial.Data)
		draft, err := s.Supports.NewTalkDraft()
		if err != nil {
			t.Skipf("support dialogue unavailable: %v", err)
		}
		original := draft.Talks()
		for _, row := range original {
			for rank, id := range row.Texts {
				if id == 0 || id > 65535 {
					continue
				}
				if err := draft.SetText(row.Index, rank, 0); err != nil {
					t.Fatal(err)
				}
				before := s.Document.Snapshot()
				plan, err := s.Supports.PlanTalks(draft)
				checkPlan(t, s, before, err)
				reopened := applySupports(t, s, plan, before)
				talks, err := reopened.Supports.Talks()
				if err != nil {
					t.Fatal(err)
				}
				if len(talks) != len(original) {
					t.Fatal("dialogue count changed")
				}
				for i, talk := range talks {
					wanted := original[i].Texts
					if i == row.Index {
						wanted[rank] = 0
					}
					if talk.Unit != original[i].Unit ||
						talk.Partner != original[i].Partner ||
						talk.Texts != wanted {
						t.Fatal("dialogue mismatch", i)
					}
					if i != row.Index &&
						!bytes.Equal(talk.Raw, original[i].Raw) {
						t.Fatal("unselected dialogue bytes changed")
					}
				}
				t.Logf(
					"dialogue=%03X, resource moves=%d",
					row.Index,
					len(plan.Moves()),
				)
				return
			}
		}
		t.Skip("no editable support dialogue")
	})
}

func applySupports(
	t *testing.T,
	s *game.Session,
	plan *support.Plan,
	before rom.Snapshot,
) *game.Session {
	t.Helper()
	if err := s.Supports.Apply(plan); err != nil {
		t.Fatal(err)
	}
	after := s.Document.Snapshot()
	prefix := bytes.Clone(before.Data)
	for _, move := range plan.Moves() {
		for _, site := range move.Pointers {
			at, valid := gba.Pointer(after.Data, site)
			if !valid || at != move.After {
				t.Fatal("native consumer not moved")
			}
			copy(prefix[site:site+4], after.Data[site:site+4])
		}
	}
	if !bytes.Equal(prefix, after.Data[:len(prefix)]) {
		t.Fatal("changed old payload or independent reference")
	}
	return historyAndReopen(t, s, before)
}
