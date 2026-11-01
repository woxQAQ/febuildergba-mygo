//go:build integration

package integration

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/arena"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

func editArena(t *testing.T, session *game.Session) {
	initial := session.Document.Snapshot()
	for _, group := range session.Arena.Groups() {
		t.Run(fmt.Sprint(group), func(t *testing.T) {
			s := openCopy(t, initial.Data)
			d, err := s.Arena.NewDraft(group)
			if err != nil {
				t.Skipf("arena group unavailable: %v", err)
			}
			expected := make(map[arena.Kind][]byte)
			switch {
			case group.ClassList():
				rows := d.Rows(group)
				if len(rows) == 0 || len(rows) >= arena.MaxClasses {
					t.Skip("no room to duplicate a class")
				}
				old := d.Raw(group)
				expected[group] = append([]byte{byte(rows[0].Value)}, old...)
				err = d.Insert(group, 0, int(rows[0].Value))
			case group == arena.LinkUnits:
				rows := d.Rows(group)
				index := slices.IndexFunc(
					rows,
					func(row arena.Row) bool { return row.Value != 0 },
				)
				if index < 0 {
					t.Skip("all link slots already clear")
				}
				expected[group] = d.Raw(group)
				expected[group][index*2], expected[group][index*2+1] = 0, 0
				err = d.Set(group, index, 0)
			case group == arena.BaseWeapons:
				items := requireTable(t, s, records.Items)
				base := d.Raw(arena.BaseWeapons)
				rows := d.Rows(arena.Upgrades)
				// Change a base weapon and its first lookup entry together.
				// The candidate is discovered from the current upgrade table.
				index := slices.IndexFunc(
					rows,
					func(row arena.Row) bool {
						return row.Editable &&
							!bytes.Contains(base, []byte{byte(row.Value)}) &&
							slices.ContainsFunc(items.Rows, func(item records.Row) bool { return item.ID == int(row.Value) })
					},
				)
				if index < 0 {
					t.Skip("no independently editable upgrade slot")
				}
				original := d.Raw(arena.Upgrades)
				match := bytes.IndexByte(original, base[0])
				if match < 0 || match >= len(rows) || !rows[match].Editable {
					t.Skip("no native base weapon lookup")
				}
				replacement := byte(rows[index].Value)
				expected[arena.BaseWeapons] = bytes.Clone(base)
				expected[arena.BaseWeapons][0] = replacement
				expected[arena.Upgrades] = original
				expected[arena.Upgrades][match] = replacement
				if err = d.Set(
					arena.BaseWeapons,
					0,
					int(replacement),
				); err != nil {
					t.Fatal(err)
				}
				err = d.Set(arena.Upgrades, match, int(replacement))
			}
			if err != nil {
				t.Fatal(err)
			}
			before := s.Document.Snapshot()
			plan, err := s.Arena.Plan(d)
			// A readable custom pool can still lack native tiers or weapon matches.
			// Such a group is not an editable candidate for this native workflow.
			if errors.Is(err, arena.ErrClasses) ||
				errors.Is(err, arena.ErrWeapons) ||
				errors.Is(err, arena.ErrReference) ||
				errors.Is(err, edit.ErrLayout) {
				if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
					t.Fatal("rejected plan wrote ROM")
				}
				t.Skipf("native arena write unavailable: %v", err)
			}
			checkPlan(t, s, before, err)
			if err = s.Arena.Apply(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			prefix := bytes.Clone(before.Data)
			for _, change := range plan.Changes() {
				for _, site := range change.Pointers {
					at, valid := gba.Pointer(after.Data, site)
					if !valid || at != change.Destination {
						t.Fatal("consumer not updated")
					}
					copy(prefix[site:site+4], after.Data[site:site+4])
				}
				if !bytes.Equal(
					after.Data[change.Destination:change.Destination+len(change.AfterBytes)],
					expected[change.Kind],
				) {
					t.Fatal("payload mismatch")
				}
			}
			if !bytes.Equal(prefix, after.Data[:len(prefix)]) {
				t.Fatal("source or independent data changed")
			}
			reopened := historyAndReopen(t, s, before)
			saved, err := reopened.Arena.NewDraft(group)
			if err != nil {
				t.Fatal(err)
			}
			for kind, wanted := range expected {
				if !bytes.Equal(saved.Raw(kind), wanted) {
					t.Fatal("save/reopen", kind)
				}
			}
			t.Logf("group=%d, moved resources=%d", group, len(plan.Changes()))
		})
	}
}
