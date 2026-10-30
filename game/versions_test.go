package game_test

import (
	"bytes"
	"errors"
	"image/color"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestSessionNames(t *testing.T) {
	for _, tt := range []struct {
		name                       string
		build                      func() *testutil.FixtureBuilder
		text, recovery, tree, root int
		japanese                   bool
	}{
		{"FE8U", testutil.NewMinimalFE8U, 0xA2A0, 0x15D48C, 0x6E0, 0x6DC, false},
		{"FE8J", testutil.NewMinimalFE8J, 0xA000, 0x14D08C, 0x6E0, 0x6DC, true},
		{"FE7U", testutil.NewMinimalFE7U, 0x12CB8, 0xB808AC, 0x6BC, 0x6B8, false},
		{"FE7J", testutil.NewMinimalFE7J, 0x13370, 0xBBB370, 0x6E0, 0x6DC, true},
		{"FE6", testutil.NewMinimalFE6, 0x13B10, 0xF635C, 0x6E0, 0x6DC, true},
	} {
		for _, recovery := range []bool{false, true} {
			mode := "primary"
			if recovery {
				mode = "recovery"
			}
			t.Run(tt.name+"/"+mode, func(t *testing.T) {
				f := tt.build()
				const tree, root, stream, table, raw, classText = 0x310000, 0x310100, 0x311000, 0x312000, 0x313000, 0x313100
				first, second := uint32(0x794C), uint32(0x6E) // "Ly", "n".
				want := "Lyn"
				rawName := []byte("Lyn\x00")
				if tt.japanese {
					first, second = 0x8A83, 0x9383 // Shift-JIS pairs for リン.
					want = "リン"
					rawName = []byte{0x83, 0x8A, 0x83, 0x93, 0}
				}
				// Codes 0, 10, 11 mean first pair, second pair, terminator;
				// bits are LSB-first.
				f.WriteU16(tree, 1)
				f.WriteU16(tree+2, 2)
				f.WriteU32(tree+4, 0x80000000|first)
				f.WriteU16(tree+8, 3)
				f.WriteU16(tree+10, 4)
				f.WriteU32(tree+12, 0x80000000|second)
				f.WriteU32(tree+16, 0x80000000)
				f.WritePointer(tt.tree, tree)
				f.WritePointer(tt.root, root)
				f.WritePointer(root, tree)
				f.WriteAt(stream, []byte{0x1A})
				f.WriteAt(raw, rawName)
				className := "Knight"
				f.WriteAt(classText, []byte("Knight\x00"))
				if tt.name == "FE7J" {
					// FE7J's seven compact glyphs ①..⑦ denote Dragon Master.
					f.WriteAt(
						classText,
						[]byte{
							0x87,
							0x40,
							0x87,
							0x41,
							0x87,
							0x42,
							0x87,
							0x43,
							0x87,
							0x44,
							0x87,
							0x45,
							0x87,
							0x46,
							0,
						},
					)
					className = "(ﾄﾞﾗｺﾞﾝﾏｽﾀｰ)"
				}
				text := table
				if recovery {
					text = tt.recovery
				} else {
					f.WritePointer(tt.text, text)
				}
				f.WritePointer(text+4, stream)
				f.WriteU32(text+8, 0x88000000|raw)
				f.WriteU32(text+12, 0x88000000|classText)
				f.WriteU16(f.GetPosition("units"), 1)
				unitStride := 52
				if tt.name == "FE6" {
					unitStride = 48
				}
				f.WriteU16(f.GetPosition("units")+unitStride, 2)
				f.WriteU16(f.GetPosition("classes"), 3)
				s := testutil.Open(t, f)
				for i, row := range s.Records.Table(records.Units).Rows[:2] {
					if row.TextError != nil || row.Name != want {
						t.Fatalf(
							"name %d: %q, %v; want %q",
							i,
							row.Name,
							row.TextError,
							want,
						)
					}
				}
				row := s.Records.Table(records.Classes).Rows[0]
				if row.TextError != nil || row.Name != className {
					t.Fatalf(
						"class name %q, %v; want %q",
						row.Name,
						row.TextError,
						className,
					)
				}
			})
		}
	}
}

func TestFE7Previews(t *testing.T) {
	for _, tt := range []struct {
		name                    string
		build                   func() *testutil.FixtureBuilder
		portrait, icon, palette int
		compressed              bool
	}{
		{"FE7U", testutil.NewMinimalFE7U, 0x6B30, 0x4E20, 0x4D40, true},
		{"FE7J", testutil.NewMinimalFE7J, 0x69C0, 0x4CFC, 0x4C1C, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.build()
			const portraits, face, card, icons, palette = 0x320000, 0x321000, 0x323000, 0x325000, 0x327000
			f.WritePointer(tt.portrait, portraits)
			f.WritePointer(tt.icon, icons)
			f.WritePointer(tt.palette, palette)
			f.WriteU16(palette+2, 0x001F) // Palette index 1 is opaque red.
			f.WritePointer(portraits+28, face)
			f.WritePointer(portraits+28+8, palette)
			f.WritePointer(portraits+56+16, card)
			f.WritePointer(portraits+56+8, palette)
			f.WriteAt(icons+128, bytes.Repeat([]byte{0x11}, 128))
			// Fixed LZ77 samples contain only literal blocks, eight tile bytes
			// per flag.
			block := []byte{0, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11}
			if tt.compressed {
				f.WriteAt(
					face,
					append(
						[]byte{0x10, 0, 0x10, 0},
						bytes.Repeat(block, 512)...),
				)
			} else {
				f.WriteAt(
					face,
					append(
						[]byte{0, 4, 0x10, 0},
						bytes.Repeat([]byte{0x11}, 4096)...),
				)
			}
			f.WriteAt(
				card,
				append(
					[]byte{0x10, 0x80, 0x0C, 0},
					bytes.Repeat(block, 400)...),
			)
			s := testutil.Open(t, f)
			for _, asset := range []struct {
				ref           graphics.Ref
				width, height int
				x             int
			}{
				{graphics.Ref{Kind: graphics.Portrait, ID: 1}, 96, 80, 16},
				{graphics.Ref{Kind: graphics.Portrait, ID: 2}, 80, 80, 0},
				{graphics.Ref{Kind: graphics.ItemIcon, ID: 1}, 16, 16, 0},
			} {
				preview, err := s.Images.Preview(asset.ref)
				if err != nil || preview.Image == nil {
					t.Fatalf("%+v: image missing, %v", asset.ref, err)
				}
				if bounds := preview.Image.Bounds(); bounds.Dx() != asset.width ||
					bounds.Dy() != asset.height {
					t.Fatalf("%+v: image size %v", asset.ref, bounds)
				}
				if got := preview.Image.RGBAAt(
					asset.x,
					0,
				); got != (color.RGBA{R: 255, A: 255}) {
					t.Fatalf("%+v: pixel %+v", asset.ref, got)
				}
			}
		})
	}
}

func TestFE7TableLimits(t *testing.T) {
	for _, build := range []func() *testutil.FixtureBuilder{testutil.NewMinimalFE7U, testutil.NewMinimalFE7J} {
		f := build()
		f.WriteU8(f.GetPosition("units")+252*52+4, 253)
		f.WriteU32(f.GetPosition("items")+36+16, 0xFFFFFFFF)
		s := testutil.Open(t, f)
		t.Run(s.Version(), func(t *testing.T) {
			units := s.Records.Table(records.Units)
			if units.Err != nil || units.Count != 253 ||
				units.Rows[252].ID != 253 {
				t.Fatalf("unit count %d, error %v", units.Count, units.Err)
			}
			if s.Records.Table(records.Items).Count != 1 {
				t.Fatal("invalid effectiveness pointer did not terminate items")
			}
			for _, edit := range []struct {
				kind   records.Kind
				id     int
				field  string
				offset int
			}{
				{records.Units, 253, "base.hp", f.GetPosition("units") + 252*52 + 12},
				{records.Classes, 1, "promotion.power", f.GetPosition("classes") + 84 + 35},
				{records.Items, 0, "weapon_experience", f.GetPosition("items") + 32},
			} {
				if err := s.Records.Set(
					edit.kind,
					[]int{edit.id},
					edit.field,
					17,
				); err != nil {
					t.Fatal(err)
				}
				if s.Document.Snapshot().Data[edit.offset] != 17 {
					t.Fatalf(
						"%s/%s wrote the wrong field",
						edit.kind,
						edit.field,
					)
				}
			}
			before := s.Document.Snapshot()
			if err := s.Records.Set(
				records.Units,
				[]int{254},
				"base.hp",
				17,
			); !errors.Is(err, edit.ErrField) ||
				s.Document.State().Revision != before.Revision {
				t.Fatalf("out-of-range character: %v", err)
			}
		})
	}
}

func TestFE7RejectsInvalidROMs(t *testing.T) {
	for _, build := range []func() *testutil.FixtureBuilder{testutil.NewMinimalFE7U, testutil.NewMinimalFE7J} {
		f := build()
		f.WritePointer(f.GetPosition("units_pointer"), (16<<20)-52)
		s := testutil.Open(t, f)
		t.Run(s.Version(), func(t *testing.T) {
			if !errors.Is(
				s.Records.Table(records.Units).Err,
				edit.ErrLayout,
			) {
				t.Fatal("truncated fixed-size character table accepted")
			}
			if _, err := s.Records.PlanTableMove(
				records.Units,
				nil,
			); !errors.Is(err, edit.ErrLayout) ||
				s.Document.State().Dirty {
				t.Fatalf("invalid table move changed document: %v", err)
			}
		})
	}
	for _, header := range []string{"AE7E01", "AE7J01", "AE7P01"} {
		t.Run(header, func(t *testing.T) {
			size := 16 << 20
			if header != "AE7P01" {
				size = 8 << 20
			}
			path := filepath.Join(t.TempDir(), "invalid.gba")
			if err := testutil.NewFixture(header, size).Save(path); err != nil {
				t.Fatal(err)
			}
			if _, err := game.Open(path); !errors.Is(err, edit.ErrVersion) {
				t.Fatalf("unsupported version or ROM size accepted: %v", err)
			}
		})
	}
}
