package maps_test

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

func palettePNG(t *testing.T, bitmap image.Image) []byte {
	t.Helper()
	var output bytes.Buffer
	if err := png.Encode(&output, bitmap); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func exportedPalette(
	t *testing.T,
	source *maps.Palette,
	section maps.PaletteSection,
) *image.Paletted {
	t.Helper()
	content, err := source.PNG(section)
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

func TestMapPalettePNGEditingAndHistory(t *testing.T) {
	for _, version := range testutil.Games() {
		for _, section := range []maps.PaletteSection{maps.NormalPalette, maps.FogPalette, maps.AllPalettes} {
			t.Run(
				fmt.Sprintf("%s/%d", version.Name, section),
				func(t *testing.T) {
					f := testutil.NewMapFixture(version.Name)
					stride := 148
					if version.Name == "FE6" {
						stride = 68
					}
					if version.Name == "FE7U" {
						stride = 152
					}
					chapter, palette := f.GetPosition(
						"chapters",
					), f.GetPosition(
						"map_palette",
					)
					f.WriteAt(
						chapter+2*stride,
						f.Bytes()[chapter:chapter+stride],
					)
					f.WriteU8(chapter+2*stride+6, 9)
					f.WritePointer(
						f.GetPosition("map_palette_list")+9*4,
						palette,
					)
					f.WriteU16(palette+2, 0x801F)
					f.WriteU16(palette+162, 0xFC00)
					f.WriteAt(palette+320, bytes.Repeat([]byte{0xA5, 0xD7}, 96))
					s := testutil.Open(t, f)
					before := s.Document.Snapshot()
					source, err := s.Maps.Palette(0)
					if err != nil {
						t.Fatal(err)
					}
					bitmap := exportedPalette(t, source, section)
					height := 40
					if section == maps.AllPalettes {
						height = 80
					}
					if bitmap.Bounds() != image.Rect(0, 0, 128, height) ||
						bitmap.ColorIndexAt(7, 7) != 0 ||
						bitmap.ColorIndexAt(8, 0) != 1 ||
						bitmap.ColorIndexAt(127, 39) != 79 {
						t.Fatal("fixed chart dimensions or slot indices")
					}
					if section == maps.AllPalettes &&
						bitmap.ColorIndexAt(8, 40) != 81 {
						t.Fatal("full chart fog slot indices")
					}
					wantSlot := color.RGBA{R: 247, G: 8, A: 255}
					if section == maps.FogPalette {
						wantSlot = color.RGBA{R: 206, G: 49, A: 255}
					}
					if got := color.RGBAModel.Convert(bitmap.Palette[17]).(color.RGBA); got != wantSlot {
						t.Fatal("fixed bank/color order", got, wantSlot)
					}
					if got := color.RGBAModel.Convert(bitmap.Palette[0]).(color.RGBA); got.A != 255 {
						t.Fatal("color zero became transparent")
					}
					want := bytes.Clone(before.Data[palette : palette+320])
					if section == maps.NormalPalette ||
						section == maps.AllPalettes {
						bitmap.Palette[1] = color.RGBA{G: 255, A: 255}
						want[2], want[3] = 0xE0, 0x83
					} else {
						bitmap.Palette[1] = color.RGBA{R: 255, G: 255, A: 255}
						want[162], want[163] = 0xFF, 0x83
					}
					if section == maps.AllPalettes {
						bitmap.Palette[81] = color.RGBA{R: 255, G: 255, A: 255}
						want[162], want[163] = 0xFF, 0x83
					}
					draft, err := source.ImportPNG(
						section,
						palettePNG(t, bitmap),
					)
					if err != nil {
						t.Fatal(err)
					}
					preview := draft.Preview(maps.NormalPalette)
					fog := draft.Preview(maps.FogPalette)
					if preview.MapErr != nil || fog.MapErr != nil {
						t.Fatal("map/fog previews", preview.MapErr, fog.MapErr)
					}
					wantNormal, wantFog := color.RGBA{
						R: 255,
						A: 255,
					}, color.RGBA{
						B: 255,
						A: 255,
					}
					if section != maps.FogPalette {
						wantNormal = color.RGBA{G: 255, A: 255}
					}
					if section != maps.NormalPalette {
						wantFog = color.RGBA{R: 255, G: 255, A: 255}
					}
					if preview.Map.Image.RGBAAt(0, 0) != wantNormal ||
						fog.Map.Image.RGBAAt(0, 0) != wantFog {
						t.Fatal("normal/fog color isolation")
					}
					plan, err := s.Maps.PlanPalette(draft)
					if err != nil {
						t.Fatal(err)
					}
					if plan.Length() != 320 || len(plan.Uses()) != 3 ||
						!plan.Uses()[0].Updated ||
						!plan.Uses()[1].Updated ||
						plan.Uses()[2].Updated {
						t.Fatal("native length or sharing", plan.Uses())
					}
					changes := plan.Changes()
					if len(changes) < 1 {
						t.Fatal("missing changes")
					}
					changes[0].After = color.RGBA{}
					if plan.Changes()[0].After.A != 255 {
						t.Fatal("mutable preview")
					}
					if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
						t.Fatal("preview wrote ROM")
					}
					if err := s.Maps.ApplyPalette(plan); err != nil {
						t.Fatal(err)
					}
					after := s.Document.Snapshot()
					at, ok := gba.Pointer(after.Data, plan.Pointer())
					if !ok || at != len(before.Data) ||
						at != plan.Destination() ||
						len(after.Data) != at+320 ||
						!bytes.Equal(want, after.Data[at:]) {
						t.Fatal("native palette bytes/high bits/allocation")
					}
					preserved := bytes.Clone(after.Data[:len(before.Data)])
					copy(
						preserved[plan.Pointer():plan.Pointer()+4],
						before.Data[plan.Pointer():plan.Pointer()+4],
					)
					if !bytes.Equal(before.Data, preserved) {
						t.Fatal(
							"old palette, following 192 bytes or other resources changed",
						)
					}
					if source.Current(s.Maps) ||
						!errors.Is(s.Maps.ApplyPalette(plan), rom.ErrStale) {
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
					loaded, err := reopened.Maps.Palette(0)
					if err != nil ||
						loaded.Preview(
							maps.NormalPalette,
						).Map.Image.RGBAAt(
							0,
							0,
						) != wantNormal {
						t.Fatal("save/reopen", err)
					}
					independent, err := reopened.Maps.Palette(2)
					if err != nil ||
						independent.Preview(
							maps.NormalPalette,
						).Map.Image.RGBAAt(
							0,
							0,
						) != (color.RGBA{R: 255, A: 255}) {
						t.Fatal("independent alias changed", err)
					}
				},
			)
		}
	}
}

func TestMapPaletteNativeBoundaryAndIndependentResources(t *testing.T) {
	for _, scenario := range []string{"rom-end", "following-entry", "bad-map", "bad-graphics", "bad-shared-graphics", "unknown-bank"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewMapFixture("FE8U")
			switch scenario {
			case "rom-end":
				palette := bytes.Clone(
					f.Bytes()[f.GetPosition("map_palette") : f.GetPosition("map_palette")+320],
				)
				at := len(f.Bytes()) - 320
				f.WriteAt(at, palette)
				f.WritePointer(f.GetPosition("map_palette_list")+8, at)
			case "following-entry":
				f.WritePointer(
					f.GetPosition("map_config_pointer"),
					f.GetPosition("map_palette")+320-12,
				)
				f.WritePointer(
					f.GetPosition("map_palette")+320,
					f.GetPosition("map_config"),
				)
			case "unknown-bank":
				f.WriteU8(f.GetPosition("map_config")+6, 0xA0)
			case "bad-map":
				f.WriteU8(f.GetPosition("map_layout"), 0)
			case "bad-graphics":
				f.WriteU8(f.GetPosition("map_graphics"), 0)
			case "bad-shared-graphics":
				f.WriteU8(f.GetPosition("chapters")+148+4, 255)
			}
			s := testutil.Open(t, f)
			source, err := s.Maps.Palette(0)
			if err != nil {
				t.Fatal("palette blocked by independent resource", err)
			}
			if scenario == "unknown-bank" {
				if _, err := s.Maps.Read(0); !errors.Is(err, maps.ErrMap) {
					t.Fatal("unknown bank borrowed adjacent colors", err)
				}
			}
			if scenario == "rom-end" {
				if _, err := s.Maps.Read(0); err != nil {
					t.Fatal("320 bytes at ROM end were not enough", err)
				}
			}
			if scenario == "following-entry" {
				definitions, err := s.Maps.NewTilesetDraft(0)
				if err != nil {
					t.Fatal(err)
				}
				if err := definitions.SetTerrain(0, 2); err != nil {
					t.Fatal(err)
				}
				if _, err := s.Maps.PlanTileset(definitions); err != nil {
					t.Fatal(
						"palette claims cover unrelated following bytes",
						err,
					)
				}
			}
			bitmap := exportedPalette(t, source, maps.NormalPalette)
			bitmap.Palette[1] = color.RGBA{B: 255, A: 255}
			draft, err := source.ImportPNG(
				maps.NormalPalette,
				palettePNG(t, bitmap),
			)
			if err != nil {
				t.Fatal(err)
			}
			plan, err := s.Maps.PlanPalette(draft)
			if err != nil {
				t.Fatal("independent palette plan", err)
			}
			if err := s.Maps.ApplyPalette(plan); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMapPalettePNGAndCommitFailuresAreAtomic(t *testing.T) {
	for _, scenario := range []string{"pixel", "truecolor", "size", "alpha", "oversize", "negative-section", "wide-section", "no-op", "record-overlap", "payload-overlap", "other-resource-entry", "revision", "layout", "document", "capacity"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewMapFixture("FE8U")
			palette := f.GetPosition("map_palette")
			switch scenario {
			case "record-overlap":
				f.WritePointer(
					f.GetPosition("map_palette_pointer"),
					f.GetPosition("chapters")-8,
				)
				f.WritePointer(f.GetPosition("chapters"), palette)
			case "payload-overlap":
				f.WritePointer(f.GetPosition("map_palette_pointer"), palette-8)
				f.WritePointer(palette, palette)
			case "other-resource-entry":
				f.WritePointer(
					f.GetPosition("map_config_pointer"),
					f.GetPosition("map_palette_list")-4,
				)
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
			source, err := s.Maps.Palette(0)
			if err != nil {
				t.Fatal(err)
			}
			bitmap := exportedPalette(t, source, maps.NormalPalette)
			bitmap.Palette[1] = color.RGBA{B: 255, A: 255}
			var content []byte
			switch scenario {
			case "pixel":
				bitmap.SetColorIndex(0, 0, 1)
			case "alpha":
				bitmap.Palette[0] = color.RGBA{}
			case "no-op":
				bitmap = exportedPalette(t, source, maps.NormalPalette)
			}
			switch scenario {
			case "truecolor":
				content = palettePNG(
					t,
					image.NewNRGBA(image.Rect(0, 0, 128, 40)),
				)
			case "size":
				content = palettePNG(
					t,
					image.NewPaletted(image.Rect(0, 0, 16, 5), bitmap.Palette),
				)
			case "oversize":
				content = make([]byte, maps.MaxPalettePNG+1)
			default:
				content = palettePNG(t, bitmap)
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			section := maps.NormalPalette
			switch scenario {
			case "negative-section":
				section = -1
			case "wide-section":
				section = 256
			}
			draft, err := source.ImportPNG(section, content)
			switch scenario {
			case "negative-section", "wide-section":
				if !errors.Is(err, edit.ErrValue) {
					t.Fatal("out-of-range palette section", section, err)
				}
			case "pixel", "truecolor", "size", "alpha", "oversize":
				if err == nil {
					t.Fatal("invalid PNG accepted")
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
				plan, err := s.Maps.PlanPalette(draft)
				if scenario == "revision" || scenario == "layout" ||
					scenario == "document" {
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
					if _, err := s.Maps.PlanPalette(
						draft,
					); !errors.Is(
						err,
						rom.ErrStale,
					) {
						t.Fatal("stale draft", err)
					}
					err = s.Maps.ApplyPalette(plan)
				}
				if err == nil {
					t.Fatal("unsafe plan accepted")
				}
				if scenario == "no-op" && !errors.Is(err, edit.ErrValue) {
					t.Fatal("no-op", err)
				}
				if scenario == "capacity" &&
					!errors.Is(err, romwrite.ErrCapacity) {
					t.Fatal("capacity", err)
				}
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("failure changed ROM/history")
			}
		})
	}
}
