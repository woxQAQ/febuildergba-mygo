package worldmap_test

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

func exportedRoadTiles(
	t *testing.T,
	source *worldmap.RoadTileset,
) *image.Paletted {
	t.Helper()
	content, err := source.PNG()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	indexed, ok := decoded.(*image.Paletted)
	if !ok {
		t.Fatal("non-indexed export")
	}
	return indexed
}

func TestWorldMapRoadTilesetReplacementAndHistory(t *testing.T) {
	for _, version := range []string{"FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewWorldMapTilesetFixture(version)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			source, err := s.WorldMap.RoadTileset()
			if err != nil {
				t.Fatal(err)
			}
			bitmap := exportedRoadTiles(t, source)
			if source.TileCount() != 2 ||
				bitmap.Bounds() != image.Rect(0, 0, 128, 8) ||
				bitmap.ColorIndexAt(0, 0) != 1 ||
				bitmap.ColorIndexAt(8, 0) != 2 ||
				bitmap.ColorIndexAt(15, 7) != 3 ||
				bitmap.ColorIndexAt(16, 0) != 0 {
				t.Fatal("native sheet layout")
			}
			if _, _, _, a := bitmap.Palette[0].RGBA(); a != 0 {
				t.Fatal("color zero must be transparent")
			}
			if _, err = source.ImportPNG(
				backgroundPNG(t, bitmap),
			); !errors.Is(
				err,
				edit.ErrValue,
			) {
				t.Fatal("no-op import", err)
			}
			bitmap.SetColorIndex(0, 0, 4)
			bitmap.Palette[4] = color.RGBA{R: 250, G: 130, B: 1, A: 255}
			content := backgroundPNG(t, bitmap)
			draft, err := source.ImportPNG(content)
			if err != nil {
				t.Fatal(err)
			}
			clear(content)
			if draft.Image().
				RGBAAt(0, 0) !=
				(color.RGBA{R: 247, G: 132, A: 255}) {
				t.Fatal("GBA preview")
			}
			clear(draft.Image().Pix)
			route, err := s.WorldMap.NewRouteDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			road, err := route.Road()
			if err != nil {
				t.Fatal(err)
			}
			if err = road.Stroke(
				[]image.Point{image.Pt(5, 5)},
				worldmap.RoadCell{Tile: 0, Present: true, FlipX: true},
			); err != nil {
				t.Fatal(err)
			}
			preview, err := draft.Preview(route)
			if err != nil {
				t.Fatal(err)
			}
			if preview.RGBAAt(47, 40) != (color.RGBA{R: 247, G: 132, A: 255}) ||
				!road.CanUndo() {
				t.Fatal("pending route edits or flip lost in preview")
			}
			plan, err := s.WorldMap.PlanRoadTileset(draft)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("preview wrote ROM")
			}
			if err = s.WorldMap.ApplyRoadTileset(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			want := bytes.Clone(before.Data)
			for _, name := range []string{"road_tiles", "road_tiles_reload", "road_palette"} {
				site := f.GetPosition(name + "_pointer")
				copy(want[site:site+4], after.Data[site:site+4])
			}
			if !bytes.Equal(want, after.Data[:len(want)]) {
				t.Fatal(
					"road streams, old graphics, independent aliases or backgrounds changed",
				)
			}
			tiles, ok := gba.Pointer(
				after.Data,
				f.GetPosition("road_tiles_pointer"),
			)
			if !ok || tiles != plan.Appended().Start {
				t.Fatal("tiles allocation")
			}
			reload, _ := gba.Pointer(
				after.Data,
				f.GetPosition("road_tiles_reload_pointer"),
			)
			if reload != tiles {
				t.Fatal("reload consumer lost")
			}
			decoded, _, err := gba.DecompressLZ77Stream(
				after.Data[tiles:],
				0x3000,
			)
			if err != nil {
				t.Fatal(err)
			}
			expected := make([]byte, 64)
			expected[0], expected[32], expected[63] = 4, 2, 0x30
			if !bytes.Equal(expected, decoded) {
				t.Fatal("fixed native tile bytes")
			}
			palette, _ := gba.Pointer(
				after.Data,
				f.GetPosition("road_palette_pointer"),
			)
			oldPalette := f.GetPosition("worldmap_road_palette")
			colors := bytes.Clone(before.Data[oldPalette : oldPalette+32])
			colors[8], colors[9] = 0x1E, 0x82
			if !bytes.Equal(colors, after.Data[palette:palette+32]) {
				t.Fatal("transparent color, palette or unused high bit changed")
			}
			if route.Current(s.WorldMap) {
				t.Fatal("route draft not invalidated")
			}
			if !s.Document.Undo() || s.Document.State().CanUndo ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("atomic undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("redo")
			}
			if !errors.Is(s.WorldMap.ApplyRoadTileset(plan), rom.ErrStale) {
				t.Fatal("stale replay")
			}
			if err = s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			scene := reopened.WorldMap.Scene(0)
			if scene.RoadErr != nil ||
				scene.Road.RGBAAt(
					8,
					8,
				) != (color.RGBA{R: 247, G: 132, A: 255}) ||
				scene.Road.RGBAAt(47, 40).A != 0 {
				t.Fatal("saved road or uncommitted painting", scene.RoadErr)
			}
		})
	}
}

func TestWorldMapRoadTilesetImportFailures(t *testing.T) {
	s := testutil.Open(t, testutil.NewWorldMapTilesetFixture("FE8U"))
	source, err := s.WorldMap.RoadTileset()
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"padding", "high-index", "opaque-zero", "transparent-color", "half-alpha", "size", "true-color", "file-size", "damaged"} {
		t.Run(scenario, func(t *testing.T) {
			bitmap := exportedRoadTiles(t, source)
			var content []byte
			want := error(worldmap.ErrRoadTilesetColors)
			switch scenario {
			case "padding":
				bitmap.SetColorIndex(16, 0, 1)
				want = worldmap.ErrRoadTilesetPadding
			case "high-index":
				bitmap.Palette = append(bitmap.Palette, color.White)
				bitmap.SetColorIndex(0, 0, 16)
			case "opaque-zero":
				bitmap.Palette[0] = color.Black
			case "transparent-color":
				bitmap.Palette[1] = color.Transparent
			case "half-alpha":
				bitmap.Palette[1] = color.NRGBA{R: 255, A: 128}
			case "size":
				bitmap.Rect.Max.X = 120
				want = worldmap.ErrRoadTilesetSize
			case "true-color":
				content = backgroundPNG(t, image.NewRGBA(bitmap.Bounds()))
				want = worldmap.ErrRoadTilesetPNG
			case "file-size":
				content = make([]byte, worldmap.MaxRoadTilesetPNG+1)
				want = worldmap.ErrRoadTilesetSize
			case "damaged":
				content = []byte("bad")
				want = worldmap.ErrRoadTilesetPNG
			}
			if content == nil {
				content = backgroundPNG(t, bitmap)
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			if _, err = source.ImportPNG(content); !errors.Is(err, want) {
				t.Fatal(scenario, err, want)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("bad import changed document")
			}
		})
	}
}

func TestWorldMapRoadTilesetPlanFailures(t *testing.T) {
	for _, scenario := range []string{"revision", "layout", "document", "reload-overlap", "palette-overlap", "capacity", "bad-source"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewWorldMapTilesetFixture("FE8J")
			if scenario == "capacity" {
				full := testutil.NewFixture("BE8J01", rom.MaxSize-24)
				full.WriteAt(0, f.Bytes())
				f = full
			}
			if scenario == "bad-source" {
				f.WriteU32(0xBDE60, 0)
			}
			s := testutil.Open(t, f)
			if scenario == "reload-overlap" || scenario == "palette-overlap" {
				at := 0xC6FF0
				if scenario == "palette-overlap" {
					at = 0xBDE5C
				}
				if err := s.Records.UseLayout(
					records.Units,
					records.Layout{Address: at, Stride: 52, Count: 1},
				); err != nil {
					t.Fatal(err)
				}
			}
			source, err := s.WorldMap.RoadTileset()
			if scenario == "bad-source" {
				if !errors.Is(err, worldmap.ErrScene) {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			bitmap := exportedRoadTiles(t, source)
			bitmap.SetColorIndex(0, 0, 4)
			bitmap.Palette[4] = color.RGBA{B: 255, A: 255}
			draft, err := source.ImportPNG(backgroundPNG(t, bitmap))
			if err != nil {
				t.Fatal(err)
			}
			var plan *worldmap.RoadTilesetPlan
			want := error(rom.ErrStale)
			switch scenario {
			case "reload-overlap", "palette-overlap":
				want = romwrite.ErrClaim
			case "capacity":
				want = romwrite.ErrCapacity
			default:
				plan, err = s.WorldMap.PlanRoadTileset(draft)
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
			if plan == nil {
				_, err = s.WorldMap.PlanRoadTileset(draft)
			} else {
				err = s.WorldMap.ApplyRoadTileset(plan)
			}
			if !errors.Is(err, want) {
				t.Fatal(scenario, err, want)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("failed plan changed document/history")
			}
		})
	}
}

func TestWorldMapRoadTilesetIndependentSources(t *testing.T) {
	for _, paletteOnly := range []bool{false, true} {
		t.Run(
			map[bool]string{false: "tiles", true: "palette"}[paletteOnly],
			func(t *testing.T) {
				f := testutil.NewWorldMapTilesetFixture("FE8U")
				f.WritePointer(0xC2238, 0x277000)
				f.WriteU32(0xBA7FC, 0)
				s := testutil.Open(t, f)
				source, err := s.WorldMap.RoadTileset()
				if err != nil {
					t.Fatal(err)
				}
				bitmap := exportedRoadTiles(t, source)
				changed := "road_tiles"
				if paletteOnly {
					bitmap.Palette[1] = color.RGBA{B: 255, A: 255}
					changed = "road_palette"
				} else {
					bitmap.SetColorIndex(0, 0, 4)
				}
				draft, err := source.ImportPNG(backgroundPNG(t, bitmap))
				if err != nil {
					t.Fatal(err)
				}
				plan, err := s.WorldMap.PlanRoadTileset(draft)
				if err != nil {
					t.Fatal(err)
				}
				before := s.Document.Snapshot()
				if err = s.WorldMap.ApplyRoadTileset(plan); err != nil {
					t.Fatal(err)
				}
				after := s.Document.Snapshot()
				want := bytes.Clone(before.Data)
				site := f.GetPosition(changed + "_pointer")
				copy(want[site:site+4], after.Data[site:site+4])
				if !bytes.Equal(want, after.Data[:len(want)]) {
					t.Fatal("independent reload or unchanged resource modified")
				}
			},
		)
	}
}
