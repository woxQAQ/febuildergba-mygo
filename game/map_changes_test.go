package game

import (
	"bytes"
	"errors"
	"image"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestMapChangesWorkflow(t *testing.T) {
	for _, version := range []string{"FE6", "FE7J", "FE7U", "FE8J", "FE8U"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewMapChangesFixture(version)
			s := openFixture(t, f)
			before := s.Document.Snapshot()
			d, err := s.NewMapChangesDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			if got := d.Entries(); len(got) != 2 || got[0].ID != 3 ||
				got[1].ID != 7 {
				t.Fatal(got)
			}
			base, err := d.Preview(0, false)
			if err != nil {
				t.Fatal(err)
			}
			changed, err := d.Preview(0, true)
			if err != nil {
				t.Fatal(err)
			}
			if cell, _ := changed.Cell(
				1,
				2,
			); cell.Tile != base.Cells[2*base.Width+1].Tile {
				t.Fatal("zero did not retain base")
			}
			if cell, _ := changed.Cell(2, 2); cell.Tile != 4 {
				t.Fatal("nonzero tile not applied")
			}
			if err = d.Stroke(
				0,
				[]image.Point{{X: 1, Y: 2}, {X: 3, Y: 2}},
				4,
			); err != nil {
				t.Fatal(err)
			}
			if !d.Undo() || d.Modified() || !d.Redo() {
				t.Fatal("stroke history")
			}
			if err = d.Copy(0, 11); err != nil {
				t.Fatal(err)
			}
			if err = d.Set(2, 11, image.Rect(0, 0, 4, 3)); err != nil {
				t.Fatal(err)
			}
			if tile, _ := d.Cell(2, 3, 2); tile != 0 {
				t.Fatal("extension did not keep base")
			}
			if err = d.Fill(2, image.Rect(0, 0, 2, 2), 4); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanMapChangesEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if !plan.Applicable() || plan.Before() != 2 || plan.After() != 3 ||
				plan.Payloads() != 2 {
				t.Fatal("wrong plan", plan)
			}
			uses := plan.Uses()
			if len(uses) != 3 || !uses[0].Updated || !uses[1].Updated ||
				uses[2].Updated {
				t.Fatal(uses)
			}
			uses[0].Updated = false
			if !plan.Uses()[0].Updated {
				t.Fatal("mutable uses")
			}
			refs := plan.References()
			direct, position := false, false
			for _, ref := range refs.References {
				if ref.ID == 3 && !ref.ByPosition {
					direct = true
				}
				if ref.ID == 7 && ref.ByPosition && ref.After == 2 {
					position = true
				}
			}
			if !direct || !position {
				t.Fatal("ID/coordinate lookup including last rectangle", refs)
			}
			if len(refs.References) > 0 {
				refs.References[0].After = -1
			}
			if !plan.Applicable() {
				t.Fatal("mutable references")
			}
			if err = d.Delete(2); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("preview wrote ROM")
			}
			if err = s.ApplyMapChangesEdit(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			at := plan.Destination()
			if len(after.Data) != at+plan.Length() {
				t.Fatal("length not atomic")
			}
			if !bytes.Equal(after.Data[at+5:at+8], []byte{0xCD, 0xAB, 0xEF}) ||
				!bytes.Equal(
					after.Data[at+36:at+48],
					before.Data[0x238418:0x238424],
				) {
				t.Fatal("unknown header or terminal bytes changed")
			}
			if u32(after.Data, at+12+8) != u32(before.Data, 0x238414) {
				t.Fatal("unchanged shared payload relocated")
			}
			preserved := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				preserved[plan.Pointer():plan.Pointer()+4],
				before.Data[plan.Pointer():plan.Pointer()+4],
			)
			if !bytes.Equal(preserved, before.Data) {
				t.Fatal("old resources or scripts changed")
			}
			for _, chapter := range []int{0, 1, 2} {
				next, e := s.NewMapChangesDraft(chapter)
				if e != nil {
					t.Fatal(e)
				}
				want := 3
				if chapter == 2 {
					want = 2
				}
				if len(next.Entries()) != want {
					t.Fatal("shared entry", chapter)
				}
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				!s.Document.Redo() ||
				!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("document history")
			}
			if err = s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			next, err := reopened.NewMapChangesDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			if len(next.Entries()) != 3 {
				t.Fatal("mutated preview or lost saved changes")
			}
			if err = next.Stroke(
				0,
				[]image.Point{{X: 1, Y: 2}},
				0,
			); err != nil {
				t.Fatal(err)
			}
			again, err := reopened.PlanMapChangesEdit(next)
			if err != nil {
				t.Fatal(err)
			}
			if err = reopened.ApplyMapChangesEdit(again); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMapChangesGuardsAndReferences(t *testing.T) {
	for _, version := range []string{"FE6", "FE7J", "FE7U", "FE8J", "FE8U"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewMapChangesFixture(version)
			s := openFixture(t, f)
			before := s.Document.Snapshot()
			d, err := s.NewMapChangesDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			for _, operation := range []func() error{
				func() error { return d.Add(128, image.Rect(0, 0, 1, 1)) }, func() error { return d.Add(3, image.Rect(0, 0, 1, 1)) },
				func() error { return d.Set(0, 3, image.Rect(0, 0, 7, 1)) }, func() error { return d.Stroke(0, []image.Point{{X: 1, Y: 2}, {X: 0, Y: 0}}, 4) },
				func() error { return d.Fill(0, image.Rect(1, 2, 2, 3), 0xFFFF) },
			} {
				if !errors.Is(operation(), ErrValue) || d.Modified() ||
					d.CanUndo() {
					t.Fatal("non-atomic invalid draft")
				}
			}
			if err = d.Delete(0); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanMapChangesEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Applicable() ||
				!errors.Is(s.ApplyMapChangesEdit(plan), ErrMapChangeReference) {
				t.Fatal("dangling script reference applied")
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("reference failure wrote ROM")
			}
			if !d.Undo() {
				t.Fatal("delete history")
			}
			if err = d.Set(0, 4, d.Entries()[0].Bounds); err != nil {
				t.Fatal(err)
			}
			plan, err = s.PlanMapChangesEdit(d)
			if err != nil || plan.Applicable() {
				t.Fatal("renaming retained old literal reference", err)
			}
			if !d.Undo() {
				t.Fatal("rename history")
			}
			if err = d.Stroke(0, []image.Point{{X: 1, Y: 2}}, 4); err != nil {
				t.Fatal(err)
			}
			plan, err = s.PlanMapChangesEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Set(Chapters, []int{0}, "weather", 2); err != nil {
				t.Fatal(err)
			}
			stale := s.Document.Snapshot()
			if !errors.Is(s.ApplyMapChangesEdit(plan), rom.ErrStale) ||
				!bytes.Equal(stale.Data, s.Document.Snapshot().Data) {
				t.Fatal("stale plan wrote ROM")
			}
		})
	}
}

func TestMapChangesEmptyAndWriteSites(t *testing.T) {
	for _, version := range []string{"FE6", "FE7J", "FE7U", "FE8J", "FE8U"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewMapChangesFixture(version)
			f.WriteU32(0x221000+9*4, 0)
			s := openFixture(t, f)
			d, err := s.NewMapChangesDraft(0)
			if err != nil || len(d.Entries()) != 0 {
				t.Fatal("empty entry", err)
			}
			if err = d.Add(3, image.Rect(0, 0, 1, 1)); err != nil {
				t.Fatal(err)
			}
			if err = d.Add(7, image.Rect(2, 2, 3, 3)); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanMapChangesEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Uses()) != 2 {
				t.Fatal("unrelated empty entries treated as sharing")
			}
			if err = s.ApplyMapChangesEdit(plan); err != nil {
				t.Fatal(err)
			}
			if next, e := s.NewMapChangesDraft(
				0,
			); e != nil ||
				len(next.Entries()) != 2 {
				t.Fatal("empty create", e)
			}
		})
	}
	t.Run("capacity", func(t *testing.T) {
		f := testutil.NewMapChangesFixture("FE8U")
		s := openFixture(t, f)
		if err := s.Document.ApplyEdit(
			s.Document.State().Revision,
			"grow",
			rom.Edit{Size: rom.MaxSize},
		); err != nil {
			t.Fatal(err)
		}
		d, err := s.NewMapChangesDraft(0)
		if err != nil {
			t.Fatal(err)
		}
		if err = d.Stroke(0, []image.Point{{X: 1, Y: 2}}, 4); err != nil {
			t.Fatal(err)
		}
		if _, err = s.PlanMapChangesEdit(
			d,
		); !errors.Is(
			err,
			space.ErrCapacity,
		) {
			t.Fatal(err)
		}
	})
	t.Run("payload protects plist", func(t *testing.T) {
		f := testutil.NewMapChangesFixture("FE8U")
		// The selected PLIST word lies inside a known change payload.
		f.WritePointer(f.GetPosition("resize_changes_root"), 0x238800-9*4)
		f.WritePointer(0x238800, 0x238400)
		s := openFixture(t, f)
		d, err := s.NewMapChangesDraft(0)
		if err != nil {
			t.Fatal(err)
		}
		if err = d.Fill(0, d.Entries()[0].Bounds, 4); err != nil {
			t.Fatal(err)
		}
		if _, err = s.PlanMapChangesEdit(d); !errors.Is(err, ErrLayout) {
			t.Fatal("payload entry overlap", err)
		}
	})
}
