package maps_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
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

func graphicsPNG(t *testing.T, bitmap image.Image) []byte {
	t.Helper()
	var output bytes.Buffer
	if err := png.Encode(&output, bitmap); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func exportedGraphics(
	t *testing.T,
	source *maps.Graphics,
	bank int,
) *image.Paletted {
	t.Helper()
	content, err := source.PNG(bank)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	bitmap, ok := decoded.(*image.Paletted)
	if !ok {
		t.Fatal("export is not indexed")
	}
	return bitmap
}

func TestMapGraphicsPNGEditingAndHistory(t *testing.T) {
	for _, version := range testutil.Games() {
		for _, slot := range []maps.GraphicsSlot{maps.FirstGraphics, maps.SecondGraphics} {
			t.Run(version.Name+"/"+string(rune('0'+slot)), func(t *testing.T) {
				f := testutil.NewMapFixture(version.Name)
				stride := 148
				if version.Name == "FE6" {
					stride = 68
				}
				if version.Name == "FE7U" {
					stride = 152
				}
				chapter, list := f.GetPosition(
					"chapters",
				), f.GetPosition(
					"map_graphics_list",
				)
				f.WriteU8(chapter+5, 8)
				f.WriteU8(chapter+stride+5, 8)
				f.WriteAt(chapter+2*stride, f.Bytes()[chapter:chapter+stride])
				id, sourceAt, count := 1, 0x230000, 2
				if slot == maps.SecondGraphics {
					id, sourceAt, count = 8, 0x230200, 1
				}
				f.WriteU8(chapter+2*stride+4+int(slot), 9)
				f.WritePointer(list+9*4, sourceAt)
				s := testutil.Open(t, f)
				before := s.Document.Snapshot()
				source, err := s.Maps.Graphics(0, slot)
				if err != nil {
					t.Fatal(err)
				}
				if source.Count() != count || source.Slot() != slot ||
					source.PaletteCount() != 10 {
					t.Fatal("source dimensions")
				}
				bitmap := exportedGraphics(t, source, 1)
				if bitmap.Bounds() != image.Rect(0, 0, 128, 8) ||
					bitmap.Palette[1] != (color.RGBA{R: 247, G: 8, A: 255}) ||
					bitmap.Palette[0] != (color.RGBA{G: 255, B: 255, A: 255}) {
					t.Fatal("fixed palette bank or opaque zero", bitmap.Palette)
				}
				want := bytes.Repeat([]byte{0x55}, count*32)
				if slot == maps.FirstGraphics {
					for index := range 32 {
						want[index] = 0x11
					}
					want[0], want[3], want[28], want[31] = 0x01, 0x21, 0x13, 0x41
					if bitmap.ColorIndexAt(0, 0) != 1 ||
						bitmap.ColorIndexAt(1, 0) != 0 ||
						bitmap.ColorIndexAt(7, 0) != 2 ||
						bitmap.ColorIndexAt(0, 7) != 3 ||
						bitmap.ColorIndexAt(7, 7) != 4 ||
						bitmap.ColorIndexAt(8, 0) != 5 {
						t.Fatal("independent source pixels/order")
					}
				} else if bitmap.ColorIndexAt(0, 0) != 5 {
					t.Fatal("second stream did not start at local tile zero")
				}
				if bitmap.ColorIndexAt(count*8, 0) != 0 {
					t.Fatal("last row padding")
				}
				bitmap.SetColorIndex(0, 0, 2)
				bitmap.SetColorIndex(1, 0, 3)
				bitmap.SetColorIndex(count*8-1, 7, 4)
				bitmap.Palette[2] = color.RGBA{R: 9, G: 13, B: 17, A: 255}
				want[0], want[len(want)-1] = 0x32, 0x45
				draft, err := source.ImportPNG(graphicsPNG(t, bitmap))
				if err != nil {
					t.Fatal(err)
				}
				preview, err := draft.Image(0)
				if err != nil ||
					preview.At(0, 0) != (color.RGBA{G: 255, A: 255}) {
					t.Fatal("PNG colors replaced ROM palette", err)
				}
				plan, err := s.Maps.PlanGraphics(draft)
				if err != nil {
					t.Fatal(err)
				}
				if plan.Pointer() != list+id*4 || plan.Source() != sourceAt ||
					len(plan.Uses()) != 3 ||
					!plan.Uses()[0].Updated ||
					!plan.Uses()[1].Updated ||
					plan.Uses()[2].Updated ||
					len(plan.ChangedTiles()) != count {
					t.Fatal(
						"entry sharing/count",
						plan.Uses(),
						plan.ChangedTiles(),
					)
				}
				changed, uses := plan.ChangedTiles(), plan.Uses()
				changed[0], uses[0].Updated = 99, false
				if plan.ChangedTiles()[0] != 0 || !plan.Uses()[0].Updated {
					t.Fatal("mutable plan")
				}
				if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
					t.Fatal("preview wrote ROM")
				}
				if err := s.Maps.ApplyGraphics(plan); err != nil {
					t.Fatal(err)
				}
				after := s.Document.Snapshot()
				at, ok := gba.Pointer(after.Data, plan.Pointer())
				if !ok || at != len(before.Data) || at != plan.Destination() ||
					len(after.Data) != at+plan.Length() {
					t.Fatal("allocation")
				}
				pixels, err := gba.DecompressLZ77(after.Data[at:], 0x8000)
				if err != nil || !bytes.Equal(pixels, want) {
					t.Fatal("exact 4bpp bytes", err, pixels, want)
				}
				preserved := bytes.Clone(after.Data[:len(before.Data)])
				copy(
					preserved[plan.Pointer():plan.Pointer()+4],
					before.Data[plan.Pointer():plan.Pointer()+4],
				)
				if !bytes.Equal(before.Data, preserved) {
					t.Fatal(
						"old streams, palette, definitions, layout or independent entry changed",
					)
				}
				if source.Current(s.Maps) ||
					!errors.Is(s.Maps.ApplyGraphics(plan), rom.ErrStale) {
					t.Fatal("source/plan survived revision")
				}
				if !s.Document.Undo() ||
					!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
					s.Document.State().CanUndo ||
					!s.Document.Redo() ||
					!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
					t.Fatal("atomic history")
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
				chart := exportedGraphics(t, loaded, 0)
				if chart.ColorIndexAt(0, 0) != 2 ||
					chart.ColorIndexAt(1, 0) != 3 ||
					chart.ColorIndexAt(count*8-1, 7) != 4 {
					t.Fatal("save/reopen")
				}
				independent, err := reopened.Maps.Graphics(2, slot)
				if err != nil {
					t.Fatal(err)
				}
				if exportedGraphics(t, independent, 0).ColorIndexAt(0, 0) == 2 {
					t.Fatal("independent alias changed")
				}
			})
		}
	}
}

func TestMapGraphicsSharedLoadingRolesAndOverlay(t *testing.T) {
	for _, scenario := range []string{"cross-chapter", "same-entry", "overlap", "independent-overlap"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewMapFixture("FE8U")
			chapter, list := f.GetPosition(
				"chapters",
			), f.GetPosition(
				"map_graphics_list",
			)
			config := make([]byte, 0x2400)
			for index := range 4 {
				binary.LittleEndian.PutUint16(config[index*2:], 512)
			}
			secondTile := uint16(513)
			if scenario == "overlap" || scenario == "independent-overlap" {
				secondTile = 512
			}
			for index := 4; index < 8; index++ {
				binary.LittleEndian.PutUint16(config[index*2:], secondTile)
			}
			f.WriteAt(0x239000, literalMapLZ(config))
			f.WritePointer(f.GetPosition("map_config_list")+10*4, 0x239000)
			f.WriteU8(chapter+148+4, 5)
			f.WriteU8(chapter+148+5, 1)
			f.WriteU8(chapter+148+7, 10)
			sourceAt := f.GetPosition("map_graphics")
			if scenario != "cross-chapter" {
				f.WriteU8(chapter+5, 1)
				f.WriteU8(chapter+7, 10)
			}
			if scenario == "overlap" || scenario == "independent-overlap" {
				pixels := bytes.Repeat([]byte{0x11}, 513*32)
				f.WriteAt(0x240000, literalMapLZ(pixels))
				f.WritePointer(list+5*4, 0x240000)
				f.WriteU8(chapter+4, 5)
				f.WriteU8(chapter+5, 8)
				if scenario == "independent-overlap" {
					f.WritePointer(list+8*4, 0x230200)
					f.WritePointer(list+9*4, 0x230200)
					f.WriteU8(chapter+5, 9)
					f.WriteU8(chapter+148+5, 8)
				}
			}
			s := testutil.Open(t, f)
			selected, slot := 0, maps.FirstGraphics
			if scenario == "overlap" {
				slot = maps.SecondGraphics
			}
			if scenario == "independent-overlap" {
				selected, slot = 1, maps.SecondGraphics
			}
			source, err := s.Maps.Graphics(selected, slot)
			if err != nil {
				t.Fatal(err)
			}
			bitmap := exportedGraphics(t, source, 0)
			bitmap.SetColorIndex(0, 0, 2)
			draft, err := source.ImportPNG(graphicsPNG(t, bitmap))
			if err != nil {
				t.Fatal(err)
			}
			preview := draft.Preview(maps.NormalPalette)
			if preview.MapErr != nil {
				t.Fatal(preview.MapErr)
			}
			point := image.Pt(0, 0)
			if selected == 1 {
				point.Y = 16
			}
			if preview.Map.Image.RGBAAt(
				point.X,
				point.Y,
			) != (color.RGBA{G: 255, A: 255}) {
				t.Fatal("selected consumer preview")
			}
			plan, err := s.Maps.PlanGraphics(draft)
			if err != nil {
				t.Fatal("shared role mistaken for another resource kind", err)
			}
			if scenario == "cross-chapter" || scenario == "same-entry" {
				want := []maps.GraphicsUse{
					{
						Chapter:    0,
						ResourceID: 1,
						Pointer:    list + 4,
						Slot:       maps.FirstGraphics,
						Updated:    true,
					},
					{
						Chapter:    1,
						ResourceID: 1,
						Pointer:    list + 4,
						Slot:       maps.SecondGraphics,
						Updated:    true,
					},
				}
				if scenario == "same-entry" {
					want = slices.Insert(
						want,
						1,
						maps.GraphicsUse{
							Chapter:    0,
							ResourceID: 1,
							Pointer:    list + 4,
							Slot:       maps.SecondGraphics,
							Updated:    true,
						},
					)
				}
				if !slices.Equal(plan.Uses(), want) ||
					plan.Source() != sourceAt {
					t.Fatal("cross-role sharing", plan.Uses(), want)
				}
			}
			if err := s.Maps.ApplyGraphics(plan); err != nil {
				t.Fatal(err)
			}
			read, err := s.Maps.Read(selected)
			if err != nil ||
				read.Image.RGBAAt(
					point.X,
					point.Y,
				) != (color.RGBA{G: 255, A: 255}) {
				t.Fatal("preview/apply disagreement", err)
			}
			if scenario == "cross-chapter" || scenario == "same-entry" {
				shared, err := s.Maps.Read(1)
				if err != nil ||
					shared.Image.RGBAAt(0, 16) != (color.RGBA{G: 255, A: 255}) {
					t.Fatal("second role did not update", err)
				}
			}
			if scenario == "independent-overlap" {
				independent, err := s.Maps.Read(0)
				if err != nil ||
					independent.Image.RGBAAt(
						0,
						0,
					) != (color.RGBA{R: 255, B: 255, A: 255}) {
					t.Fatal("independent later consumer was replaced", err)
				}
			}
		})
	}
}

func TestMapGraphicsIndependentPreviewResources(t *testing.T) {
	for _, scenario := range []string{"bad-map", "bad-config", "bad-other-graphics", "bad-first-graphics", "bad-palette", "unaligned-palette", "header-palette"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewMapFixture("FE8U")
			slot := maps.FirstGraphics
			switch scenario {
			case "bad-first-graphics":
				slot = maps.SecondGraphics
				f.WriteU8(f.GetPosition("chapters")+5, 8)
				f.WriteU8(f.GetPosition("map_graphics"), 0)
			case "bad-map":
				f.WriteU8(f.GetPosition("map_layout"), 0)
			case "bad-config":
				f.WriteU8(f.GetPosition("map_config"), 0)
			case "bad-other-graphics":
				f.WriteU8(f.GetPosition("chapters")+5, 255)
			case "unaligned-palette":
				f.WritePointer(f.GetPosition("map_palette_list")+8, 0x231001)
			case "header-palette":
				f.WritePointer(f.GetPosition("map_palette_list")+8, 0xA0)
			case "bad-palette":
				f.WritePointer(
					f.GetPosition("map_palette_list")+8,
					len(f.Bytes())-4,
				)
			}
			s := testutil.Open(t, f)
			source, err := s.Maps.Graphics(0, slot)
			if err != nil {
				t.Fatal("independent resource blocked pixels", err)
			}
			colors := make(color.Palette, 16)
			for index := range colors {
				colors[index] = color.RGBA{A: 255}
			}
			bitmap := image.NewPaletted(image.Rect(0, 0, 128, 8), colors)
			bitmap.SetColorIndex(0, 0, 2)
			draft, err := source.ImportPNG(graphicsPNG(t, bitmap))
			if err != nil {
				t.Fatal(err)
			}
			preview := draft.Preview(maps.NormalPalette)
			if draft.Preview(maps.FogPalette).MapErr == nil {
				t.Fatal("broken fog preview not reported")
			}
			if preview.MapErr == nil {
				t.Fatal("broken preview not reported")
			}
			if scenario == "bad-map" && preview.TilesetErr != nil {
				t.Fatal("layout blocked definitions")
			}
			plan, err := s.Maps.PlanGraphics(draft)
			if err != nil {
				t.Fatal("independent graphics plan", err)
			}
			if err := s.Maps.ApplyGraphics(plan); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMapGraphicsPNGAndCommitFailuresAreAtomic(t *testing.T) {
	for _, scenario := range []string{"invalid", "truecolor", "size", "index", "alpha", "padding", "oversize", "no-op", "palette-only", "absent", "record-overlap", "payload-overlap", "own-payload-overlap", "other-resource-entry", "cross-role-capacity", "revision", "layout", "document", "capacity"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewMapFixture("FE8U")
			list, at := f.GetPosition(
				"map_graphics_list",
			), f.GetPosition(
				"map_graphics",
			)
			switch scenario {
			case "record-overlap":
				f.WritePointer(
					f.GetPosition("map_graphics_pointer"),
					f.GetPosition("chapters")-4,
				)
				f.WritePointer(f.GetPosition("chapters"), at)
			case "own-payload-overlap":
				f.WritePointer(f.GetPosition("map_graphics_pointer"), at)
				f.WritePointer(at+4, at)
			case "payload-overlap":
				f.WritePointer(
					f.GetPosition("map_graphics_pointer"),
					f.GetPosition("map_palette")-4,
				)
				f.WritePointer(f.GetPosition("map_palette"), at)
			case "other-resource-entry":
				f.WritePointer(f.GetPosition("map_palette_pointer"), list-4)
			case "cross-role-capacity":
				f.WriteAt(
					0x240000,
					literalMapLZ(bytes.Repeat([]byte{0x11}, 513*32)),
				)
				f.WritePointer(list+4, 0x240000)
				f.WriteU8(f.GetPosition("chapters")+148+5, 1)
			}
			s := testutil.Open(t, f)
			if scenario == "capacity" {
				if err := s.Document.ApplyEdit(
					s.Document.State().Revision,
					"grow",
					rom.Edit{Size: rom.MaxSize},
				); err != nil {
					t.Fatal(err)
				}
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			if scenario == "absent" {
				if _, err := s.Maps.Graphics(
					0,
					maps.SecondGraphics,
				); !errors.Is(
					err,
					maps.ErrGraphicsAbsent,
				) {
					t.Fatal("absent second stream", err)
				}
			} else {
				source, err := s.Maps.Graphics(0, maps.FirstGraphics)
				if err != nil {
					t.Fatal(err)
				}
				bitmap := exportedGraphics(t, source, 0)
				if scenario != "no-op" && scenario != "palette-only" {
					bitmap.SetColorIndex(0, 0, 2)
				}
				switch scenario {
				case "index":
					bitmap.Palette = append(bitmap.Palette, color.RGBA{A: 255})
					bitmap.SetColorIndex(0, 0, 16)
				case "alpha":
					bitmap.Palette[0] = color.RGBA{}
				case "padding":
					bitmap.SetColorIndex(16, 0, 2)
				case "palette-only":
					bitmap.Palette[2] = color.RGBA{R: 12, G: 34, B: 56, A: 255}
				}
				content := graphicsPNG(t, bitmap)
				switch scenario {
				case "invalid":
					content = []byte("invalid")
				case "truecolor":
					content = graphicsPNG(t, image.NewNRGBA(bitmap.Bounds()))
				case "size":
					content = graphicsPNG(
						t,
						image.NewPaletted(
							image.Rect(0, 0, 256, 8),
							bitmap.Palette,
						),
					)
				case "oversize":
					content = make([]byte, maps.MaxGraphicsPNG+1)
				}
				draft, err := source.ImportPNG(content)
				if slices.Contains(
					[]string{
						"invalid",
						"truecolor",
						"size",
						"index",
						"alpha",
						"padding",
						"oversize",
					},
					scenario,
				) {
					if err == nil {
						t.Fatal("invalid PNG accepted")
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					plan, err := s.Maps.PlanGraphics(draft)
					if scenario == "revision" || scenario == "layout" ||
						scenario == "document" {
						if err != nil {
							t.Fatal(err)
						}
						switch scenario {
						case "revision":
							err = s.Records.Set(
								records.Units,
								[]int{1},
								"level",
								9,
							)
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
					if err == nil {
						t.Fatal("unsafe plan accepted")
					}
					if (scenario == "no-op" || scenario == "palette-only") &&
						!errors.Is(err, edit.ErrValue) {
						t.Fatal("no pixel changes", err)
					}
					if scenario == "capacity" &&
						!errors.Is(err, space.ErrCapacity) {
						t.Fatal("capacity", err)
					}
					if scenario == "cross-role-capacity" &&
						!errors.Is(err, maps.ErrGraphicsReference) {
						t.Fatal("second position capacity", err)
					}
				}
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("failure changed ROM/history")
			}
		})
	}
}

func TestMapGraphicsNativeSheetSizes(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		slot  maps.GraphicsSlot
		count int
	}{
		{"partial-row", maps.FirstGraphics, 17},
		{"first-limit", maps.FirstGraphics, 1024},
		{"second-limit", maps.SecondGraphics, 512},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := testutil.NewMapFixture("FE8U")
			pixels := bytes.Repeat([]byte{0x11}, scenario.count*32)
			pixels[(scenario.count-1)*32] = 0x32
			f.WriteAt(0x240000, literalMapLZ(pixels))
			id := 1
			if scenario.slot == maps.SecondGraphics {
				id = 8
				f.WriteU8(f.GetPosition("chapters")+5, 8)
			}
			f.WritePointer(f.GetPosition("map_graphics_list")+id*4, 0x240000)
			s := testutil.Open(t, f)
			source, err := s.Maps.Graphics(0, scenario.slot)
			if err != nil {
				t.Fatal(err)
			}
			bitmap := exportedGraphics(t, source, 9)
			x, y := (scenario.count-1)%16*8, (scenario.count-1)/16*8
			if bitmap.Bounds() != image.Rect(
				0,
				0,
				128,
				((scenario.count+15)/16)*8,
			) ||
				bitmap.ColorIndexAt(x, y) != 2 ||
				bitmap.ColorIndexAt(x+1, y) != 3 {
				t.Fatal("row transition or native sheet limit")
			}
			bitmap.SetColorIndex(x, y, 4)
			pixels[(scenario.count-1)*32] = 0x34
			draft, err := source.ImportPNG(graphicsPNG(t, bitmap))
			if err != nil {
				t.Fatal(err)
			}
			plan, err := s.Maps.PlanGraphics(draft)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(plan.ChangedTiles(), []int{scenario.count - 1}) {
				t.Fatal("changed tile numbering", plan.ChangedTiles())
			}
			if err := s.Maps.ApplyGraphics(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			decoded, err := gba.DecompressLZ77(
				after.Data[plan.Destination():],
				0x8000,
			)
			if err != nil || !bytes.Equal(decoded, pixels) {
				t.Fatal("fixed pixels at native limit", err)
			}
		})
	}
}

// Literal streams are independently specified, including their exact length.
func literalMapLZ(data []byte) []byte {
	output := []byte{
		0x10,
		byte(len(data)),
		byte(len(data) >> 8),
		byte(len(data) >> 16),
	}
	for at := 0; at < len(data); at += 8 {
		output = append(output, 0)
		output = append(output, data[at:min(at+8, len(data))]...)
	}
	return output
}
