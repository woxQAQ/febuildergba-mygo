package game

import (
	"bytes"
	"errors"
	"image"
	"reflect"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestMapResizeWorkflow(t *testing.T) {
	for _, version := range []string{"FE6", "FE7J", "FE7U", "FE8J", "FE8U"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewMapResizeFixture(version)
			s := openFixture(t, f)
			before := s.Document.Snapshot()
			d, err := s.NewMapDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			if err = d.Resize(3, 4, MapTopLeft, 0); err != nil {
				t.Fatal(err)
			}
			if cell, _ := d.Cell(1, 2); cell.Tile != 4 {
				t.Fatal("retained cell", cell)
			}
			if d.Width() != 3 || d.Height() != 4 || !d.Modified() {
				t.Fatal("resize dimensions")
			}
			if err = d.Stroke([]image.Point{{0, 0}}, 4); err != nil {
				t.Fatal(err)
			}
			if !d.Undo() {
				t.Fatal("stroke undo")
			}
			if cell, _ := d.Cell(0, 0); cell.Tile != 0 {
				t.Fatal("stroke was not undone")
			}
			if !d.Undo() || d.Width() != 6 || d.Height() != 10 || d.Modified() {
				t.Fatal("resize undo")
			}
			if !d.Redo() || d.Width() != 3 || d.Height() != 4 {
				t.Fatal("resize redo")
			}
			plan, err := s.PlanMapEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if !plan.Resized() || plan.BeforeSize() != image.Pt(6, 10) ||
				plan.AfterSize() != image.Pt(3, 4) ||
				plan.ChangedCells() != 48 {
				t.Fatalf("resize summary: %+v", plan)
			}
			items := plan.Impacts()
			var area, change, alreadyOutside, script, path bool
			for _, item := range items {
				if item.Chapter == 2 {
					t.Fatal("retained chapter included", item)
				}
				if item.Chapter != 0 {
					continue
				}
				switch {
				case item.Kind == MapTriggerArea && item.Address == f.GetPosition("area_table")+12:
					area = item.LostCells == 11 && !item.BeforeOutside &&
						item.AfterOutside
				case item.Kind == MapChangeArea && item.Index == 2:
					change = item.LostCells == 1 && item.BeforeOutside &&
						item.AfterOutside
				case item.Kind == MapUnitStart && item.Index == 1 && item.Bounds.Min == image.Pt(7, 12):
					alreadyOutside = item.LostCells == 0 &&
						item.BeforeOutside &&
						item.AfterOutside
				case item.Kind == MapScriptPosition && item.Bounds.Min == image.Pt(4, 8):
					script = item.LostCells == 1 && !item.BeforeOutside
				case item.Kind == MapScriptPath || item.Kind == MapMovementNode:
					path = path || (item.LostCells == 1 && !item.BeforeOutside)
				}
			}
			if !area || !change || !alreadyOutside || !script || !path {
				t.Fatalf(
					"impact coverage: area=%v change=%v existing=%v script=%v path=%v\n%+v",
					area,
					change,
					alreadyOutside,
					script,
					path,
					items,
				)
			}
			items[0].Chapter = 99
			if plan.Impacts()[0].Chapter == 99 {
				t.Fatal("mutable impact slice")
			}
			warnings := plan.ImpactWarnings()
			if len(warnings) == 0 {
				t.Fatal("opaque references were not reported")
			}
			warnings[0].Chapter = 99
			if plan.ImpactWarnings()[0].Chapter == 99 {
				t.Fatal("mutable warnings")
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("preview wrote bytes")
			}
			if err = d.Resize(8, 12, MapCenter, 4); err != nil {
				t.Fatal(err)
			}
			if err = s.ApplyMapEdit(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			unchanged := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				unchanged[plan.Pointer():plan.Pointer()+4],
				before.Data[plan.Pointer():plan.Pointer()+4],
			)
			if !bytes.Equal(unchanged, before.Data) {
				t.Fatal("resize changed old resource or coordinate bytes")
			}
			layout, err := gba.DecompressLZ77(
				after.Data[plan.Destination():],
				maxMapBytes,
			)
			if err != nil || len(layout) != 2+3*4*2+4 ||
				!bytes.Equal(
					layout[len(layout)-4:],
					[]byte{0xA5, 0x5A, 0xC3, 0x7E},
				) {
				t.Fatal("decoded tail lost", layout, err)
			}
			for chapter := range 3 {
				view, e := s.ChapterMap(chapter)
				if e != nil {
					t.Fatal(e)
				}
				want := image.Pt(3, 4)
				if chapter == 2 {
					want = image.Pt(6, 10)
				}
				if image.Pt(view.Width, view.Height) != want {
					t.Fatal(
						"shared map ownership",
						chapter,
						view.Width,
						view.Height,
					)
				}
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("atomic resize undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("atomic resize redo")
			}
			if err = s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			next, err := reopened.NewMapDraft(0)
			if err != nil || next.Width() != 3 || next.Height() != 4 {
				t.Fatal("reopened dimensions", err)
			}
			if err = next.Resize(6, 10, MapTopLeft, 4); err != nil {
				t.Fatal(err)
			}
			restored, err := reopened.PlanMapEdit(next)
			if err != nil {
				t.Fatal(err)
			}
			resolved := false
			for _, item := range restored.Impacts() {
				if item.Kind == MapTriggerArea && item.Chapter == 0 {
					resolved = item.BeforeOutside && !item.AfterOutside &&
						item.GainedCells == 11
				}
			}
			if !resolved {
				t.Fatal("expansion did not resolve the trigger area")
			}
		})
	}
}

func TestMapResizeAnchorsAndLimits(t *testing.T) {
	anchors := []struct {
		anchor MapAnchor
		point  image.Point
	}{
		{MapTopLeft, image.Pt(0, 0)},
		{MapTop, image.Pt(1, 0)},
		{MapTopRight, image.Pt(3, 0)},
		{MapLeft, image.Pt(0, 1)},
		{MapCenter, image.Pt(1, 1)},
		{MapRight, image.Pt(3, 1)},
		{MapBottomLeft, image.Pt(0, 3)},
		{MapBottom, image.Pt(1, 3)},
		{MapBottomRight, image.Pt(3, 3)},
	}
	for _, version := range []string{"FE6", "FE7J", "FE7U", "FE8J", "FE8U"} {
		t.Run(version, func(t *testing.T) {
			s := openFixture(t, testutil.NewMapFixture(version))
			for _, anchor := range anchors {
				d, err := s.NewMapDraft(0)
				if err != nil {
					t.Fatal(err)
				}
				if err = d.Resize(5, 4, anchor.anchor, 0); err != nil {
					t.Fatal(err)
				}
				for y := range 4 {
					for x := range 5 {
						cell, _ := d.Cell(x, y)
						want := uint16(0)
						if image.Pt(x, y) == anchor.point.Add(image.Pt(1, 0)) {
							want = 4
						}
						if cell.Tile != want {
							t.Fatal(
								"anchor placement",
								anchor.anchor,
								x,
								y,
								cell.Tile,
								want,
							)
						}
					}
				}
				if err = d.Resize(
					2,
					1,
					anchor.anchor,
					4,
				); err != nil ||
					d.Modified() {
					t.Fatal("odd centered crop", anchor.anchor, err)
				}
				if !d.Undo() || d.Width() != 5 || d.Height() != 4 {
					t.Fatal("crop undo")
				}
				if !d.Undo() || d.Modified() {
					t.Fatal("expansion undo")
				}
				if err = d.Fill(
					image.Rect(0, 0, 2, 1),
					4,
				); err != nil ||
					d.CanRedo() {
					t.Fatal("history branch", err)
				}
			}
			d, _ := s.NewMapDraft(0)
			for _, size := range []image.Point{{60, 25}, {24, 63}, {32, 48}, {1, 1}} {
				if err := d.Resize(size.X, size.Y, MapCenter, 4); err != nil {
					t.Fatal("valid native size", size, err)
				}
				if _, err := s.PlanMapEdit(d); err != nil {
					t.Fatal("preview native size", size, err)
				}
			}
			before, _ := d.Preview()
			for _, invalid := range []struct {
				w, h   int
				anchor MapAnchor
				tile   uint16
			}{
				{0, 1, MapTopLeft, 0},
				{1, 0, MapTopLeft, 0},
				{61, 1, MapTopLeft, 0},
				{1, 64, MapTopLeft, 0},
				{60, 26, MapTopLeft, 0},
				{2, 1, MapAnchor(9), 0},
				{2, 1, MapTopLeft, 0xFFFF},
			} {
				if err := d.Resize(
					invalid.w,
					invalid.h,
					invalid.anchor,
					invalid.tile,
				); !errors.Is(
					err,
					ErrValue,
				) {
					t.Fatal("bad resize accepted", invalid, err)
				}
				after, _ := d.Preview()
				if !reflect.DeepEqual(before, after) {
					t.Fatal("failed resize changed draft")
				}
			}
			if s.Document.State().Dirty {
				t.Fatal("draft history changed document")
			}
		})
	}
	withTail := openFixture(t, testutil.NewMapResizeFixture("FE8U"))
	d, _ := withTail.NewMapDraft(0)
	if err := d.Resize(
		32,
		48,
		MapTopLeft,
		0,
	); !errors.Is(err, ErrValue) ||
		d.Modified() {
		t.Fatal("unknown tail would exceed native unpack buffer", err)
	}
}

func TestMapResizeIncompleteInspection(t *testing.T) {
	for _, mode := range []string{"invalid change root", "truncated change table", "report limit"} {
		t.Run(mode, func(t *testing.T) {
			f := testutil.NewMapResizeFixture("FE8U")
			switch mode {
			case "invalid change root":
				f.WriteU32(f.GetPosition("resize_changes_root"), 0x02000000)
			case "truncated change table":
				at := len(f.Bytes()) - 12
				f.WritePointer(0x221000+9*4, at)
				f.WriteAt(at, []byte{1, 5, 8, 1, 1})
			case "report limit":
				base := f.GetPosition("chapters")
				row := bytes.Clone(f.Bytes()[base : base+148])
				for chapter := range 64 {
					f.WriteAt(base+chapter*148, row)
				}
				f.WritePointer(f.GetPosition("deploy_header")+12, 0x260000)
				for index := range 128 {
					f.WriteAt(
						0x260000+index*12,
						[]byte{11, 0, 1, 0, 0, 0, 0, 0, 4, 5, 4, 5},
					)
				}
			}
			s := openFixture(t, f)
			if mode == "report limit" {
				if err := s.UseLayout(
					Chapters,
					Layout{
						Address: f.GetPosition("chapters"),
						Stride:  148,
						Count:   64,
					},
				); err != nil {
					t.Fatal(err)
				}
			}
			d, err := s.NewMapDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			if err = d.Resize(3, 4, MapTopLeft, 0); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanMapEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, warning := range plan.ImpactWarnings() {
				if mode == "report limit" {
					found = found || errors.Is(warning.Err, ErrMapImpactLimit)
				} else {
					found = found || warning.Kind == MapChangeArea
				}
			}
			if !found || len(plan.Impacts()) > maxMapImpacts {
				t.Fatal(
					"missing incomplete-inspection warning",
					plan.ImpactWarnings(),
				)
			}
			if s.Document.State().Dirty {
				t.Fatal("inspection wrote ROM")
			}
		})
	}
}
