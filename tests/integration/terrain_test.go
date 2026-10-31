//go:build integration

package integration

import (
	"bytes"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/terrain"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

func editTerrain(t *testing.T, session *game.Session) {
	initial := session.Document.Snapshot()
	classes := requireTable(t, session, records.Classes)
	for _, column := range session.Terrain.Format().Columns() {
		t.Run(string(column.Kind), func(t *testing.T) {
			s := openCopy(t, initial.Data)
			var draft *terrain.Draft
			var chosen terrain.Table
			for _, row := range classes.Rows {
				candidate, err := s.Terrain.NewDraft(row.ID)
				if err != nil {
					continue
				}
				for _, table := range candidate.Tables() {
					if table.Kind == column.Kind && table.Err == nil &&
						!table.Empty &&
						len(table.Values) > 0 {
						draft, chosen = candidate, table
						break
					}
				}
				if draft != nil {
					break
				}
			}
			if draft == nil {
				t.Skip("no readable native class terrain table")
			}
			expected := bytes.Clone(
				initial.Data[chosen.Address : chosen.Address+len(chosen.Values)],
			)
			value := chosen.Values[0] ^ 1
			expected[0] ^= 1
			if err := draft.Set(column.Kind, 0, value); err != nil {
				t.Fatal(err)
			}
			before := s.Document.Snapshot()
			plan, err := s.Terrain.Plan(draft)
			checkPlan(t, s, before, err)
			if err := s.Terrain.Apply(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			prefix := bytes.Clone(before.Data)
			copy(
				prefix[chosen.Pointer:chosen.Pointer+4],
				after.Data[chosen.Pointer:chosen.Pointer+4],
			)
			if !bytes.Equal(prefix, after.Data[:len(prefix)]) {
				t.Fatal(
					"unselected pointers, old table or unknown bytes changed",
				)
			}
			address, valid := gba.Pointer(after.Data, chosen.Pointer)
			if !valid ||
				!bytes.Equal(
					expected,
					after.Data[address:address+len(expected)],
				) {
				t.Fatal("new terrain payload differs")
			}
			reopened := historyAndReopen(t, s, before)
			actual, err := reopened.Terrain.NewDraft(draft.Class().ID)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, table := range actual.Tables() {
				if table.Kind == column.Kind {
					found = true
					if table.Err != nil || table.Empty ||
						table.Values[0] != value ||
						table.Address != address {
						t.Fatal("saved terrain value or reference differs")
					}
				}
			}
			if !found {
				t.Fatal("saved column missing")
			}
			t.Logf(
				"class=%02X, column=%s, known independent entries=%d",
				draft.Class().ID,
				column.Kind,
				len(plan.Moves()[0].Shared),
			)
		})
	}
}
