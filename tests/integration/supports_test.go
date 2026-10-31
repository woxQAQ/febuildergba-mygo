//go:build integration

package integration

import (
	"bytes"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/support"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

func editSupports(t *testing.T, session *game.Session) {
	initial := session.Document.Snapshot()
	rows, err := session.Supports.Records()
	if err != nil {
		t.Skipf("support characters unavailable: %v", err)
	}
	for _, workflow := range []string{"relationships", "dialogue"} {
		t.Run(workflow, func(t *testing.T) {
			s := openCopy(t, initial.Data)
			var draft *support.Draft
			var expected []byte
			var talkIndex, rank int
			for _, row := range rows {
				candidate, err := s.Supports.NewDraft(row.ID)
				if err != nil {
					continue
				}
				if workflow == "relationships" {
					relations := candidate.Relationships()
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
					expected = candidate.Raw()
					relations[0].Initial ^= 1
					expected[candidate.Capacity()] = relations[0].Initial
					if err = candidate.Set(0, relations[0]); err != nil {
						t.Fatal(err)
					}
					draft = candidate
					break
				}
				if candidate.TalkError() != nil {
					continue
				}
				for _, talk := range candidate.Talks() {
					for r, id := range talk.Texts {
						if id == 0 || id > 65535 {
							continue
						}
						if err = candidate.SetText(
							talk.Index,
							r,
							0,
						); err != nil {
							t.Fatal(err)
						}
						talkIndex, rank = talk.Index, r
						draft = candidate
						break
					}
					if draft != nil {
						break
					}
				}
				if draft != nil {
					break
				}
			}
			if draft == nil {
				t.Skip("no editable support entry for this workflow")
			}
			before := s.Document.Snapshot()
			plan, err := s.Supports.Plan(draft, false)
			checkPlan(t, s, before, err)
			if err = s.Supports.Apply(plan); err != nil {
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
			reopened := historyAndReopen(t, s, before)
			if workflow == "relationships" {
				saved, err := reopened.Supports.NewDraft(draft.Unit().ID)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(expected, saved.Raw()) {
					t.Fatal("relationship values or opaque bytes changed")
				}
			} else {
				oldTalks, err := session.Supports.Talks()
				if err != nil {
					t.Fatal(err)
				}
				talks, err := reopened.Supports.Talks()
				if err != nil {
					t.Fatal(err)
				}
				if len(talks) != len(oldTalks) {
					t.Fatal("dialogue count changed")
				}
				for i, talk := range talks {
					wanted := oldTalks[i].Texts
					if i == talkIndex {
						wanted[rank] = 0
					}
					if talk.Unit != oldTalks[i].Unit ||
						talk.Partner != oldTalks[i].Partner ||
						talk.Texts != wanted {
						t.Fatal("dialogue mismatch", i)
					}
					if i != talkIndex &&
						!bytes.Equal(talk.Raw, oldTalks[i].Raw) {
						t.Fatal("unselected dialogue bytes changed")
					}
				}
			}
			t.Logf(
				"character=%02X, resource moves=%d",
				draft.Unit().ID,
				len(plan.Moves()),
			)
		})
	}
}
