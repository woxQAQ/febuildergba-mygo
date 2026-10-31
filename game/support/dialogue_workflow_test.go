package support_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game/support"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func TestIndependentSupportPanels(t *testing.T) {
	for _, version := range []string{"FE6", "FE7J", "FE7U", "FE8J", "FE8U"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewSupportFixture(version)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			relation, err := s.Supports.NewDraft(1)
			if err != nil {
				t.Fatal(err)
			}
			if err := relation.Set(
				0,
				support.Relationship{Unit: 2, Initial: 60, Growth: 9},
			); err != nil {
				t.Fatal(err)
			}
			relationshipPlan, err := s.Supports.Plan(relation, false)
			if err != nil {
				t.Fatal(err)
			}
			dialog, err := s.Supports.NewTalkDraft()
			if err != nil {
				t.Fatal(err)
			}
			if err := dialog.SetText(1, 2, 2); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Supports.PlanTalks(dialog)
			if err != nil {
				t.Fatal(err)
			}
			if moves := plan.Moves(); len(moves) != 1 || moves[0].Unit != 0 {
				t.Fatal("dialog includes relationship writes")
			}
			// Subsequent input must not change the frozen dialog plan.
			if err := dialog.SetText(0, 0, 0); err != nil {
				t.Fatal(err)
			}
			if err := s.Supports.Apply(plan); err != nil {
				t.Fatal(err)
			}
			talks, err := s.Supports.Talks()
			if err != nil {
				t.Fatal(err)
			}
			if talks[0].Texts[0] != 1 || talks[1].Texts[2] != 2 {
				t.Fatal("dialog selection or frozen plan changed")
			}
			actual, err := s.Supports.NewDraft(1)
			if err != nil {
				t.Fatal(err)
			}
			if actual.Relationships()[0].Initial != 20 {
				t.Fatal("unapplied relationship draft was committed")
			}
			after := s.Document.Snapshot()
			if relation.Current(s.Supports) || dialog.Current(s.Supports) {
				t.Fatal("other panel retained a current draft")
			}
			if err := s.Supports.Apply(
				relationshipPlan,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatal("stale panel applied", err)
			}
			if !bytes.Equal(after.Data, s.Document.Snapshot().Data) ||
				after.Revision != s.Document.State().Revision {
				t.Fatal("stale panel left a partial edit")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("dialog undo did not restore document")
			}
			if !s.Document.Redo() {
				t.Fatal("dialog redo unavailable")
			}

			// A damaged relationship must not prevent editing the independent table.
			countOffset := 21
			if version == "FE6" {
				countOffset = 30
			}
			f.WriteU8(f.GetPosition("support_1")+countOffset, 255)
			f.WriteU8(f.GetPosition("support_2")+countOffset, 255)
			damaged := testutil.Open(t, f)
			if _, err := damaged.Supports.NewDraft(
				1,
			); !errors.Is(
				err,
				support.ErrData,
			) {
				t.Fatal("fixture relationship not damaged", err)
			}
			d, err := damaged.Supports.NewTalkDraft()
			if err != nil {
				t.Fatal(err)
			}
			if err := d.SetText(0, 0, 2); err != nil {
				t.Fatal(err)
			}
			p, err := damaged.Supports.PlanTalks(d)
			if err != nil {
				t.Fatal(err)
			}
			original := damaged.Document.Snapshot()
			if err := damaged.Supports.Apply(p); err != nil {
				t.Fatal(err)
			}
			saved := damaged.Document.Snapshot()
			for _, name := range []string{"support_1", "support_2"} {
				at := f.GetPosition(name)
				if !bytes.Equal(
					original.Data[at:at+countOffset+2],
					saved.Data[at:at+countOffset+2],
				) {
					t.Fatal("damaged relationship normalized")
				}
			}
			other := testutil.Open(t, f)
			if _, err := other.Supports.PlanTalks(
				d,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatal("cross-document dialog accepted", err)
			}
		})
	}
}
