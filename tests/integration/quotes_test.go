//go:build integration

package integration

import (
	"bytes"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/quote"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

func editQuotes(t *testing.T, s *game.Session) {
	initial := s.Document.Snapshot()
	tables := s.Quotes.Tables()
	if len(tables) == 0 {
		t.Skip("no native dialogue tables")
	}
	for _, table := range tables {
		t.Run(string(table.Kind), func(t *testing.T) {
			if table.Err != nil {
				t.Skipf("table unavailable: %v", table.Err)
			}
			if len(table.Entries) == 0 {
				t.Skip("empty table has no existing record to duplicate")
			}
			current := openCopy(t, initial.Data)
			draft, err := current.Quotes.NewDraft(table.Kind)
			if err != nil {
				t.Fatal(err)
			}
			expected := make([][]byte, 0, len(table.Entries)+1)
			if len(table.Entries) < quote.MaxEntries {
				if err = draft.Duplicate(0); err != nil {
					t.Fatal(err)
				}
				expected = append(expected, table.Entries[0].Raw)
				for _, entry := range table.Entries {
					expected = append(expected, entry.Raw)
				}
			} else {
				if err = draft.Remove(len(table.Entries) - 1); err != nil {
					t.Fatal(err)
				}
				for _, entry := range table.Entries[:len(table.Entries)-1] {
					expected = append(expected, entry.Raw)
				}
			}
			before := current.Document.Snapshot()
			plan, err := current.Quotes.Plan(draft)
			checkPlan(t, current, before, err)
			t.Logf(
				"records=%d -> %d, source=0x%08X, native-consumers=%d",
				len(table.Entries),
				plan.Count(),
				table.Address,
				len(plan.Pointers()),
			)
			if err = current.Quotes.Apply(plan); err != nil {
				t.Fatal(err)
			}
			after := current.Document.Snapshot()
			prefix := bytes.Clone(before.Data)
			for _, site := range table.Pointers {
				at, valid := gba.Pointer(after.Data, site)
				if !valid || at != plan.After() {
					t.Fatal("native consumer did not move")
				}
				copy(prefix[site:site+4], after.Data[site:site+4])
			}
			if !bytes.Equal(prefix, after.Data[:len(prefix)]) {
				t.Fatal(
					"changed original table, independent references or other bytes",
				)
			}
			reopened := historyAndReopen(t, current, before)
			saved, err := reopened.Quotes.NewDraft(table.Kind)
			if err != nil {
				t.Fatal(err)
			}
			rows := saved.Entries()
			if len(rows) != len(expected) {
				t.Fatal("record count changed after reopening")
			}
			for i, row := range rows {
				if !bytes.Equal(row.Raw, expected[i]) {
					t.Fatal("record bytes or ordering changed", i)
				}
			}
		})
	}
}
