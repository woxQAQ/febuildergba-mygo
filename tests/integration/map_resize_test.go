//go:build integration

package integration

import (
	"bytes"
	"image"
	"testing"
	"time"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
)

func resizeMap(t *testing.T, s *game.Session) {
	t.Helper()
	for _, row := range requireTable(t, s, records.Chapters).Rows {
		draft, err := s.Maps.NewDraft(row.ID)
		if err != nil {
			continue
		}
		old := image.Pt(draft.Width(), draft.Height())
		size := old
		switch {
		case old.X > 1:
			size.X--
		case old.Y > 1:
			size.Y--
		default:
			size.X++
		}
		fill, _ := draft.Cell(0, 0)
		before := s.Document.Snapshot()
		if err = draft.Resize(
			size.X,
			size.Y,
			maps.MapBottomRight,
			fill.Tile,
		); err != nil {
			t.Fatal(err)
		}
		if !draft.Undo() || draft.Modified() || !draft.Redo() {
			t.Fatal("resize draft history")
		}
		started := time.Now()
		plan, err := s.Maps.Plan(draft)
		checkPlan(t, s, before, err)
		if !plan.Resized() || plan.BeforeSize() != old ||
			plan.AfterSize() != size {
			t.Fatal("resize summary")
		}
		t.Logf(
			"chapter=%02X, size=%v→%v, impacts=%d, incomplete=%d, planning=%s",
			row.ID,
			old,
			size,
			len(plan.Impacts()),
			len(plan.ImpactWarnings()),
			time.Since(started),
		)
		if err = s.Maps.Apply(plan); err != nil {
			t.Fatal(err)
		}
		after := s.Document.Snapshot()
		preserved := bytes.Clone(after.Data[:len(before.Data)])
		copy(
			preserved[plan.Pointer():plan.Pointer()+4],
			before.Data[plan.Pointer():plan.Pointer()+4],
		)
		if !bytes.Equal(preserved, before.Data) {
			t.Fatal(
				"resize modified coordinate resources or independent references",
			)
		}
		for _, use := range plan.Uses() {
			view, readErr := s.Maps.Read(use.Chapter)
			if readErr != nil {
				t.Fatal(readErr)
			}
			want := old
			if use.Updated {
				want = size
			}
			if image.Pt(view.Width, view.Height) != want {
				t.Fatal(
					"shared resize ownership",
					use,
					want,
					view.Width,
					view.Height,
				)
			}
		}
		reopened := historyAndReopen(t, s, before)
		next, err := reopened.Maps.NewDraft(row.ID)
		if err != nil || image.Pt(next.Width(), next.Height()) != size {
			t.Fatal("reopened map size", err)
		}
		if err = next.Resize(
			old.X,
			old.Y,
			maps.MapTopLeft,
			fill.Tile,
		); err != nil {
			t.Fatal(err)
		}
		expand, err := reopened.Maps.Plan(next)
		checkPlan(t, reopened, reopened.Document.Snapshot(), err)
		if err = reopened.Maps.Apply(expand); err != nil {
			t.Fatal(err)
		}
		view, err := reopened.Maps.Read(row.ID)
		if err != nil || image.Pt(view.Width, view.Height) != old {
			t.Fatal("continued resize", err)
		}
		return
	}
	t.Skip("no readable base map")
}
