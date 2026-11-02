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
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func backgroundPNG(t *testing.T, bitmap image.Image) []byte {
	t.Helper()
	var content bytes.Buffer
	if err := png.Encode(&content, bitmap); err != nil {
		t.Fatal(err)
	}
	return content.Bytes()
}

func exportedBackground(
	t *testing.T,
	source *worldmap.Background,
) *image.Paletted {
	t.Helper()
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
		t.Fatal("export lost palette indices")
	}
	return indexed
}

func TestWorldMapBackgroundReplacementAndHistory(t *testing.T) {
	for _, version := range []string{"FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewWorldMapBackgroundFixture(version)
			s := testutil.Open(t, f)
			before, state := s.Document.Snapshot(), s.Document.State()
			source, err := s.WorldMap.Background()
			if err != nil {
				t.Fatal(err)
			}
			bitmap := exportedBackground(t, source)
			if len(bitmap.Palette) != 64 || bitmap.ColorIndexAt(0, 0) != 1 ||
				bitmap.ColorIndexAt(8, 0) != 17 ||
				bitmap.ColorIndexAt(0, 8) != 33 ||
				bitmap.ColorIndexAt(0, 312) != 49 ||
				bitmap.ColorIndexAt(479, 319) != 1 {
				t.Fatal("native bank selection lost")
			}
			if got := color.RGBAModel.Convert(
				bitmap.Palette[1],
			); got != (color.RGBA{R: 8, A: 255}) {
				t.Fatal("fixed native color", got)
			}
			if _, err = source.ImportPNG(
				backgroundPNG(t, bitmap),
			); !errors.Is(
				err,
				edit.ErrValue,
			) {
				t.Fatal("unchanged PNG was rewritten", err)
			}
			for y := range 8 {
				for x := range 8 {
					bitmap.SetColorIndex(x, y, 51)
				}
			}
			bitmap.Palette[51] = color.RGBA{R: 250, G: 130, B: 1, A: 255}
			content := backgroundPNG(t, bitmap)
			draft, err := source.ImportPNG(content)
			if err != nil {
				t.Fatal(err)
			}
			clear(content)
			preview := draft.Image()
			if preview.RGBAAt(0, 0) != (color.RGBA{R: 247, G: 132, A: 255}) {
				t.Fatal("preview did not use GBA colors", preview.RGBAAt(0, 0))
			}
			clear(preview.Pix)
			plan, err := s.WorldMap.PlanBackground(draft)
			if err != nil {
				t.Fatal(err)
			}
			if !source.Current(s.WorldMap) || !draft.Current(s.WorldMap) ||
				!plan.Current(s.WorldMap) ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("preview changed document/history")
			}
			if err = s.WorldMap.ApplyBackground(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			want := bytes.Clone(before.Data)
			for _, name := range []string{"worldmap_image", "worldmap_palette", "worldmap_palette_map", "worldmap_palette_reload", "worldmap_palette_fade"} {
				site := f.GetPosition(name + "_pointer")
				copy(want[site:site+4], after.Data[site:site+4])
				at, ok := gba.Pointer(after.Data, site)
				if !ok || at < len(before.Data) {
					t.Fatal("resource was not appended", name)
				}
			}
			if !bytes.Equal(want, after.Data[:len(want)]) {
				t.Fatal(
					"old resources, independent references, routes or unknown bytes changed",
				)
			}
			pixels, _ := gba.Pointer(
				after.Data,
				f.GetPosition("worldmap_image_pointer"),
			)
			wantPixels := bytes.Repeat([]byte{0x11}, 480*320/2)
			copy(wantPixels, bytes.Repeat([]byte{0x33}, 32))
			if !bytes.Equal(
				wantPixels,
				after.Data[pixels:pixels+len(wantPixels)],
			) {
				t.Fatal("incorrect 4bpp tile packing")
			}
			palette, _ := gba.Pointer(
				after.Data,
				f.GetPosition("worldmap_palette_pointer"),
			)
			for _, name := range []string{"worldmap_palette_reload", "worldmap_palette_fade"} {
				at, _ := gba.Pointer(after.Data, f.GetPosition(name+"_pointer"))
				if at != palette {
					t.Fatal("confirmed palette consumer lost", name)
				}
			}
			oldPalette := f.GetPosition("worldmap_palette")
			wantPalette := bytes.Clone(before.Data[oldPalette : oldPalette+128])
			copy(wantPalette[102:], []byte{0x1E, 0x82})
			if !bytes.Equal(wantPalette, after.Data[palette:palette+128]) {
				t.Fatal("incorrect color or unused high bits")
			}
			banks, _ := gba.Pointer(
				after.Data,
				f.GetPosition("worldmap_palette_map_pointer"),
			)
			decoded, _, err := gba.DecompressLZ77Stream(
				after.Data[banks:],
				1280,
			)
			if err != nil {
				t.Fatal(err)
			}
			wantBanks := make([]byte, 1280)
			wantBanks[0], wantBanks[32], wantBanks[39*32] = 0x13, 2, 3
			for y := range 40 {
				wantBanks[y*32+30], wantBanks[y*32+31] = 0xA5, 0xD7
			}
			if !bytes.Equal(decoded, wantBanks) {
				t.Fatal("bank nibble or row padding changed")
			}
			if plan.Appended().Start != len(before.Data) ||
				plan.Appended().End != len(after.Data) {
				t.Fatal("allocation preview")
			}
			if !s.Document.Undo() || s.Document.State().CanUndo ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("undo was not atomic")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("redo")
			}
			if !errors.Is(s.WorldMap.ApplyBackground(plan), rom.ErrStale) {
				t.Fatal("replayed stale plan")
			}
			if err = s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			scene := reopened.WorldMap.Scene(0)
			if scene.BackgroundErr != nil ||
				scene.Background.RGBAAt(
					0,
					0,
				) != (color.RGBA{R: 247, G: 132, A: 255}) ||
				scene.RoadErr != nil {
				t.Fatal("saved scene", scene.BackgroundErr, scene.RoadErr)
			}
			fresh, err := reopened.WorldMap.Background()
			if err != nil {
				t.Fatal(err)
			}
			if exportedBackground(t, fresh).ColorIndexAt(0, 0) != 51 {
				t.Fatal("saved bank selection")
			}
		})
	}
}

func TestWorldMapBackgroundPartialReplacement(t *testing.T) {
	for _, changed := range []string{"worldmap_image", "worldmap_palette", "worldmap_palette_map"} {
		t.Run(changed, func(t *testing.T) {
			f := testutil.NewWorldMapBackgroundFixture("FE8U")
			// Unreadable roads do not block an independent background edit.
			f.WriteU32(0xB8F98, 0)
			s := testutil.Open(t, f)
			source, err := s.WorldMap.Background()
			if err != nil {
				t.Fatal(err)
			}
			bitmap := exportedBackground(t, source)
			switch changed {
			case "worldmap_image":
				bitmap.SetColorIndex(0, 0, 0)
			case "worldmap_palette":
				bitmap.Palette[1] = color.RGBA{B: 255, A: 255}
			case "worldmap_palette_map":
				for y := range 8 {
					for x := range 8 {
						bitmap.SetColorIndex(x, y, 49)
					}
				}
			}
			draft, err := source.ImportPNG(backgroundPNG(t, bitmap))
			if err != nil {
				t.Fatal(err)
			}
			plan, err := s.WorldMap.PlanBackground(draft)
			if err != nil {
				t.Fatal(err)
			}
			before := s.Document.Snapshot()
			if err = s.WorldMap.ApplyBackground(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			want := bytes.Clone(before.Data)
			names := []string{changed}
			if changed == "worldmap_palette" {
				names = append(
					names,
					"worldmap_palette_reload",
					"worldmap_palette_fade",
				)
			}
			for _, name := range names {
				site := f.GetPosition(name + "_pointer")
				copy(want[site:site+4], after.Data[site:site+4])
			}
			if !bytes.Equal(want, after.Data[:len(want)]) {
				t.Fatal("unchanged resource or compressed stream was rewritten")
			}
			if changed == "worldmap_image" &&
				draft.Image().RGBAAt(0, 0).A != 255 {
				t.Fatal("color zero became transparent")
			}
		})
	}
}

func TestWorldMapBackgroundIndependentPaletteConsumers(t *testing.T) {
	for _, independent := range []string{"worldmap_palette_reload", "worldmap_palette_fade"} {
		t.Run(independent, func(t *testing.T) {
			f := testutil.NewWorldMapBackgroundFixture("FE8U")
			f.WritePointer(f.GetPosition(independent+"_pointer"), 0x279000)
			s := testutil.Open(t, f)
			source, err := s.WorldMap.Background()
			if err != nil {
				t.Fatal(err)
			}
			bitmap := exportedBackground(t, source)
			bitmap.Palette[1] = color.RGBA{B: 255, A: 255}
			draft, err := source.ImportPNG(backgroundPNG(t, bitmap))
			if err != nil {
				t.Fatal(err)
			}
			plan, err := s.WorldMap.PlanBackground(draft)
			if err != nil {
				t.Fatal(err)
			}
			before := s.Document.Snapshot()
			if err = s.WorldMap.ApplyBackground(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			want := bytes.Clone(before.Data)
			for _, name := range []string{"worldmap_palette", "worldmap_palette_reload", "worldmap_palette_fade"} {
				if name == independent {
					continue
				}
				site := f.GetPosition(name + "_pointer")
				copy(want[site:site+4], after.Data[site:site+4])
				at, _ := gba.Pointer(after.Data, site)
				if at != plan.Appended().Start {
					t.Fatal("shared consumer not redirected", name)
				}
			}
			if !bytes.Equal(want, after.Data[:len(want)]) ||
				plan.Appended().End-plan.Appended().Start != 128 {
				t.Fatal("independent consumer or dark palette changed")
			}
		})
	}
}

func TestWorldMapBackgroundImportDuringDocumentEdit(t *testing.T) {
	s := testutil.Open(t, testutil.NewWorldMapBackgroundFixture("FE8U"))
	source, err := s.WorldMap.Background()
	if err != nil {
		t.Fatal(err)
	}
	bitmap := exportedBackground(t, source)
	bitmap.SetColorIndex(0, 0, 2)
	content := backgroundPNG(t, bitmap)
	type result struct {
		draft *worldmap.BackgroundDraft
		err   error
	}
	finished := make(chan result, 1)
	go func() {
		draft, err := source.ImportPNG(content)
		finished <- result{draft, err}
	}()
	if err = s.Records.Set(records.Units, []int{1}, "level", 9); err != nil {
		t.Fatal(err)
	}
	imported := <-finished
	if imported.err != nil {
		t.Fatal(imported.err)
	}
	before, state := s.Document.Snapshot(), s.Document.State()
	if _, err = s.WorldMap.PlanBackground(
		imported.draft,
	); !errors.Is(
		err,
		rom.ErrStale,
	) {
		t.Fatal("late import was not rejected", err)
	}
	if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
		state != s.Document.State() {
		t.Fatal("late import changed history")
	}
	if exportedBackground(t, source).ColorIndexAt(0, 0) != 1 {
		t.Fatal("source snapshot changed")
	}
}

func TestWorldMapBackgroundRejectedImports(t *testing.T) {
	s := testutil.Open(t, testutil.NewWorldMapBackgroundFixture("FE8J"))
	source, err := s.WorldMap.Background()
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"mixed-banks", "high-index", "transparent", "semi-transparent", "size", "true-color", "truncated", "file-size"} {
		t.Run(scenario, func(t *testing.T) {
			bitmap := exportedBackground(t, source)
			var content []byte
			want := error(worldmap.ErrBackgroundColors)
			switch scenario {
			case "mixed-banks":
				bitmap.SetColorIndex(1, 0, 17)
			case "high-index":
				bitmap.Palette = append(bitmap.Palette, color.Black)
				bitmap.SetColorIndex(0, 0, 64)
			case "transparent":
				bitmap.Palette[0] = color.Transparent
			case "semi-transparent":
				bitmap.Palette[0] = color.NRGBA{R: 255, A: 128}
			case "size":
				bitmap.Rect.Max.X--
				want = worldmap.ErrBackgroundSize
			case "true-color":
				content = backgroundPNG(
					t,
					image.NewRGBA(image.Rect(0, 0, 480, 320)),
				)
				want = worldmap.ErrBackgroundPNG
			case "truncated":
				content = []byte("not a PNG")
				want = worldmap.ErrBackgroundPNG
			case "file-size":
				content = make([]byte, worldmap.MaxBackgroundPNG+1)
				want = worldmap.ErrBackgroundSize
			}
			if content == nil {
				content = backgroundPNG(t, bitmap)
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			if _, err := source.ImportPNG(content); !errors.Is(err, want) {
				t.Fatal(scenario, err, want)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("rejection changed document/history")
			}
		})
	}
}

func TestWorldMapBackgroundPlanFailures(t *testing.T) {
	for _, scenario := range []string{"revision", "layout", "document", "overlap", "palette-consumer-overlap", "capacity", "combined-capacity", "malformed", "source-overlap"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewWorldMapBackgroundFixture("FE8J")
			if scenario == "capacity" || scenario == "combined-capacity" {
				remaining := 32
				if scenario == "combined-capacity" {
					remaining = 480*320/2 + 8
				}
				full := testutil.NewFixture("BE8J01", rom.MaxSize-remaining)
				full.WriteAt(0, f.Bytes())
				f = full
			}
			if scenario == "malformed" {
				f.WriteU32(0xBF69C, 0)
			}
			// The image's own pointer may not overwrite its own payload either.
			if scenario == "source-overlap" {
				f.WritePointer(0xBF698, 0xBF698)
			}
			s := testutil.Open(t, f)
			if scenario == "overlap" || scenario == "palette-consumer-overlap" {
				at := 0xBF698
				if scenario == "palette-consumer-overlap" {
					at = 0xC4590
				}
				if err := s.Records.UseLayout(
					records.Units,
					records.Layout{Address: at, Stride: 52, Count: 1},
				); err != nil {
					t.Fatal(err)
				}
			}
			source, err := s.WorldMap.Background()
			if scenario == "malformed" {
				if !errors.Is(err, worldmap.ErrScene) {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			bitmap := exportedBackground(t, source)
			bitmap.SetColorIndex(0, 0, (bitmap.ColorIndexAt(0, 0)&0x30)|2)
			if scenario == "combined-capacity" ||
				scenario == "palette-consumer-overlap" {
				bitmap.Palette[2] = color.RGBA{B: 255, A: 255}
			}
			draft, err := source.ImportPNG(backgroundPNG(t, bitmap))
			if err != nil {
				t.Fatal(err)
			}
			var plan *worldmap.BackgroundPlan
			want := error(rom.ErrStale)
			switch scenario {
			case "overlap", "source-overlap", "palette-consumer-overlap":
				want = space.ErrClaim
			case "capacity", "combined-capacity":
				want = space.ErrCapacity
			default:
				plan, err = s.WorldMap.PlanBackground(draft)
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
				err = s.WorldMap.ApplyBackground(plan)
			} else {
				_, err = s.WorldMap.PlanBackground(draft)
			}
			if !errors.Is(err, want) {
				t.Fatal("unexpected failure", err, want)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("failure changed ROM/history")
			}
			if plan != nil {
				if _, err = s.WorldMap.PlanBackground(
					draft,
				); !errors.Is(
					err,
					rom.ErrStale,
				) {
					t.Fatal("stale import planned", err)
				}
			}
		})
	}
}
