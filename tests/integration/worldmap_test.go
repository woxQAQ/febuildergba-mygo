//go:build integration

package integration

import (
	"bytes"
	"encoding/binary"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
)

func editWorldMapNode(t *testing.T, s *game.Session) {
	if !s.WorldMap.Available() {
		t.Skip("no native world-map node editor for this input")
	}
	nodes, err := s.WorldMap.Nodes()
	if err != nil || len(nodes) == 0 {
		t.Skipf("world-map nodes unavailable: %v", err)
	}
	node := nodes[0]
	draft, err := s.WorldMap.NewDraft(node.ID)
	if err != nil {
		t.Fatal(err)
	}
	before := s.Document.Snapshot()
	want := bytes.Clone(before.Data)
	// Coordinates, references and source bytes come from this input, not a
	// particular release. Only the native field offsets are fixed expectations.
	x := node.Value(worldmap.X) ^ 1
	if err := draft.Set(worldmap.X, x); err != nil {
		t.Fatal(err)
	}
	binary.LittleEndian.PutUint16(want[node.Address+24:], uint16(x))
	texts, _ := s.Texts.Entries()
	for _, entry := range texts {
		if entry.Err == nil && entry.ID != node.Name {
			if err := draft.Set(worldmap.Name, int(entry.ID)); err != nil {
				t.Fatal(err)
			}
			binary.LittleEndian.PutUint16(want[node.Address+28:], entry.ID)
			break
		}
	}
	if chapters := s.Records.Table(
		records.Chapters,
	); chapters != nil &&
		chapters.Err == nil {
		for _, row := range chapters.Rows {
			if row.ID >= 0 && row.ID < 256 &&
				row.ID != node.Value(worldmap.EirikaChapter) {
				if err := draft.Set(
					worldmap.EirikaChapter,
					row.ID,
				); err != nil {
					t.Fatal(err)
				}
				want[node.Address+4] = byte(row.ID)
				break
			}
		}
	}
	plan, err := s.WorldMap.Plan(draft)
	checkPlan(t, s, before, err)
	t.Logf(
		"node=%d address=0x%08X fields=%d",
		node.ID,
		node.Address,
		len(plan.Changes()),
	)
	if err := s.WorldMap.Apply(plan); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, s.Document.Snapshot().Data) {
		t.Fatal(
			"node edit changed unrelated bytes, inventory pointers or file length",
		)
	}
	reopened := historyAndReopen(t, s, before)
	saved, err := reopened.WorldMap.NewDraft(node.ID)
	if err != nil || saved.Node().Value(worldmap.X) != x ||
		saved.Node().Name != draft.Node().Name ||
		saved.Node().
			Value(worldmap.EirikaChapter) !=
			draft.Node().
				Value(worldmap.EirikaChapter) {
		t.Fatal("node fields differ after reopening", err)
	}
}

func editWorldMapRoute(t *testing.T, s *game.Session) {
	if !s.WorldMap.RoutesAvailable() {
		t.Skip("no native world-map route editor")
	}
	routes, err := s.WorldMap.Routes()
	if err != nil {
		t.Skipf("route table unavailable: %v", err)
	}
	for _, route := range routes {
		if route.MovementErr != nil {
			continue
		}
		draft, err := s.WorldMap.NewRouteDraft(route.ID)
		if err != nil {
			t.Fatal(err)
		}
		points := route.Points()
		if len(points) == 0 {
			points = []worldmap.MovementPoint{
				{Progress: worldmap.MovementEnd / 2},
			}
		} else {
			points[0].X ^= 1
		}
		if err := draft.SetPoints(points); err != nil {
			continue
		}
		before := s.Document.Snapshot()
		plan, err := s.WorldMap.PlanRoute(draft)
		if err != nil {
			continue
		}
		// Discover an allowable endpoint change using this ROM's graph.
		for id := range worldmap.NodeCount {
			if id == route.Nodes[1] {
				continue
			}
			if err := draft.SetNode(1, id); err != nil {
				t.Fatal(err)
			}
			candidate, err := s.WorldMap.PlanRoute(draft)
			if err == nil {
				plan = candidate
				break
			}
		}
		checkPlan(t, s, before, nil)
		scene := s.WorldMap.Scene(route.ID)
		t.Logf(
			"route=%d points=%d shared=%v background=%v road=%v",
			route.ID,
			len(points),
			plan.Shared(),
			scene.BackgroundErr,
			scene.RoadErr,
		)
		if err := s.WorldMap.ApplyRoute(plan); err != nil {
			t.Fatal(err)
		}
		data := s.Document.Snapshot().Data
		want := bytes.Clone(before.Data)
		endpoints := plan.After().Nodes
		want[route.Address+4], want[route.Address+5] = byte(
			endpoints[0],
		), byte(
			endpoints[1],
		)
		binary.LittleEndian.PutUint32(
			want[route.Address+8:],
			uint32(plan.After().Movement)+0x08000000,
		)
		if !bytes.Equal(want, data[:len(want)]) {
			t.Fatal(
				"changed unselected routes, source payload, padding or graphics",
			)
		}
		reopened := historyAndReopen(t, s, before)
		saved, err := reopened.WorldMap.NewRouteDraft(route.ID)
		if err != nil || !slices.Equal(saved.Route().Points(), points) ||
			saved.Route().Nodes != endpoints {
			t.Fatal("route differs after reopening", err)
		}
		for _, other := range routes {
			if other.ID == route.ID {
				continue
			}
			current, err := reopened.WorldMap.NewRouteDraft(other.ID)
			if err != nil || current.Route().Movement != other.Movement ||
				!slices.Equal(current.Route().Points(), other.Points()) {
				t.Fatal("independent route changed", other.ID, err)
			}
		}
		return
	}
	t.Skip("no readable route with a writable record")
}
