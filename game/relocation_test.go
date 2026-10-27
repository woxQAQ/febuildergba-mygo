package game

import (
	"bytes"
	"errors"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestTableMoves(t *testing.T) {
	for _, fixture := range gameFixtures() {
		for _, kind := range Kinds {
			t.Run(fixture.name+"/"+string(kind), func(t *testing.T) {
				f := fixture.build()
				// This pointer-shaped value is not a rule-confirmed reference.
				f.WritePointer(0x400, f.GetPosition(string(kind)+"_base"))
				s := openFixture(t, f)
				before := s.Document.Snapshot()
				table := *s.Table(kind)
				move, err := s.PlanTableMove(kind, nil)
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
				if kind != Units {
					wantLength += table.Stride // The stopping record must survive relocation.
				} else if fixture.name == "FE6" {
					wantLength += table.Stride // Includes the hidden ID 0.
				}
				if move.Length() != wantLength {
					t.Fatalf(
						"copied length %d, want %d",
						move.Length(),
						wantLength,
					)
				}
				if err := s.ApplyTableMove(move); err != nil {
					t.Fatal(err)
				}
				after := s.Document.Snapshot()
				if u32(
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
				if s.Table(kind).Address != table.Address ||
					!s.Document.Redo() {
					t.Fatal("undo interpretation or redo failed")
				}
				s.Refresh()
				if !bytes.Equal(after.Data, s.Document.Snapshot().Data) ||
					s.Table(kind).Count != table.Count {
					t.Fatal("redo did not restore moved table")
				}
				if err := s.Document.Save(); err != nil {
					t.Fatal(err)
				}
				reopened, err := Open(s.Document.State().Path)
				if err != nil {
					t.Fatal(err)
				}
				if reopened.Table(kind).Address != s.Table(kind).Address ||
					reopened.Table(kind).Count != table.Count {
					t.Fatal("saved table cannot be automatically located")
				}
				field := "base.hp"
				switch kind {
				case Items:
					field = "combat.might"
				case Chapters:
					field = "fog_level"
				}
				if err := reopened.Set(
					kind,
					[]int{table.FirstID},
					field,
					23,
				); err != nil {
					t.Fatal(err)
				}
				if got, err := reopened.Value(
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
	s := openFixture(t, testutil.NewMinimalFE8U())
	move, err := s.PlanTableMove(Items, nil)
	if err != nil {
		t.Fatal(err)
	}
	other := openFixture(t, testutil.NewMinimalFE8U())
	if err := other.ApplyTableMove(
		move,
	); !errors.Is(err, rom.ErrStale) ||
		other.Document.State().Dirty {
		t.Fatalf("foreign preview: %v", err)
	}
	if err := s.Set(Items, []int{0}, "combat.might", 12); err != nil {
		t.Fatal(err)
	}
	stable := s.Document.Snapshot()
	if err := s.ApplyTableMove(
		move,
	); !errors.Is(err, rom.ErrStale) ||
		!bytes.Equal(stable.Data, s.Document.Snapshot().Data) {
		t.Fatalf("stale preview changed document: %v", err)
	}
	move, err = s.PlanTableMove(Items, nil)
	if err != nil {
		t.Fatal(err)
	}
	table := s.Table(Items)
	if err := s.UseLayout(Items, table.Layout); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyTableMove(move); !errors.Is(err, ErrLayout) {
		t.Fatalf("manual interpretation accepted old preview: %v", err)
	}
	if _, err := s.PlanTableMove(Items, nil); !errors.Is(err, ErrLayout) {
		t.Fatalf("manual layout produced automatic move: %v", err)
	}
}

func TestTableMoveUsesDeclaredSpace(t *testing.T) {
	s := openFixture(t, testutil.NewMinimalFE8U())
	available := space.Range{Start: 0x400000, End: 0x401000}
	free := []space.FreeRange{
		{Range: available, Source: "explicitly reserved test interval"},
	}
	move, err := s.PlanTableMove(Items, free)
	if err != nil {
		t.Fatal(err)
	}
	originalSize := s.Document.State().Size
	if move.Destination() != free[0].Start || move.Size() != originalSize {
		t.Fatal("declared range was not used")
	}
	if err := s.ApplyTableMove(move); err != nil {
		t.Fatal(err)
	}
	if s.Table(Items).Address != free[0].Start ||
		s.Document.State().Size != originalSize {
		t.Fatal("relocation did not use declared range")
	}
	free[0].Start, free[0].End = s.Table(
		Units,
	).Address, s.Table(
		Units,
	).Address+52
	if _, err := s.PlanTableMove(Items, free); !errors.Is(err, space.ErrRange) {
		t.Fatalf("overlapping known table: %v", err)
	}
}
