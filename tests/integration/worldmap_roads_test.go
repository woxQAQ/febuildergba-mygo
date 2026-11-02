//go:build integration

package integration

import (
	"bytes"
	"encoding/binary"
	"image"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
)

func editWorldMapRoad(t *testing.T, s *game.Session) {
	if !s.WorldMap.RoutesAvailable() {
		t.Skip("no native world-map road editing")
	}
	routes, err := s.WorldMap.Routes()
	if err != nil {
		t.Skipf("route table unavailable: %v", err)
	}
	for _, route := range routes {
		d, err := s.WorldMap.NewRouteDraft(route.ID)
		if err != nil {
			continue
		}
		road, err := d.Road()
		if err != nil {
			continue
		}
		point, brush := image.Point{}, worldmap.RoadCell{Present: true}
		found := false
		for y := 0; y < worldmap.RoadRows && !found; y++ {
			for x := 0; x < worldmap.RoadColumns; x++ {
				if cell, _ := road.Cell(x, y); cell.Present {
					point, brush, found = image.Pt(x, y), cell, true
					brush.FlipX = !brush.FlipX
					break
				}
			}
		}
		if err = road.Stroke([]image.Point{point}, brush); err != nil {
			t.Fatal(err)
		}
		before := s.Document.Snapshot()
		plan, err := s.WorldMap.PlanRoute(d)
		checkPlan(t, s, before, err)
		if len(plan.RoadChanges()) != 1 {
			t.Fatal("expected one changed road cell")
		}
		if err = s.WorldMap.ApplyRoute(plan); err != nil {
			t.Fatal(err)
		}
		want := bytes.Clone(before.Data)
		binary.LittleEndian.PutUint32(
			want[route.Address:],
			uint32(plan.After().Road)+0x08000000,
		)
		if !bytes.Equal(want, s.Document.Snapshot().Data[:len(want)]) {
			t.Fatal(
				"road edit changed source stream, independent pointer, movement or padding",
			)
		}
		reopened := historyAndReopen(t, s, before)
		fresh, err := reopened.WorldMap.NewRouteDraft(route.ID)
		if err != nil {
			t.Fatal(err)
		}
		saved, err := fresh.Road()
		if err != nil {
			t.Fatal(err)
		}
		if cell, ok := saved.Cell(point.X, point.Y); !ok || cell != brush {
			t.Fatal("road differs after reopening", cell)
		}
		if scene := reopened.WorldMap.Scene(route.ID); scene.RoadErr != nil {
			t.Fatal("saved road cannot render", scene.RoadErr)
		}
		current, _ := reopened.WorldMap.Routes()
		for _, other := range current {
			if other.ID != route.ID && other.Road != routes[other.ID].Road {
				t.Fatal("unselected road redirected")
			}
		}
		t.Logf(
			"route=%d cell=(%d,%d) tile=%d preserved road references=%d",
			route.ID,
			point.X,
			point.Y,
			brush.Tile,
			len(plan.RoadShared()),
		)
		return
	}
	t.Skip("no road with confirmed stream and graphics")
}
