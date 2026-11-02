//go:build integration

package integration

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

func editWorldMapBackground(t *testing.T, s *game.Session) {
	if !s.WorldMap.Available() {
		t.Skip("no native world-map background")
	}
	source, err := s.WorldMap.Background()
	if err != nil {
		t.Skipf("background unavailable: %v", err)
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
		t.Fatal("background export lost palette indices")
	}
	index := bitmap.ColorIndexAt(0, 0) ^ 1
	bitmap.SetColorIndex(0, 0, index)
	var replacement bytes.Buffer
	if err = png.Encode(&replacement, bitmap); err != nil {
		t.Fatal(err)
	}
	before := s.Document.Snapshot()
	draft, err := source.ImportPNG(replacement.Bytes())
	checkPlan(t, s, before, err)
	plan, err := s.WorldMap.PlanBackground(draft)
	checkPlan(t, s, before, err)
	if err = s.WorldMap.ApplyBackground(plan); err != nil {
		t.Fatal(err)
	}
	after := s.Document.Snapshot()
	// A pixel-only import changes exactly one aligned pointer in the original
	// file. Discover it from the edit; no ROM-specific address is assumed.
	site := -1
	for at := 0; at < len(before.Data); at++ {
		if before.Data[at] == after.Data[at] {
			continue
		}
		if site < 0 {
			site = at &^ 3
		}
		if at < site || at >= site+4 {
			t.Fatal("pixel import changed independent resources")
		}
	}
	if site < 0 {
		t.Fatal("pixel replacement did not update its entry")
	}
	oldPixels, ok := gba.Pointer(before.Data, site)
	if !ok ||
		!gba.Within(
			before.Data,
			oldPixels,
			worldmap.MapWidth*worldmap.MapHeight/2,
		) {
		t.Fatal("old image boundary")
	}
	newPixels, ok := gba.Pointer(after.Data, site)
	if !ok || newPixels != plan.Appended().Start ||
		binary.LittleEndian.Uint32(
			after.Data[site:],
		) != uint32(
			newPixels,
		)+0x08000000 {
		t.Fatal("appended image pointer")
	}
	want := bytes.Clone(
		before.Data[oldPixels : oldPixels+worldmap.MapWidth*worldmap.MapHeight/2],
	)
	want[0] = want[0]&0xF0 | index&15
	if plan.Appended().End != newPixels+len(want) ||
		!bytes.Equal(want, after.Data[newPixels:]) {
		t.Fatal("native pixel replacement differs")
	}
	reopened := historyAndReopen(t, s, before)
	fresh, err := reopened.WorldMap.Background()
	if err != nil {
		t.Fatal(err)
	}
	saved, err := fresh.PNG()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved, replacement.Bytes()) {
		t.Fatal("saved indexed PNG differs")
	}
	scene := reopened.WorldMap.Scene(0)
	if scene.BackgroundErr != nil ||
		scene.Background.RGBAAt(0, 0) != draft.Image().RGBAAt(0, 0) {
		t.Fatal("saved render differs", scene.BackgroundErr)
	}
	// Change one used color in the reopened document. All redirected native
	// daytime consumers must share a single new palette; pixels stay in place.
	pixel := color.NRGBAModel.Convert(bitmap.Palette[index]).(color.NRGBA)
	pixel.R = 255 - pixel.R
	bitmap.Palette[index] = pixel
	replacement.Reset()
	if err = png.Encode(&replacement, bitmap); err != nil {
		t.Fatal(err)
	}
	before = reopened.Document.Snapshot()
	draft, err = fresh.ImportPNG(replacement.Bytes())
	checkPlan(t, reopened, before, err)
	plan, err = reopened.WorldMap.PlanBackground(draft)
	checkPlan(t, reopened, before, err)
	if err = reopened.WorldMap.ApplyBackground(plan); err != nil {
		t.Fatal(err)
	}
	after = reopened.Document.Snapshot()
	var sites []int
	for at := range len(before.Data) {
		if before.Data[at] != after.Data[at] &&
			(len(sites) == 0 || sites[len(sites)-1] != at&^3) {
			sites = append(sites, at&^3)
		}
	}
	if len(sites) < 1 || len(sites) > 3 {
		t.Fatal("unexpected daytime palette consumers", sites)
	}
	oldPalette, ok := gba.Pointer(before.Data, sites[0])
	if !ok || !gba.Within(before.Data, oldPalette, 128) {
		t.Fatal("old palette bounds")
	}
	for _, site := range sites {
		old, valid := gba.Pointer(before.Data, site)
		current, updated := gba.Pointer(after.Data, site)
		if !valid || !updated || old != oldPalette ||
			current != plan.Appended().Start {
			t.Fatal("palette consumers diverged")
		}
	}
	want = bytes.Clone(before.Data[oldPalette : oldPalette+128])
	want[int(index)*2] ^= 31
	if !bytes.Equal(want, after.Data[plan.Appended().Start:]) {
		t.Fatal("palette bits or other resource changed")
	}
	reopened = historyAndReopen(t, reopened, before)
	scene = reopened.WorldMap.Scene(0)
	if scene.BackgroundErr != nil ||
		scene.Background.RGBAAt(0, 0) != draft.Image().RGBAAt(0, 0) {
		t.Fatal("saved palette render differs", scene.BackgroundErr)
	}
	t.Logf(
		"background=%dx%d pixel and palette replacements; daytime consumers=%d",
		worldmap.MapWidth,
		worldmap.MapHeight,
		len(sites),
	)
}
