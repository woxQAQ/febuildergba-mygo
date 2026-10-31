package support_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/support"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func TestSupportIndependentChangesAndStalePlans(t *testing.T) {
	f := testutil.NewSupportFixture("FE8U")
	s := testutil.Open(t, f)
	d, err := s.Supports.NewDraft(1)
	if err != nil {
		t.Fatal(err)
	}
	if err = d.Set(
		0,
		support.Relationship{Unit: 2, Initial: 60, Growth: 9},
	); err != nil {
		t.Fatal(err)
	}
	plan, err := s.Supports.Plan(d, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Moves()) != 1 {
		t.Fatal("unrequested synchronization")
	}
	other := testutil.Open(t, f)
	before := other.Document.Snapshot()
	if err = other.Supports.Apply(plan); !errors.Is(err, rom.ErrStale) {
		t.Fatal("cross-document apply", err)
	}
	if !bytes.Equal(before.Data, other.Document.Snapshot().Data) {
		t.Fatal("cross-document write")
	}
	if err = s.Supports.Apply(plan); err != nil {
		t.Fatal(err)
	}
	reverse, err := s.Supports.NewDraft(2)
	if err != nil {
		t.Fatal(err)
	}
	if r := reverse.Relationships()[0]; r.Initial != 20 || r.Growth != 3 {
		t.Fatal("reverse changed without selection")
	}
	d, err = s.Supports.NewDraft(1)
	if err != nil {
		t.Fatal(err)
	}
	if err = d.SetText(0, 1, 2); err != nil {
		t.Fatal(err)
	}
	plan, err = s.Supports.Plan(d, false)
	if err != nil {
		t.Fatal(err)
	}
	s.Refresh()
	if err = s.Records.Set(records.Units, []int{1}, "level", 8); err != nil {
		t.Fatal(err)
	}
	before = s.Document.Snapshot()
	if plan.Current(s.Supports) {
		t.Fatal("stale plan marked current")
	}
	if err = s.Supports.Apply(plan); !errors.Is(err, rom.ErrStale) {
		t.Fatal("revision apply", err)
	}
	if after := s.Document.Snapshot(); after.Revision != before.Revision ||
		!bytes.Equal(before.Data, after.Data) {
		t.Fatal("partial stale apply")
	}
}

func TestSupportSharedReverseRecords(t *testing.T) {
	f := testutil.NewSupportFixture("FE8U")
	f.WritePointer(
		f.GetPosition("support_pointer_3"),
		f.GetPosition("support_2"),
	)
	s := testutil.Open(t, f)
	d, err := s.Supports.NewDraft(1)
	if err != nil {
		t.Fatal(err)
	}
	if err = d.Set(
		0,
		support.Relationship{Unit: 2, Initial: 31, Growth: 5},
	); err != nil {
		t.Fatal(err)
	}
	if err = d.Add(
		support.Relationship{Unit: 3, Initial: 47, Growth: 8},
	); err != nil {
		t.Fatal(err)
	}
	plan, err := s.Supports.Plan(d, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Moves()) != 3 {
		t.Fatal("missing reverse writes")
	}
	for _, move := range plan.Moves() {
		if len(move.Shared) != 0 {
			t.Fatal("changed owner reported as retaining its original data")
		}
	}
	if err = s.Supports.Apply(plan); err != nil {
		t.Fatal(err)
	}
	for id, initial := range map[int]uint8{2: 31, 3: 47} {
		saved, err := s.Supports.NewDraft(id)
		if err != nil {
			t.Fatal(err)
		}
		if got := saved.Relationships()[0]; got.Initial != initial {
			t.Fatal("shared reverse values contaminated", id, got)
		}
	}
}

func TestSupportAmbiguousReverseAndOpaqueText(t *testing.T) {
	t.Run("ambiguous-reverse", func(t *testing.T) {
		f := testutil.NewSupportFixture("FE8U")
		at := f.GetPosition("support_2")
		f.WriteU8(at+1, 1)
		f.WriteU8(at+21, 2)
		s := testutil.Open(t, f)
		before := s.Document.Snapshot()
		d, err := s.Supports.NewDraft(1)
		if err != nil {
			t.Fatal(err)
		}
		if err = d.Set(
			0,
			support.Relationship{Unit: 2, Initial: 70, Growth: 6},
		); err != nil {
			t.Fatal(err)
		}
		if _, err = s.Supports.Plan(
			d,
			true,
		); !errors.Is(
			err,
			support.ErrReferences,
		) {
			t.Fatal("ambiguous reverse", err)
		}
		if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
			t.Fatal("partial reverse write")
		}
	})
	for _, version := range []string{"FE6", "FE7U", "FE7J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewSupportFixture(version)
			f.WriteU32(f.GetPosition("support_talks")+4, 0x12340001)
			s := testutil.Open(t, f)
			d, err := s.Supports.NewDraft(1)
			if err != nil {
				t.Fatal(err)
			}
			if err = d.SetText(0, 1, 2); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Supports.Plan(d, false)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Supports.Apply(plan); err != nil {
				t.Fatal(err)
			}
			rows, err := s.Supports.Talks()
			if err != nil {
				t.Fatal(err)
			}
			if rows[0].Texts != [3]uint32{0x12340001, 2, 1} {
				t.Fatal("unchanged wide value truncated")
			}
		})
	}
}
