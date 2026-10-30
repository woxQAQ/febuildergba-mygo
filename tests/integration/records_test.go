//go:build integration

package integration

import (
	"bytes"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
)

func editRecords(t *testing.T, s *game.Session) {
	source := s.Document.Snapshot().Data
	for _, kind := range records.Kinds {
		t.Run(string(kind), func(t *testing.T) {
			s := openCopy(t, source)
			table := requireTable(t, s, kind)
			var selected *records.Field
			for _, field := range table.Fields {
				if field.ID != "id" && field.Text == text.NoRole &&
					field.Reference == "" && field.Asset == "" && !field.Hex &&
					field.Group != records.GroupAdvanced {
					selected = &field
					break
				}
			}
			if selected == nil {
				t.Skip("no scalar field in this table")
			}
			field, row := *selected, table.Rows[0]
			value, err := s.Records.Value(kind, row.ID, field.ID)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("record=%d, field=%s", row.ID, field.ID)
			before := s.Document.Snapshot()
			if err := s.Records.Set(
				kind,
				[]int{row.ID},
				field.ID,
				value^1,
			); err != nil {
				t.Fatal(err)
			}
			// Flipping the low bit changes exactly one bit at the reported field
			// position, irrespective of signedness or the input's original value.
			expected := bytes.Clone(before.Data)
			expected[row.Address+field.Offset] ^= 1
			if !bytes.Equal(expected, s.Document.Snapshot().Data) {
				t.Fatal("scalar edit changed unrelated bytes")
			}
			reopened := historyAndReopen(t, s, before)
			if got, err := reopened.Records.Value(
				kind,
				row.ID,
				field.ID,
			); err != nil ||
				got != value^1 {
				t.Fatalf("edited value after reopening: %d, %v", got, err)
			}
			if err := reopened.Records.Set(
				kind,
				[]int{row.ID},
				field.ID,
				value,
			); err != nil {
				t.Fatal(err)
			}
			saveAndReopen(t, reopened, before.Data)
		})
	}
}

func relocateRecords(t *testing.T, s *game.Session) {
	source := s.Document.Snapshot().Data
	for _, kind := range records.Kinds {
		t.Run(string(kind), func(t *testing.T) {
			s := openCopy(t, source)
			table := requireTable(t, s, kind)
			original := *table
			before := s.Document.Snapshot()
			move, err := s.Records.PlanTableMove(kind, nil)
			checkPlan(t, s, before, err)
			if err := s.Records.ApplyTableMove(move); err != nil {
				t.Fatal(err)
			}
			reopened := historyAndReopen(t, s, before)
			moved := requireTable(t, reopened, kind)
			if moved.Address != move.Destination()+original.Address-move.Source() ||
				moved.Count != original.Count ||
				len(moved.Rows) != len(original.Rows) {
				t.Fatal("table location or record count was not preserved")
			}
			for index, row := range original.Rows {
				got := moved.Rows[index]
				if got.ID != row.ID || got.NameID != row.NameID ||
					got.Name != row.Name {
					t.Fatal("relocation changed record identity or text")
				}
			}
		})
	}
}
