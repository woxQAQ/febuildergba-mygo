package records_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestTableMoves(t *testing.T) {
	for _, fixture := range testutil.Games() {
		for _, kind := range records.Kinds {
			t.Run(fixture.Name+"/"+string(kind), func(t *testing.T) {
				f := fixture.Build()
				// This pointer-shaped value is not a rule-confirmed reference.
				f.WritePointer(0x400, f.GetPosition(string(kind)+"_base"))
				s := testutil.Open(t, f)
				before := s.Document.Snapshot()
				table := *s.Records.Table(kind)
				move, err := s.Records.PlanTableMove(kind, nil)
				if err != nil {
					t.Fatal(err)
				}
				if s.Document.State().Revision != before.Revision ||
					s.Document.State().Dirty {
					t.Fatal("preview changed document")
				}
				if move.Destination() != len(before.Data) ||
					move.ReferenceCount() < 1 {
					t.Fatalf(
						"unexpected destination %X or reference count %d",
						move.Destination(),
						move.ReferenceCount(),
					)
				}
				wantLength := table.Count * table.Stride
				if kind != records.Units {
					wantLength += table.Stride // The stopping record must survive relocation.
				} else if fixture.Name == "FE6" {
					wantLength += table.Stride // Includes the hidden ID 0.
				}
				if move.Length() != wantLength {
					t.Fatalf(
						"copied length %d, want %d",
						move.Length(),
						wantLength,
					)
				}
				if err := s.Records.ApplyTableMove(move); err != nil {
					t.Fatal(err)
				}
				after := s.Document.Snapshot()
				if gba.Uint32(
					after.Data,
					table.Pointer,
				) != uint32(
					0x08000000+move.Destination(),
				) {
					t.Fatal("table pointer not updated")
				}
				if !bytes.Equal(
					before.Data[0x400:0x404],
					after.Data[0x400:0x404],
				) ||
					!bytes.Equal(
						before.Data[move.Source():move.Source()+move.Length()],
						after.Data[move.Source():move.Source()+move.Length()],
					) ||
					!bytes.Equal(
						before.Data[move.Source():move.Source()+move.Length()],
						after.Data[move.Destination():move.Destination()+move.Length()],
					) {
					t.Fatal(
						"source or unselected pointer changed, or copied bytes differ",
					)
				}
				if !s.Document.Undo() || s.Document.State().CanUndo ||
					!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
					s.Document.State().Dirty {
					t.Fatal(
						"one undo did not restore complete document and length",
					)
				}
				s.Refresh()
				if s.Records.Table(kind).Address != table.Address ||
					!s.Document.Redo() {
					t.Fatal("undo interpretation or redo failed")
				}
				s.Refresh()
				if !bytes.Equal(after.Data, s.Document.Snapshot().Data) ||
					s.Records.Table(kind).Count != table.Count {
					t.Fatal("redo did not restore moved table")
				}
				if err := s.Document.Save(); err != nil {
					t.Fatal(err)
				}
				reopened, err := game.Open(s.Document.State().Path)
				if err != nil {
					t.Fatal(err)
				}
				if reopened.Records.Table(
					kind,
				).Address != s.Records.Table(
					kind,
				).Address ||
					reopened.Records.Table(kind).Count != table.Count {
					t.Fatal("saved table cannot be automatically located")
				}
				field := "base.hp"
				switch kind {
				case records.Items:
					field = "combat.might"
				case records.Chapters:
					field = "fog_level"
				}
				if err := reopened.Records.Set(
					kind,
					[]int{table.FirstID},
					field,
					23,
				); err != nil {
					t.Fatal(err)
				}
				if got, err := reopened.Records.Value(
					kind,
					table.FirstID,
					field,
				); err != nil ||
					got != 23 {
					t.Fatalf("moved record edit: %d, %v", got, err)
				}
			})
		}
	}
}

func TestTableMoveRejectsStaleOrForeignPreview(t *testing.T) {
	s := testutil.Open(t, testutil.NewMinimalFE8U())
	move, err := s.Records.PlanTableMove(records.Items, nil)
	if err != nil {
		t.Fatal(err)
	}
	other := testutil.Open(t, testutil.NewMinimalFE8U())
	if err := other.Records.ApplyTableMove(
		move,
	); !errors.Is(err, rom.ErrStale) ||
		other.Document.State().Dirty {
		t.Fatalf("foreign preview: %v", err)
	}
	if err := s.Records.Set(
		records.Items,
		[]int{0},
		"combat.might",
		12,
	); err != nil {
		t.Fatal(err)
	}
	stable := s.Document.Snapshot()
	if err := s.Records.ApplyTableMove(
		move,
	); !errors.Is(err, rom.ErrStale) ||
		!bytes.Equal(stable.Data, s.Document.Snapshot().Data) {
		t.Fatalf("stale preview changed document: %v", err)
	}
	move, err = s.Records.PlanTableMove(records.Items, nil)
	if err != nil {
		t.Fatal(err)
	}
	table := s.Records.Table(records.Items)
	if err := s.Records.UseLayout(records.Items, table.Layout); err != nil {
		t.Fatal(err)
	}
	if err := s.Records.ApplyTableMove(
		move,
	); !errors.Is(
		err,
		edit.ErrLayout,
	) {
		t.Fatalf("manual interpretation accepted old preview: %v", err)
	}
	if _, err := s.Records.PlanTableMove(
		records.Items,
		nil,
	); !errors.Is(
		err,
		edit.ErrLayout,
	) {
		t.Fatalf("manual layout produced automatic move: %v", err)
	}
}

func TestTableMoveUsesDeclaredSpace(t *testing.T) {
	s := testutil.Open(t, testutil.NewMinimalFE8U())
	available := space.Range{Start: 0x400000, End: 0x401000}
	free := []space.FreeRange{
		{Range: available, Source: "explicitly reserved test interval"},
	}
	move, err := s.Records.PlanTableMove(records.Items, free)
	if err != nil {
		t.Fatal(err)
	}
	originalSize := s.Document.State().Size
	if move.Destination() != free[0].Start || move.Size() != originalSize {
		t.Fatal("declared range was not used")
	}
	if err := s.Records.ApplyTableMove(move); err != nil {
		t.Fatal(err)
	}
	if s.Records.Table(records.Items).Address != free[0].Start ||
		s.Document.State().Size != originalSize {
		t.Fatal("relocation did not use declared range")
	}
	free[0].Start, free[0].End = s.Records.Table(
		records.Units,
	).Address, s.Records.Table(
		records.Units,
	).Address+52
	if _, err := s.Records.PlanTableMove(
		records.Items,
		free,
	); !errors.Is(
		err,
		space.ErrRange,
	) {
		t.Fatalf("overlapping known table: %v", err)
	}
}
