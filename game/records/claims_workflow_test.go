package records_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestRecordFieldClaimWorkflow(t *testing.T) {
	for _, sample := range []struct {
		name string
		ids  []int
	}{
		{"foreign-payload", []int{1}},
		{"batch-keeps-safe-first-record", []int{0, 1}},
	} {
		t.Run(sample.name, func(t *testing.T) {
			f := testutil.NewMinimalFE8U()
			// A readable affinity row and its terminator overlap the
			// second item's combat fields without altering its pointers.
			f.WritePointer(0x28534, f.GetPosition("items")+36+20)
			s := testutil.Open(t, f)
			bonus, err := s.Supports.NewBonusDraft()
			if err != nil || len(bonus.Rows()) != 1 {
				t.Fatalf("affinity table precondition: %v", err)
			}
			if table := s.Records.Table(
				records.Items,
			); table.Err != nil ||
				len(table.Rows) != 2 {
				t.Fatal("item table precondition", table.Err)
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			err = s.Records.Set(records.Items, sample.ids, "combat.might", 42)
			if fault.CodeOf(err) != edit.ErrLayout ||
				!errors.Is(err, space.ErrClaim) {
				t.Fatalf(
					"overlapping payload was not rejected with its cause: %v",
					err,
				)
			}
			recordContext, rangeContext := false, false
			for frame := range fault.Frames(err) {
				if frame.Context.RecordID != nil &&
					*frame.Context.RecordID == 1 &&
					frame.Context.Table == string(records.Items) &&
					frame.Context.Field == "combat.might" {
					recordContext = true
				}
				if region := frame.Context.Range; region != nil &&
					region.Offset == f.GetPosition(
						"items",
					)+36+21 && region.Length == 1 {
					rangeContext = true
				}
			}
			if !recordContext || !rangeContext {
				t.Fatal("conflict lost the failing record or field range", err)
			}
			if s.Document.State() != state ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("rejected fields changed bytes or document state")
			}
		})
	}

	t.Run("only-the-written-field-is-checked", func(t *testing.T) {
		f := testutil.NewMinimalFE8U()
		f.WritePointer(0x28534, f.GetPosition("items")+36+20)
		s := testutil.Open(t, f)
		before, state := s.Document.Snapshot(), s.Document.State()
		// Equal values do not write the overlapping field. Duplicate
		// selections still undergo the document's overlap validation.
		if err := s.Records.Set(
			records.Items,
			[]int{1},
			"combat.might",
			6,
		); err != nil {
			t.Fatal(err)
		}
		if err := s.Records.Set(
			records.Items,
			[]int{1, 1},
			"combat.might",
			6,
		); !errors.Is(
			err,
			rom.ErrRange,
		) {
			t.Fatalf("duplicate no-op fields lost overlap validation: %v", err)
		}
		if s.Document.State() != state {
			t.Fatal("no-op fields changed history")
		}
		if err := s.Records.Set(
			records.Items,
			[]int{1},
			"weapon_type",
			3,
		); err != nil {
			t.Fatal("unrelated field in a partially overlapping row", err)
		}
		expected := bytes.Clone(before.Data)
		expected[f.GetPosition("items")+36+7] = 3
		if !bytes.Equal(expected, s.Document.Snapshot().Data) ||
			!s.Document.Undo() || !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
			s.Document.State().CanUndo || s.Document.State().Dirty ||
			!s.Document.Redo() || !bytes.Equal(expected, s.Document.Snapshot().Data) {
			t.Fatal("safe field write lost unknown bytes or atomic history")
		}
	})

	for _, version := range testutil.Games() {
		t.Run(version.Name, func(t *testing.T) {
			for _, overlap := range []bool{true, false} {
				name := "independent-manual-layout"
				if overlap {
					name = "overlapping-record-owner"
				}
				t.Run(name, func(t *testing.T) {
					f := version.Build()
					s := testutil.Open(t, f)
					layout := s.Records.Table(records.Items).Layout
					if overlap {
						layout.Address = f.GetPosition("units")
						layout.Count = 1
					} else {
						data := f.Bytes()
						f.WriteAt(
							0x400000,
							data[layout.Address:layout.Address+layout.Stride*layout.Count],
						)
						s = testutil.Open(t, f)
						layout.Address = 0x400000
					}
					if err := s.Records.UseLayout(
						records.Items,
						layout,
					); err != nil {
						t.Fatal(err)
					}
					before, state := s.Document.Snapshot(), s.Document.State()
					err := s.Records.Set(
						records.Items,
						[]int{0},
						"weapon_type",
						3,
					)
					if overlap {
						if fault.CodeOf(err) != edit.ErrLayout ||
							!errors.Is(err, space.ErrClaim) {
							t.Fatalf(
								"another record owner did not protect its bytes: %v",
								err,
							)
						}
						if s.Document.State() != state ||
							!bytes.Equal(
								before.Data,
								s.Document.Snapshot().Data,
							) {
							t.Fatal(
								"rejected manual field changed the document",
							)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					expected := bytes.Clone(before.Data)
					expected[layout.Address+7] = 3
					if !bytes.Equal(expected, s.Document.Snapshot().Data) ||
						!s.Document.Undo() || !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
						s.Document.State().CanUndo || s.Document.State().Dirty ||
						!s.Document.Redo() || !bytes.Equal(expected, s.Document.Snapshot().Data) {
						t.Fatal(
							"manual field write lost its layout, preserved bytes or atomic history",
						)
					}
				})
			}
		})
	}
}
