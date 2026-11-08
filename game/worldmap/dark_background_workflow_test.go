package worldmap_test

import (
	"bytes"
	"errors"
	"image/color"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

func TestDarkWorldMapPaletteAndHistory(t *testing.T) {
	for _, version := range []string{"FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewWorldMapBackgroundFixture(version)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			source, err := s.WorldMap.Background(worldmap.Darkened)
			if err != nil {
				t.Fatal(err)
			}
			bitmap := exportedBackground(t, source)
			bitmap.Palette[1] = color.RGBA{B: 255, A: 255}
			draft, err := source.ImportPNG(backgroundPNG(t, bitmap))
			if err != nil {
				t.Fatal(err)
			}
			if draft.Image().RGBAAt(0, 0) != (color.RGBA{B: 255, A: 255}) {
				t.Fatal("dark preview")
			}
			plan, err := s.WorldMap.PlanBackground(draft)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("preview wrote ROM")
			}
			if err = s.WorldMap.ApplyBackground(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			site := f.GetPosition("worldmap_palette_dark_pointer")
			want := bytes.Clone(before.Data)
			copy(want[site:site+4], after.Data[site:site+4])
			if !bytes.Equal(want, after.Data[:len(want)]) {
				t.Fatal(
					"daytime consumers, pixels, banks or independent aliases changed",
				)
			}
			palette, ok := gba.Pointer(after.Data, site)
			if !ok || palette != len(before.Data) ||
				len(after.Data) != palette+128 {
				t.Fatal("dark allocation")
			}
			original := f.GetPosition("worldmap_palette")
			colors := bytes.Clone(before.Data[original : original+128])
			colors[2], colors[3] = 0, 0xFC
			if !bytes.Equal(colors, after.Data[palette:]) {
				t.Fatal("dark color or reserved bits")
			}
			scene := s.WorldMap.Scene(0)
			if scene.DarkBackgroundErr != nil || scene.BackgroundErr != nil ||
				scene.DarkBackground.RGBAAt(
					0,
					0,
				) != (color.RGBA{B: 255, A: 255}) ||
				scene.Background.RGBAAt(0, 0) != (color.RGBA{R: 8, A: 255}) {
				t.Fatal("palette isolation", scene)
			}
			if !s.Document.Undo() || s.Document.State().CanUndo ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("atomic undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("redo")
			}
			if !errors.Is(s.WorldMap.ApplyBackground(plan), rom.ErrStale) {
				t.Fatal("stale dark plan")
			}
			if err = s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			if reopened.WorldMap.Scene(
				0,
			).DarkBackground.RGBAAt(
				0,
				0,
			) != (color.RGBA{B: 255, A: 255}) {
				t.Fatal("saved dark palette")
			}
		})
	}
}

func TestDarkWorldMapPaletteFailures(t *testing.T) {
	for _, scenario := range []string{"pixel-index", "bank", "overlap", "capacity", "bad-dark", "bad-day"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewWorldMapBackgroundFixture("FE8J")
			if scenario == "bad-dark" {
				f.WriteU32(0xC4594, 0)
			}
			if scenario == "bad-day" {
				f.WriteU32(0xBF6A4, 0)
			}
			if scenario == "capacity" {
				full := testutil.NewFixture("BE8J01", rom.MaxSize-64)
				full.WriteAt(0, f.Bytes())
				f = full
			}
			s := testutil.Open(t, f)
			if scenario == "overlap" {
				if err := s.Records.UseLayout(
					records.Units,
					records.Layout{Address: 0xC4594, Stride: 52, Count: 1},
				); err != nil {
					t.Fatal(err)
				}
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			source, err := s.WorldMap.Background(worldmap.Darkened)
			if scenario == "bad-dark" {
				if !errors.Is(err, worldmap.ErrScene) {
					t.Fatal(err)
				}
				if _, err = s.WorldMap.Background(
					worldmap.Daytime,
				); err != nil {
					t.Fatal("dark fault blocked daytime", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			bitmap := exportedBackground(t, source)
			bitmap.Palette[1] = color.RGBA{B: 255, A: 255}
			if scenario == "pixel-index" {
				bitmap.SetColorIndex(0, 0, 2)
			}
			if scenario == "bank" {
				for y := range 8 {
					for x := range 8 {
						bitmap.SetColorIndex(x, y, 17)
					}
				}
			}
			draft, err := source.ImportPNG(backgroundPNG(t, bitmap))
			if scenario == "pixel-index" || scenario == "bank" {
				if !errors.Is(err, worldmap.ErrDarkBackgroundPixels) {
					t.Fatal(err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				plan, err := s.WorldMap.PlanBackground(draft)
				switch scenario {
				case "overlap":
					if !errors.Is(err, romwrite.ErrClaim) {
						t.Fatal(err)
					}
				case "capacity":
					if !errors.Is(err, romwrite.ErrCapacity) {
						t.Fatal(err)
					}
				case "bad-day":
					if err != nil {
						t.Fatal("day fault blocked dark edit", err)
					}
					if err = s.WorldMap.ApplyBackground(plan); err != nil {
						t.Fatal(err)
					}
					return
				}
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("failed dark edit changed document/history")
			}
		})
	}
}
