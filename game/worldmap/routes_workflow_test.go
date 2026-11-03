package worldmap_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image/color"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestNativeRoutesEditAndHistory(t *testing.T) {
	for _, version := range []string{"FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewWorldMapRouteFixture(version)
			s := testutil.Open(t, f)
			routes, err := s.WorldMap.Routes()
			if err != nil || len(routes) != 32 {
				t.Fatal(routes, err)
			}
			original := []worldmap.MovementPoint{
				{Progress: 1024, X: -2, Y: 24},
				{Progress: 3072, X: 48, Y: 40},
			}
			if !slices.Equal(routes[0].Points(), original) ||
				routes[2].Movement != 0 {
				t.Fatal("native read", routes[0].Points(), routes[2])
			}
			d, err := s.WorldMap.NewRouteDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			if err = d.SetNode(1, 5); err != nil {
				t.Fatal(err)
			}
			points := []worldmap.MovementPoint{
				{Progress: 512, X: -32768, Y: 32767},
				{Progress: 2048, X: 41, Y: 52},
				{Progress: 3584, X: 80, Y: -4},
			}
			if err = d.SetPoints(points); err != nil {
				t.Fatal(err)
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			plan, err := s.WorldMap.PlanRoute(d)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("preview changed document")
			}
			if !slices.Equal(plan.Shared(), []int{1}) {
				t.Fatal("shared users", plan.Shared())
			}
			// Changes to the draft and returned slices cannot mutate a frozen plan.
			points[0].X = 7
			if err = d.SetPoints(points); err != nil {
				t.Fatal(err)
			}
			copyPoints := plan.After().Points()
			copyPoints[0].X = 8
			if err = s.WorldMap.ApplyRoute(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			want := bytes.Clone(before.Data)
			at := f.GetPosition("worldmap_routes")
			want[at+5] = 5
			binary.LittleEndian.PutUint32(
				want[at+8:],
				testutil.PointerWord(len(before.Data)),
			)
			want = append(want, []byte{
				0, 2, 0, 0, 0, 0x80, 0xFF, 0x7F,
				0, 8, 0, 0, 41, 0, 52, 0,
				0, 14, 0, 0, 80, 0, 0xFC, 0xFF,
				0xFE, 0xFF, 0xFF, 0xFF, 0x91, 0xA2, 0xB3, 0xC4,
			}...)
			if !bytes.Equal(after.Data, want) {
				t.Fatal(
					"unexpected bytes, source overwrite, padding or terminator loss",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal("redo")
			}
			routes, err = s.WorldMap.Routes()
			if err != nil || !slices.Equal(routes[1].Points(), original) ||
				routes[0].Points()[0].X != -32768 {
				t.Fatal("shared path changed", routes, err)
			}
			if err = s.WorldMap.ApplyRoute(
				plan,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatal("replayed plan", err)
			}
		})
	}
}

func TestRoutePointCreationAndRemoval(t *testing.T) {
	s := testutil.Open(t, testutil.NewWorldMapRouteFixture("FE8U"))
	d, _ := s.WorldMap.NewRouteDraft(2)
	points := []worldmap.MovementPoint{
		{Progress: 300, X: 1, Y: 2},
		{Progress: 800, X: 2, Y: 3},
		{Progress: 1300, X: 3, Y: 4},
		{Progress: 1800, X: 4, Y: 5},
		{Progress: 2300, X: 5, Y: 6},
		{Progress: 2800, X: 6, Y: 7},
	}
	if err := d.SetPoints(
		append(slices.Clone(points), worldmap.MovementPoint{Progress: 3300}),
	); !errors.Is(
		err,
		edit.ErrValue,
	) {
		t.Fatal("capacity", err)
	}
	if d.Modified() {
		t.Fatal("failed mutation changed draft")
	}
	if err := d.SetPoints(points); err != nil {
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
	data := s.Document.Snapshot().Data
	if !bytes.Equal(
		data[len(data)-8:],
		[]byte{255, 255, 255, 255, 0, 0, 0, 0},
	) {
		t.Fatal("new terminator")
	}
	if len(data) != len(before.Data)+56 {
		t.Fatal("payload size")
	}
	for _, count := range []int{3, 0} {
		d, _ = s.WorldMap.NewRouteDraft(2)
		if err = d.SetPoints(points[:count]); err != nil {
			t.Fatal(err)
		}
		plan, err = s.WorldMap.PlanRoute(d)
		if err != nil {
			t.Fatal(err)
		}
		old := s.Document.Snapshot()
		if err = s.WorldMap.ApplyRoute(plan); err != nil {
			t.Fatal(err)
		}
		current := s.Document.Snapshot()
		at := plan.Before().Movement
		if !bytes.Equal(old.Data[at:], current.Data[at:len(old.Data)]) {
			t.Fatal("old payload reclaimed")
		}
		if count == 0 &&
			(len(current.Data) != len(old.Data) || gba.Uint32(current.Data, plan.Before().Address+8) != 0) {
			t.Fatal("empty path was not cleared in place")
		}
	}
}

func TestRouteFailuresAreAtomic(t *testing.T) {
	for _, scenario := range []string{"revision", "layout", "document", "duplicate", "self", "capacity", "progress", "record", "movement-overlap", "road-overlap", "halfword-road-overlap", "node-overlap", "scene-overlap", "root-overlap", "unreadable-movement"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewWorldMapRouteFixture("FE8J")
			at := f.GetPosition("worldmap_routes")
			switch scenario {
			case "movement-overlap":
				f.WritePointer(at+12+8, at)
			case "road-overlap":
				f.WritePointer(at+12, at+4)
			case "halfword-road-overlap":
				f.WritePointer(at+12, at+6)
			case "node-overlap":
				f.WritePointer(f.GetPosition("worldmap_nodes_pointer"), at)
			case "scene-overlap":
				f.WritePointer(0xBF698, at)
			case "root-overlap":
				f.WritePointer(
					f.GetPosition("worldmap_routes_pointer"),
					f.GetPosition("worldmap_routes_pointer"),
				)
			case "unreadable-movement":
				f.WritePointer(at+8, 0x7FFFFC)
			case "capacity":
				for i := 4; i < 11; i++ {
					f.WriteAt(at+i*12+4, []byte{5, byte(i + 5)})
				}
			}
			s := testutil.Open(t, f)
			if scenario == "record" {
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
			if err = d.SetNode(1, 5); err != nil {
				t.Fatal(err)
			}
			want := error(space.ErrClaim)
			switch scenario {
			case "duplicate":
				_ = d.SetNode(1, 3)
				want = worldmap.ErrConnections
			case "self":
				_ = d.SetNode(1, 0)
				want = worldmap.ErrConnections
			case "capacity":
				want = worldmap.ErrConnections
			case "progress":
				_ = d.SetPoints(
					[]worldmap.MovementPoint{
						{Progress: 3000},
						{Progress: 1000},
					},
				)
				want = worldmap.ErrMovement
			case "unreadable-movement":
				before := s.Document.State()
				if err = d.SetPoints(
					[]worldmap.MovementPoint{{Progress: 1024}},
				); !errors.Is(err, worldmap.ErrMovement) ||
					before != s.Document.State() {
					t.Fatal(err)
				}
				return
			}
			var plan *worldmap.RoutePlan
			if scenario == "revision" || scenario == "layout" ||
				scenario == "document" {
				plan, err = s.WorldMap.PlanRoute(d)
				if err != nil {
					t.Fatal(err)
				}
				want = rom.ErrStale
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
				t.Fatalf("got %v, want %v", err, want)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("failure mutated bytes/history")
			}
		})
	}
}

func TestRouteBoundariesAndCapacity(t *testing.T) {
	for _, scenario := range []string{"absent-table", "unaligned-table", "truncated-table", "unterminated-movement", "rom-capacity"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewWorldMapRouteFixture("FE8U")
			root, at := f.GetPosition(
				"worldmap_routes_pointer",
			), f.GetPosition(
				"worldmap_routes",
			)
			switch scenario {
			case "absent-table":
				f.WriteU32(root, 0)
			case "unaligned-table":
				f.WritePointer(root, at+1)
			case "truncated-table":
				f.WritePointer(root, len(f.Bytes())-16)
			case "unterminated-movement":
				f.WriteAt(f.GetPosition("worldmap_movement"), make([]byte, 56))
			case "rom-capacity":
				full := testutil.NewFixture("BE8E01", rom.MaxSize)
				full.WriteAt(0, f.Bytes())
				f = full
			}
			s := testutil.Open(t, f)
			before, state := s.Document.Snapshot(), s.Document.State()
			d, err := s.WorldMap.NewRouteDraft(0)
			switch scenario {
			case "absent-table", "unaligned-table", "truncated-table":
				if !errors.Is(err, worldmap.ErrRoutes) {
					t.Fatal(err)
				}
			case "unterminated-movement":
				if err != nil ||
					!errors.Is(d.Route().MovementErr, worldmap.ErrMovement) {
					t.Fatal(err)
				}
				if err = d.SetPoints(
					[]worldmap.MovementPoint{{Progress: 2048}},
				); !errors.Is(
					err,
					worldmap.ErrMovement,
				) {
					t.Fatal(err)
				}
			case "rom-capacity":
				if err != nil {
					t.Fatal(err)
				}
				if err = d.SetPoints(
					[]worldmap.MovementPoint{{Progress: 2048}},
				); err != nil {
					t.Fatal(err)
				}
				if err = d.SetNode(1, 5); err != nil {
					t.Fatal(err)
				}
				if _, err = s.WorldMap.PlanRoute(
					d,
				); !errors.Is(
					err,
					space.ErrCapacity,
				) {
					t.Fatal(err)
				}
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("failed read/plan changed document")
			}
		})
	}
}

func TestRouteSceneAndIndependentEditing(t *testing.T) {
	for _, version := range []string{"FE8J", "FE8U"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewWorldMapRouteFixture(version)
			s := testutil.Open(t, f)
			scene := s.WorldMap.Scene(0)
			if scene.BackgroundErr != nil || scene.RoadErr != nil {
				t.Fatal(scene.BackgroundErr, scene.RoadErr)
			}
			red, green, blue, white := color.RGBA{
				255,
				0,
				0,
				255,
			}, color.RGBA{
				0,
				255,
				0,
				255,
			}, color.RGBA{
				0,
				0,
				255,
				255,
			}, color.RGBA{
				255,
				255,
				255,
				255,
			}
			for _, check := range []struct {
				x, y  int
				color color.RGBA
			}{{0, 0, red}, {8, 0, green}, {0, 8, blue}, {0, 312, white}} {
				if got := scene.Background.RGBAAt(
					check.x,
					check.y,
				); got != check.color {
					t.Fatalf("background (%d,%d): %v", check.x, check.y, got)
				}
			}
			for _, check := range []struct {
				x, y  int
				color color.RGBA
			}{{8, 8, red}, {23, 8, green}, {16, 15, blue}, {24, 15, green}, {31, 8, blue}, {39, 15, green}, {32, 8, blue}, {9, 8, color.RGBA{}}, {8, 16, color.RGBA{}}} {
				if got := scene.Road.RGBAAt(
					check.x,
					check.y,
				); got != check.color {
					t.Fatalf("road (%d,%d): %v", check.x, check.y, got)
				}
			}
			if s.WorldMap.Scene(0).Background != scene.Background {
				t.Fatal("scene cache not reused")
			}
			d, _ := s.WorldMap.NewRouteDraft(0)
			if err := d.SetNode(1, 5); err != nil {
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
			want[f.GetPosition("worldmap_routes")+5] = 5
			if !bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal("endpoint edit changed other bytes")
			}
			if s.WorldMap.Scene(0).Background == scene.Background {
				t.Fatal("scene outlived revision")
			}
			// Invalid compressed assets retain the decoder's cause and do not
			// prevent independent endpoint editing.
			f.WriteU8(f.GetPosition("worldmap_palette_map"), 0)
			f.WriteU8(f.GetPosition("worldmap_road_tiles"), 0)
			s = testutil.Open(t, f)
			scene = s.WorldMap.Scene(0)
			if !errors.Is(scene.BackgroundErr, gba.ErrGraphics) ||
				!errors.Is(scene.BackgroundErr, worldmap.ErrScene) ||
				!errors.Is(scene.RoadErr, gba.ErrGraphics) {
				t.Fatal(
					"lost graphics error cause",
					scene.BackgroundErr,
					scene.RoadErr,
				)
			}
			// Broken unrelated graphics and movement do not prevent endpoint changes.
			f.WriteU32(f.GetPosition("worldmap_routes"), 0)
			f.WriteU32(f.GetPosition("worldmap_routes")+8, 1)
			s = testutil.Open(t, f)
			d, _ = s.WorldMap.NewRouteDraft(0)
			_ = d.SetNode(1, 5)
			if _, err = s.WorldMap.PlanRoute(d); err != nil {
				t.Fatal("unrelated graphics/movement blocked endpoints", err)
			}
		})
	}
}
