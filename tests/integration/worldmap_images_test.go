//go:build integration

package integration

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

func editWorldMapDarkPalette(t *testing.T, s *game.Session) {
	if !s.WorldMap.Available() {
		t.Skip("no native world map")
	}
	source, err := s.WorldMap.Background(worldmap.Darkened)
	if err != nil {
		t.Skipf("dark palette unavailable: %v", err)
	}
	content, err := source.PNG()
	if err != nil {
		t.Fatal(err)
	}
	bitmap, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	indexed, ok := bitmap.(*image.Paletted)
	if !ok {
		t.Fatal("indexed export")
	}
	index := indexed.ColorIndexAt(0, 0)
	pixel := color.NRGBAModel.Convert(indexed.Palette[index]).(color.NRGBA)
	pixel.R = 255 - pixel.R
	indexed.Palette[index] = pixel
	var replacement bytes.Buffer
	if err = png.Encode(&replacement, indexed); err != nil {
		t.Fatal(err)
	}
	before := s.Document.Snapshot()
	day := s.WorldMap.Scene(0).Background
	draft, err := source.ImportPNG(replacement.Bytes())
	checkPlan(t, s, before, err)
	plan, err := s.WorldMap.PlanBackground(draft)
	checkPlan(t, s, before, err)
	if err = s.WorldMap.ApplyBackground(plan); err != nil {
		t.Fatal(err)
	}
	after := s.Document.Snapshot()
	site := -1
	for at := range len(before.Data) {
		if before.Data[at] != after.Data[at] {
			if site < 0 {
				site = at &^ 3
			}
			if at < site || at >= site+4 {
				t.Fatal("dark edit changed independent data")
			}
		}
	}
	if site < 0 {
		t.Fatal("dark pointer unchanged")
	}
	old, ok := gba.Pointer(before.Data, site)
	if !ok || !gba.Within(before.Data, old, 128) {
		t.Fatal("old palette")
	}
	current, ok := gba.Pointer(after.Data, site)
	if !ok || current != plan.Appended().Start {
		t.Fatal("dark pointer")
	}
	expected := bytes.Clone(before.Data[old : old+128])
	expected[int(index)*2] ^= 31
	if !bytes.Equal(expected, after.Data[current:]) {
		t.Fatal("dark color bits")
	}
	reopened := historyAndReopen(t, s, before)
	scene := reopened.WorldMap.Scene(0)
	if scene.DarkBackgroundErr != nil ||
		scene.DarkBackground.RGBAAt(0, 0) != draft.Image().RGBAAt(0, 0) {
		t.Fatal("saved dark preview", scene.DarkBackgroundErr)
	}
	if day != nil &&
		(scene.Background == nil || !bytes.Equal(day.Pix, scene.Background.Pix)) {
		t.Fatal("daytime render changed")
	}
	t.Log(
		"replaced independent dark palette; pixels, map and daytime unchanged",
	)
}

func editWorldMapRoadTileset(t *testing.T, s *game.Session) {
	if !s.WorldMap.Available() {
		t.Skip("no native world map")
	}
	source, err := s.WorldMap.RoadTileset()
	if err != nil {
		t.Skipf("road tiles unavailable: %v", err)
	}
	content, err := source.PNG()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	bitmap, ok := decoded.(*image.Paletted)
	if !ok {
		t.Fatal("indexed export")
	}
	bitmap.SetColorIndex(0, 0, bitmap.ColorIndexAt(0, 0)^1)
	pixel := color.NRGBAModel.Convert(bitmap.Palette[1]).(color.NRGBA)
	pixel.R = 255 - pixel.R
	bitmap.Palette[1] = pixel
	var replacement bytes.Buffer
	if err = png.Encode(&replacement, bitmap); err != nil {
		t.Fatal(err)
	}
	before := s.Document.Snapshot()
	routes, routeErr := s.WorldMap.Routes()
	draft, err := source.ImportPNG(replacement.Bytes())
	checkPlan(t, s, before, err)
	plan, err := s.WorldMap.PlanRoadTileset(draft)
	checkPlan(t, s, before, err)
	if err = s.WorldMap.ApplyRoadTileset(plan); err != nil {
		t.Fatal(err)
	}
	after := s.Document.Snapshot()
	var sites []int
	for at := range len(before.Data) {
		if before.Data[at] != after.Data[at] &&
			(len(sites) == 0 || sites[len(sites)-1] != at&^3) {
			sites = append(sites, at&^3)
		}
	}
	if len(sites) < 2 || len(sites) > 3 {
		t.Fatal("unexpected graphics pointers", sites)
	}
	for _, site := range sites {
		old, valid := gba.Pointer(before.Data, site)
		at, ok := gba.Pointer(after.Data, site)
		if !valid || old < 0xC0 || !ok || at < plan.Appended().Start ||
			at >= plan.Appended().End {
			t.Fatal("unrelated bytes changed")
		}
	}
	reopened := historyAndReopen(t, s, before)
	fresh, err := reopened.WorldMap.RoadTileset()
	if err != nil {
		t.Fatal(err)
	}
	saved, err := fresh.PNG()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved, replacement.Bytes()) ||
		fresh.TileCount() != source.TileCount() {
		t.Fatal("saved tile sheet or palette differs")
	}
	if routeErr == nil {
		current, err := reopened.WorldMap.Routes()
		if err != nil {
			t.Fatal(err)
		}
		for i, route := range current {
			if route.Road != routes[i].Road || route.Nodes != routes[i].Nodes ||
				route.Movement != routes[i].Movement {
				t.Fatal("route records changed")
			}
		}
	}
	t.Logf(
		"tiles=%d pointer consumers=%d; original route layouts retained",
		source.TileCount(),
		len(sites),
	)
}
