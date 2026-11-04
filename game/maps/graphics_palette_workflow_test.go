package maps_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image/color"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestMapGraphicsAndPaletteJointImport(t *testing.T) {
	for _, version := range testutil.Games() {
		for _, slot := range []maps.GraphicsSlot{maps.FirstGraphics, maps.SecondGraphics} {
			for _, bank := range []int{2, 7} {
				t.Run(
					fmt.Sprintf("%s/%d/%d", version.Name, slot, bank),
					func(t *testing.T) {
						f := testutil.NewMapFixture(version.Name)
						stride := 148
						if version.Name == "FE6" {
							stride = 68
						}
						if version.Name == "FE7U" {
							stride = 152
						}
						chapter := f.GetPosition("chapters")
						graphicsList, paletteList := f.GetPosition(
							"map_graphics_list",
						), f.GetPosition(
							"map_palette_list",
						)
						for index := 1; index < 4; index++ {
							f.WriteAt(
								chapter+index*stride,
								f.Bytes()[chapter:chapter+stride],
							)
						}
						// The image-only and palette-only consumers are different chapters.
						f.WriteU8(chapter+stride+6, 7)
						f.WriteU8(chapter+2*stride+4, 5)
						f.WriteU8(chapter+3*stride+6, 9)
						id, sourceAt, count := 1, 0x230000, 2
						if slot == maps.SecondGraphics {
							id, sourceAt, count = 8, 0x230200, 1
							f.WriteU8(chapter+5, 8)
							f.WriteU8(chapter+stride+5, 8)
						}
						f.WriteU8(chapter+3*stride+4+int(slot), 9)
						f.WritePointer(graphicsList+9*4, sourceAt)
						paletteAt := f.GetPosition("map_palette")
						f.WritePointer(paletteList+9*4, paletteAt)
						f.WriteU16(paletteAt+bank*32+4, 0x83E0)
						config := make([]byte, 0x2400)
						word := uint16(0x2000)
						if slot == maps.SecondGraphics {
							word = 0x2200
						}
						for index := range 4 {
							binary.LittleEndian.PutUint16(
								config[index*2:],
								word,
							)
						}
						f.WriteAt(
							f.GetPosition("map_config"),
							literalMapLZ(config),
						)
						s := testutil.Open(t, f)
						before := s.Document.Snapshot()
						source, err := s.Maps.Graphics(0, slot)
						if err != nil {
							t.Fatal(err)
						}
						bitmap := exportedGraphics(t, source, bank)
						bitmap.SetColorIndex(0, 0, 2)
						bitmap.SetColorIndex(1, 0, 3)
						bitmap.Palette[2] = color.NRGBA{
							R: 9,
							G: 13,
							B: 17,
							A: 255,
						}
						content := graphicsPNG(t, bitmap)
						options := maps.GraphicsImport{
							WithPalette: true,
							Bank:        bank,
						}
						draft, err := source.ImportPNG(content, options)
						if err != nil {
							t.Fatal(err)
						}
						clear(content)
						bitmap.Palette[2] = color.RGBA{A: 255}
						wantColor := color.RGBA{R: 8, G: 16, B: 16, A: 255}
						preview, err := draft.Image(bank)
						if err != nil || preview.At(0, 0) != wantColor {
							t.Fatal("quantized joint preview", err)
						}
						tone := maps.NormalPalette
						if bank == 7 {
							tone = maps.FogPalette
						}
						view := draft.Preview(tone)
						if view.MapErr != nil || view.TilesetErr != nil ||
							view.Map.Image.RGBAAt(0, 0) != wantColor {
							t.Fatal(
								"combined map preview",
								view.MapErr,
								view.TilesetErr,
							)
						}
						plan, err := s.Maps.PlanGraphics(draft)
						if err != nil {
							t.Fatal(err)
						}
						if plan.ImportOptions() != options ||
							len(plan.PaletteChanges()) != 1 ||
							plan.PaletteChanges()[0].Index != bank*16+2 ||
							plan.PaletteChanges()[0].After != wantColor {
							t.Fatal(
								"frozen palette changes",
								plan.PaletteChanges(),
							)
						}
						for _, use := range plan.Uses() {
							if use.Updated != (use.Chapter == 0 || use.Chapter == 1) {
								t.Fatal("graphics sharing", plan.Uses())
							}
						}
						for _, use := range plan.PaletteUses() {
							if use.Updated != (use.Chapter == 0 || use.Chapter == 2) {
								t.Fatal("palette sharing", plan.PaletteUses())
							}
						}
						if len(plan.Uses()) != 3 ||
							len(plan.PaletteUses()) != 3 {
							t.Fatal("missing consumers")
						}
						colors, uses := plan.PaletteChanges(), plan.PaletteUses()
						colors[0].Index, uses[0].Updated = -1, false
						if plan.PaletteChanges()[0].Index < 0 ||
							!plan.PaletteUses()[0].Updated {
							t.Fatal("mutable plan")
						}
						if !bytes.Equal(
							before.Data,
							s.Document.Snapshot().Data,
						) {
							t.Fatal("preview wrote ROM")
						}
						if err := s.Maps.ApplyGraphics(plan); err != nil {
							t.Fatal(err)
						}
						after := s.Document.Snapshot()
						graphicsSite, paletteSite := graphicsList+id*4, paletteList+2*4
						graphicsAt, _ := gba.Pointer(after.Data, graphicsSite)
						newPaletteAt, _ := gba.Pointer(after.Data, paletteSite)
						pixels, err := gba.DecompressLZ77(
							after.Data[graphicsAt:],
							0x8000,
						)
						wantPixels := bytes.Repeat([]byte{0x55}, count*32)
						if slot == maps.FirstGraphics {
							copy(wantPixels, bytes.Repeat([]byte{0x11}, 32))
							wantPixels[3], wantPixels[28], wantPixels[31] = 0x21, 0x13, 0x41
						}
						wantPixels[0] = 0x32
						wantPalette := bytes.Clone(
							before.Data[paletteAt : paletteAt+320],
						)
						wantPalette[bank*32+4], wantPalette[bank*32+5] = 0x41, 0x88
						if err != nil || !bytes.Equal(pixels, wantPixels) ||
							!bytes.Equal(
								after.Data[newPaletteAt:newPaletteAt+320],
								wantPalette,
							) {
							t.Fatal("fixed bytes/high bit", err)
						}
						preserved := bytes.Clone(after.Data[:len(before.Data)])
						for _, site := range []int{graphicsSite, paletteSite} {
							copy(
								preserved[site:site+4],
								before.Data[site:site+4],
							)
						}
						if !bytes.Equal(preserved, before.Data) {
							t.Fatal(
								"unselected resources or unknown bytes changed",
							)
						}
						if !s.Document.Undo() || s.Document.State().CanUndo ||
							!bytes.Equal(
								before.Data,
								s.Document.Snapshot().Data,
							) ||
							!s.Document.Redo() ||
							!bytes.Equal(
								after.Data,
								s.Document.Snapshot().Data,
							) {
							t.Fatal("joint history")
						}
						if err := s.Document.Save(); err != nil {
							t.Fatal(err)
						}
						reopened, err := game.Open(s.Document.State().Path)
						if err != nil {
							t.Fatal(err)
						}
						loaded, err := reopened.Maps.Graphics(0, slot)
						if err != nil {
							t.Fatal(err)
						}
						if exportedGraphics(
							t,
							loaded,
							bank,
						).At(0, 0) !=
							wantColor {
							t.Fatal("save/reopen")
						}
					},
				)
			}
		}
	}
}

func TestMapGraphicsJointImportPartialChanges(t *testing.T) {
	for _, mode := range []string{"pixels", "colors", "none", "quantized-none"} {
		t.Run(mode, func(t *testing.T) {
			f := testutil.NewMapFixture("FE8U")
			s := testutil.Open(t, f)
			source, err := s.Maps.Graphics(0, maps.FirstGraphics)
			if err != nil {
				t.Fatal(err)
			}
			bitmap := exportedGraphics(t, source, 0)
			switch mode {
			case "pixels":
				bitmap.SetColorIndex(0, 0, 2)
			case "colors":
				bitmap.Palette[1] = color.RGBA{B: 255, A: 255}
			case "quantized-none":
				bitmap.Palette[1] = color.RGBA{R: 254, A: 255}
			}
			draft, err := source.ImportPNG(
				graphicsPNG(t, bitmap),
				maps.GraphicsImport{WithPalette: true},
			)
			if err != nil {
				t.Fatal(err)
			}
			before := s.Document.Snapshot()
			plan, err := s.Maps.PlanGraphics(draft)
			if mode == "none" || mode == "quantized-none" {
				if !errors.Is(err, edit.ErrValue) {
					t.Fatal("no-op", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Maps.ApplyGraphics(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			site := f.GetPosition("map_graphics_list") + 4
			if mode == "colors" {
				site = f.GetPosition("map_palette_list") + 8
				if len(plan.Uses()) != 0 || len(plan.ChangedTiles()) != 0 ||
					plan.Length() != 320 {
					t.Fatal("unchanged graphics allocated")
				}
			} else if len(plan.PaletteUses()) != 0 || len(plan.PaletteChanges()) != 0 {
				t.Fatal("unchanged palette allocated")
			}
			preserved := bytes.Clone(after.Data[:len(before.Data)])
			copy(preserved[site:site+4], before.Data[site:site+4])
			if !bytes.Equal(preserved, before.Data) {
				t.Fatal("unmodified resource moved")
			}
		})
	}
}

func TestMapGraphicsJointImportRejectsUnsafeWrites(t *testing.T) {
	for _, scenario := range []string{"negative-bank", "wide-bank", "missing-colors", "unused-alpha", "missing-palette", "palette-overlap", "partial-capacity", "revision", "layout", "document"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewMapFixture("FE8U")
			if scenario == "missing-palette" {
				f.WritePointer(f.GetPosition("map_palette_list")+8, 0)
			}
			if scenario == "palette-overlap" {
				at := f.GetPosition("map_palette")
				f.WritePointer(f.GetPosition("map_palette_pointer"), at-8)
				f.WritePointer(at, at)
			}
			s := testutil.Open(t, f)
			if scenario == "partial-capacity" {
				if err := s.Document.ApplyEdit(
					s.Document.State().Revision,
					"grow",
					rom.Edit{Size: rom.MaxSize - 64},
				); err != nil {
					t.Fatal(err)
				}
			}
			source, err := s.Maps.Graphics(0, maps.FirstGraphics)
			if err != nil {
				t.Fatal(err)
			}
			// A valid independent source supplies pixels when this palette is broken.
			valid := source
			if scenario == "missing-palette" {
				validSession := testutil.Open(t, testutil.NewMapFixture("FE8U"))
				valid, err = validSession.Maps.Graphics(0, maps.FirstGraphics)
				if err != nil {
					t.Fatal(err)
				}
			}
			bitmap := exportedGraphics(t, valid, 0)
			bitmap.SetColorIndex(0, 0, 2)
			bitmap.Palette[2] = color.RGBA{B: 255, A: 255}
			options := maps.GraphicsImport{WithPalette: true}
			switch scenario {
			case "negative-bank":
				options.Bank = -1
			case "wide-bank":
				options.Bank = 10
			case "missing-colors":
				bitmap.Palette = bitmap.Palette[:6]
			case "unused-alpha":
				bitmap.Palette[15] = color.RGBA{}
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			draft, err := source.ImportPNG(graphicsPNG(t, bitmap), options)
			if !slices.Contains(
				[]string{
					"negative-bank",
					"wide-bank",
					"missing-colors",
					"unused-alpha",
					"missing-palette",
				},
				scenario,
			) {
				if err != nil {
					t.Fatal(err)
				}
				plan, planErr := s.Maps.PlanGraphics(draft)
				err = planErr
				if slices.Contains(
					[]string{"revision", "layout", "document"},
					scenario,
				) {
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
					before, state = s.Document.Snapshot(), s.Document.State()
					if _, err := s.Maps.PlanGraphics(
						draft,
					); !errors.Is(
						err,
						rom.ErrStale,
					) {
						t.Fatal("stale draft", err)
					}
					err = s.Maps.ApplyGraphics(plan)
				}
			}
			if err == nil {
				t.Fatal("unsafe joint import accepted")
			}
			if scenario == "partial-capacity" &&
				!errors.Is(err, space.ErrCapacity) {
				t.Fatal("capacity", err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("partial write/history")
			}
		})
	}
}
