package worldmap_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

func TestRoadRouteCommitPreservesSharedSources(t *testing.T) {
	for _, version := range []string{"FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewWorldMapRoadFixture(version)
			s := testutil.Open(t, f)
			d, err := s.WorldMap.NewRouteDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			road, err := d.Road()
			if err != nil {
				t.Fatal(err)
			}
			for _, check := range []struct {
				point image.Point
				cell  worldmap.RoadCell
			}{
				{image.Pt(0, 1), worldmap.RoadCell{}},
				{image.Pt(2, 1), worldmap.RoadCell{Present: true}},
				{image.Pt(59, 2), worldmap.RoadCell{Tile: 1, Present: true}},
				{image.Pt(0, 3), worldmap.RoadCell{Tile: 1, FlipX: true, Present: true}},
			} {
				if got, ok := road.Cell(
					check.point.X,
					check.point.Y,
				); !ok ||
					got != check.cell {
					t.Fatal("native grid", check.point, got)
				}
			}
			brush := worldmap.RoadCell{
				Tile:    1,
				FlipX:   true,
				FlipY:   true,
				Present: true,
			}
			if err = road.Stroke(
				[]image.Point{image.Pt(6, 1), image.Pt(8, 1)},
				brush,
			); err != nil {
				t.Fatal(err)
			}
			if err = road.Stroke(
				[]image.Point{image.Pt(3, 1)},
				worldmap.RoadCell{},
			); err != nil {
				t.Fatal(err)
			}
			if err = d.SetNode(1, 5); err != nil {
				t.Fatal(err)
			}
			if err = d.SetPoints(
				[]worldmap.MovementPoint{{Progress: 2048, X: -1, Y: 55}},
			); err != nil {
				t.Fatal(err)
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			plan, err := s.WorldMap.PlanRoute(d)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.RoadChanges()) != 4 || len(plan.RoadShared()) != 31 ||
				!slices.Equal(plan.Shared(), []int{1}) {
				t.Fatal("plan changes/sharing")
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("planning changed ROM")
			}
			// Freeze the full transaction independently of later draft history.
			if !road.Undo() {
				t.Fatal("draft undo")
			}
			changes := plan.RoadChanges()
			changes[0].After = worldmap.RoadCell{}
			if err = s.WorldMap.ApplyRoute(plan); err != nil {
				t.Fatal(err)
			}
			want := bytes.Clone(before.Data)
			at := f.GetPosition("worldmap_routes")
			binary.LittleEndian.PutUint32(
				want[at:],
				testutil.PointerWord(len(want)),
			)
			binary.LittleEndian.PutUint32(
				want[at+8:],
				testutil.PointerWord(len(want)+40),
			)
			want[at+5] = 5
			want = append(want, []byte{
				1, 1, 2, 1, 0, 0, 0, 0,
				4, 1, 1, 1, 1, 12,
				6, 1, 3, 1, 1, 12, 1, 12, 1, 12,
				59, 2, 1, 1, 1, 0,
				0, 3, 1, 1, 1, 4,
				0xFF, 0, 0, 0,
				0, 8, 0, 0, 0xFF, 0xFF, 55, 0,
				0xFE, 0xFF, 0xFF, 0xFF, 0x91, 0xA2, 0xB3, 0xC4,
			}...)
			if !bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal(
					"unexpected stream, gap filling, changed source or unselected reference",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("atomic undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal("atomic redo")
			}
			if err = s.WorldMap.ApplyRoute(
				plan,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatal("replayed plan", err)
			}
			fresh, _ := s.WorldMap.NewRouteDraft(0)
			saved, _ := fresh.Road()
			if pixel := saved.Preview().
				RGBAAt(6*8, 8); pixel != (color.RGBA{B: 255, A: 255}) {
				t.Fatal("flipped preview", pixel)
			}
			other, _ := s.WorldMap.NewRouteDraft(1)
			old, _ := other.Road()
			if cell, _ := old.Cell(3, 1); !cell.Present {
				t.Fatal("shared road overwritten")
			}
		})
	}
}

func TestRoadStrokeHistoryAndEmptyLayout(t *testing.T) {
	s := testutil.Open(t, testutil.NewWorldMapRoadFixture("FE8U"))
	d, _ := s.WorldMap.NewRouteDraft(0)
	road, err := d.Road()
	if err != nil {
		t.Fatal(err)
	}
	initial := road.Preview()
	brush := worldmap.RoadCell{Tile: 1, Present: true}
	if err = road.Stroke(
		[]image.Point{image.Pt(5, 5), image.Pt(8, 8)},
		brush,
	); err != nil {
		t.Fatal(err)
	}
	for _, p := range []image.Point{image.Pt(5, 5), image.Pt(6, 6), image.Pt(7, 7), image.Pt(8, 8)} {
		if cell, _ := road.Cell(p.X, p.Y); cell != brush {
			t.Fatal("hole in stroke", p)
		}
	}
	painted := road.Preview()
	for _, invalid := range [][]image.Point{{image.Pt(1, 1), image.Pt(60, 1)}, {image.Pt(-1, 0)}, nil} {
		if err = road.Stroke(
			invalid,
			brush,
		); !errors.Is(err, edit.ErrValue) ||
			road.Preview() != painted {
			t.Fatal("failed stroke changed layout", err)
		}
	}
	if err = road.Stroke(
		[]image.Point{image.Pt(1, 1)},
		worldmap.RoadCell{Tile: road.TileCount(), Present: true},
	); !errors.Is(
		err,
		edit.ErrValue,
	) {
		t.Fatal(err)
	}
	if !road.Undo() || road.Modified() ||
		!bytes.Equal(initial.Pix, road.Preview().Pix) {
		t.Fatal("one stroke did not undo together")
	}
	if !road.Redo() || !bytes.Equal(painted.Pix, road.Preview().Pix) {
		t.Fatal("redo")
	}
	if !road.Undo() {
		t.Fatal("undo")
	}
	// Painting an explicit tile zero into a gap is a change, and erasing it
	// removes the cell rather than replacing it with another transparent tile.
	if err = road.Stroke(
		[]image.Point{image.Pt(0, 0)},
		worldmap.RoadCell{Present: true},
	); err != nil {
		t.Fatal(err)
	}
	if road.CanRedo() {
		t.Fatal("history branch retained")
	}
	if cell, _ := road.Cell(0, 0); !cell.Present {
		t.Fatal("explicit tile zero omitted")
	}
	if err = road.Stroke(
		[]image.Point{image.Pt(0, 0)},
		worldmap.RoadCell{},
	); err != nil ||
		road.Modified() {
		t.Fatal("erase", err)
	}
	var cells []image.Point
	for y := range worldmap.RoadRows {
		for x := range worldmap.RoadColumns {
			if cell, _ := road.Cell(x, y); cell.Present {
				cells = append(cells, image.Pt(x, y))
			}
		}
	}
	if err = road.Stroke(cells, worldmap.RoadCell{}); err != nil {
		t.Fatal(err)
	}
	if err = d.SetPoints(nil); err != nil {
		t.Fatal(err)
	}
	plan, err := s.WorldMap.PlanRoute(d)
	if err != nil {
		t.Fatal(err)
	}
	before := s.Document.Snapshot()
	if err = s.WorldMap.ApplyRoute(plan); err != nil {
		t.Fatal(err)
	}
	after := s.Document.Snapshot()
	if len(after.Data) != len(before.Data)+4 ||
		!bytes.Equal(after.Data[len(before.Data):], []byte{0xFF, 0, 0, 0}) ||
		plan.After().Road == 0 ||
		plan.After().Movement != 0 {
		t.Fatal(
			"empty road requires a valid terminator pointer; empty movement requires null",
		)
	}
}

func TestRoadEditingWithUnreadableBackground(t *testing.T) {
	f := testutil.NewWorldMapRoadFixture("FE8U")
	f.WriteU32(0xBA7FC, 0)
	s := testutil.Open(t, f)
	if scene := s.WorldMap.Scene(
		0,
	); scene.BackgroundErr == nil ||
		scene.RoadErr != nil {
		t.Fatal("independent image failures", scene)
	}
	d, err := s.WorldMap.NewRouteDraft(0)
	if err != nil {
		t.Fatal(err)
	}
	road, err := d.Road()
	if err != nil {
		t.Fatal("background blocked roads", err)
	}
	if err = road.Stroke(
		[]image.Point{image.Pt(5, 5)},
		worldmap.RoadCell{Tile: 1, Present: true},
	); err != nil {
		t.Fatal(err)
	}
	plan, err := s.WorldMap.PlanRoute(d)
	if err != nil {
		t.Fatal(err)
	}
	before := s.Document.Snapshot()
	if err = s.WorldMap.ApplyRoute(plan); err != nil {
		t.Fatal(err)
	}
	want := bytes.Clone(before.Data)
	binary.LittleEndian.PutUint32(
		want[f.GetPosition("worldmap_routes"):],
		testutil.PointerWord(plan.After().Road),
	)
	if !bytes.Equal(want, s.Document.Snapshot().Data[:len(want)]) {
		t.Fatal("road editing rewrote background")
	}
}

func TestRoadPlanFailuresAndIndependentFields(t *testing.T) {
	for _, scenario := range []string{"revision", "layout", "document", "overlap", "capacity", "combined-capacity", "malformed", "unknown-tile", "unknown-bits", "broken-palette"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewWorldMapRoadFixture("FE8J")
			at := f.GetPosition("worldmap_routes")
			switch scenario {
			case "malformed":
				f.WriteU8(f.GetPosition("worldmap_road"), 60)
			case "unknown-tile":
				f.WriteU16(f.GetPosition("worldmap_road")+4, 500)
			case "unknown-bits":
				f.WriteU16(f.GetPosition("worldmap_road")+4, 0x1000)
			case "broken-palette":
				f.WriteU32(0xBDE5C, 0)
			case "capacity", "combined-capacity":
				size := rom.MaxSize
				if scenario == "combined-capacity" {
					size -= 40
				}
				full := testutil.NewFixture("BE8J01", size)
				full.WriteAt(0, f.Bytes())
				f = full
			}
			s := testutil.Open(t, f)
			if scenario == "overlap" {
				if err := s.Records.UseLayout(
					records.Units,
					records.Layout{Address: at, Stride: 52, Count: 1},
				); err != nil {
					t.Fatal(err)
				}
			}
			d, err := s.WorldMap.NewRouteDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			road, roadErr := d.Road()
			if roadErr != nil {
				if scenario != "malformed" && scenario != "unknown-tile" &&
					scenario != "unknown-bits" &&
					scenario != "broken-palette" {
					t.Fatal(roadErr)
				}
				if road != nil || !errors.Is(roadErr, worldmap.ErrScene) {
					t.Fatal("unconfirmed road writable", roadErr)
				}
				if err = d.SetNode(1, 5); err != nil {
					t.Fatal(err)
				}
				plan, err := s.WorldMap.PlanRoute(d)
				if err != nil {
					t.Fatal("unrelated road blocked endpoints", err)
				}
				before := s.Document.Snapshot()
				if err = s.WorldMap.ApplyRoute(plan); err != nil {
					t.Fatal(err)
				}
				want := bytes.Clone(before.Data)
				want[at+5] = 5
				if !bytes.Equal(want, s.Document.Snapshot().Data) {
					t.Fatal("rewrote unknown road")
				}
				return
			}
			if err = road.Stroke(
				[]image.Point{image.Pt(5, 5)},
				worldmap.RoadCell{Tile: 1, Present: true},
			); err != nil {
				t.Fatal(err)
			}
			if scenario == "combined-capacity" {
				if err = d.SetPoints(
					[]worldmap.MovementPoint{{Progress: 2048, X: 10, Y: 10}},
				); err != nil {
					t.Fatal(err)
				}
			}
			var plan *worldmap.RoutePlan
			want := error(rom.ErrStale)
			switch scenario {
			case "overlap":
				want = romwrite.ErrClaim
			case "capacity", "combined-capacity":
				want = romwrite.ErrCapacity
			default:
				plan, err = s.WorldMap.PlanRoute(d)
				if err != nil {
					t.Fatal(err)
				}
				switch scenario {
				case "revision":
					err = s.Records.Set(records.Units, []int{1}, "level", 9)
				case "layout":
					table := s.Records.Table(records.Chapters)
					err = s.Records.UseLayout(
						records.Chapters,
						records.Layout{
							Address: table.Address,
							Stride:  table.Stride,
							Count:   1,
						},
					)
				case "document":
					s = testutil.Open(t, f)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			if plan != nil {
				err = s.WorldMap.ApplyRoute(plan)
			} else {
				_, err = s.WorldMap.PlanRoute(d)
			}
			if !errors.Is(err, want) {
				t.Fatal("unexpected failure", err, want)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("failure changed ROM/history")
			}
		})
	}
}
