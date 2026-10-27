package game

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

const assetTable, assetFace, assetMini, assetMouth, assetPalette, assetCard, assetIcons = 0x340000, 0x341000, 0x344000, 0x345000, 0x346000, 0x347000, 0x348000

func assetFixture(t *testing.T, index int) *testutil.FixtureBuilder {
	t.Helper()
	f := gameFixtures()[index].build()
	format := []gameFormat{fe8uFormat(), fe8jFormat(), fe7uFormat(), fe7jFormat(), fe6Format()}[index]
	f.WritePointer(format.portraits.pointer, assetTable)
	f.WritePointer(format.icons.pointer, assetIcons)
	f.WritePointer(format.icons.palette, assetPalette)
	f.WriteAt(assetPalette, []byte{0, 0, 0x1F, 0, 0xE0, 3, 0, 0x7C})
	f.WriteAt(assetIcons, bytes.Repeat([]byte{0x11}, 128))
	f.WriteAt(assetIcons+128, bytes.Repeat([]byte{0x22}, 128))
	length := 4096
	if format.portraits.stride == 16 {
		length = 4352
	}
	raw := bytes.Repeat([]byte{0x11}, length)
	if index == 2 || index == 4 {
		compressed, err := gba.CompressLZ77(raw)
		if err != nil {
			t.Fatal(err)
		}
		// A two-literal prefix and distance-two run are valid for BIOS VRAM
		// decoding.
		if !bytes.Equal(compressed[4:9], []byte{0x3F, 0x11, 0x11, 0xF0, 1}) {
			t.Fatalf("noncanonical VRAM packet prefix: % X", compressed[4:9])
		}
		f.WriteAt(assetFace, compressed)
	} else {
		f.WriteAt(assetFace, append([]byte{0, 4, 0x10, 0}, raw...))
	}
	mini := bytes.Repeat([]byte{0x22}, 512)
	if index != 4 {
		var err error
		mini, err = gba.CompressLZ77(mini)
		if err != nil {
			t.Fatal(err)
		}
	}
	f.WriteAt(assetMini, mini)
	f.WriteAt(assetMouth, bytes.Repeat([]byte{0x33}, 1536))
	card, err := gba.CompressLZ77(bytes.Repeat([]byte{0x22}, 3200))
	if err != nil {
		t.Fatal(err)
	}
	f.WriteAt(assetCard, card)
	for _, id := range []int{1, 2} {
		at := assetTable + id*format.portraits.stride
		f.WritePointer(at, assetFace)
		f.WritePointer(at+4, assetMini)
		f.WritePointer(at+8, assetPalette)
		if index != 4 {
			f.WritePointer(at+12, assetMouth)
		}
	}
	cardAt := assetTable + 3*format.portraits.stride
	f.WritePointer(cardAt+format.portraits.cardPointer, assetCard)
	f.WritePointer(cardAt+8, assetPalette)
	unitStride, classStride := 52, 84
	if index == 4 {
		unitStride, classStride = 48, 72
	}
	f.WriteU16(f.GetPosition("units")+6, 1)
	f.WriteU16(f.GetPosition("units")+unitStride+6, 1)
	f.WriteU16(f.GetPosition("classes")+8, 1)
	f.WriteU16(f.GetPosition("classes")+classStride+8, 2)
	return f
}

func imagePNG(t *testing.T, bitmap image.Image) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := png.Encode(&out, bitmap); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestAssetReplacementWorkflows(t *testing.T) {
	for index, fixture := range gameFixtures() {
		for _, ref := range []AssetRef{{ItemIconAsset, 0}, {PortraitAsset, 1}, {PortraitAsset, 3}} {
			t.Run(
				fmt.Sprintf("%s/%s/%d", fixture.name, ref.Kind, ref.ID),
				func(t *testing.T) {
					s := openFixture(t, assetFixture(t, index))
					format := s.Adapter.(gameFormat)
					original, err := s.AssetPNG(ref)
					if err != nil {
						t.Fatal(err)
					}
					aliasRef := AssetRef{PortraitAsset, 2}
					if ref.Kind == ItemIconAsset {
						aliasRef = AssetRef{ItemIconAsset, 1}
					}
					alias, err := s.AssetPNG(aliasRef)
					if err != nil {
						t.Fatal(err)
					}
					bounds := image.Rect(0, 0, 16, 16)
					points := []image.Point{{0, 0}}
					ink, wantInk := color.RGBA{
						0,
						255,
						0,
						255,
					}, color.RGBA{
						0,
						255,
						0,
						255,
					}
					wantUses := 2
					if ref.Kind == PortraitAsset {
						ink, wantInk = color.RGBA{
							123,
							64,
							32,
							255,
						}, color.RGBA{
							123,
							66,
							33,
							255,
						}
						bounds = image.Rect(0, 0, 128, 112)
						points = []image.Point{
							{16, 0},
							{96, 16},
							{0, 80},
							{96, 80},
						}
						wantUses = 3
						if index != 4 {
							points = append(points, image.Pt(96, 48))
						}
						if ref.ID == 3 {
							bounds, points, wantUses = image.Rect(
								0,
								0,
								80,
								80,
							), []image.Point{
								{0, 0},
								{79, 79},
							}, 0
						}
					}
					replacement, expected := image.NewRGBA(
						bounds,
					), image.NewRGBA(
						bounds,
					)
					for _, point := range points {
						replacement.SetRGBA(point.X, point.Y, ink)
						expected.SetRGBA(point.X, point.Y, wantInk)
					}
					before := s.Document.Snapshot()
					edit, err := s.PlanAssetEdit(ref, imagePNG(t, replacement))
					if err != nil {
						t.Fatal(err)
					}
					if len(edit.Uses()) != wantUses ||
						!bytes.Equal(edit.Image().Pix, expected.Pix) ||
						!bytes.Equal(s.Document.Snapshot().Data, before.Data) {
						t.Fatal(
							"preview lost colors, component placement, sharing, or changed ROM bytes",
						)
					}
					imageCopy := edit.Image()
					clear(imageCopy.Pix)
					if !bytes.Equal(edit.Image().Pix, expected.Pix) {
						t.Fatal("preview image is mutable")
					}
					if err := s.ApplyAssetEdit(edit); err != nil {
						t.Fatal(err)
					}
					after := s.Document.Snapshot()
					preserved := bytes.Clone(after.Data[:len(before.Data)])
					allowed := []int{format.icons.pointer}
					if ref.Kind == PortraitAsset {
						at := assetTable + ref.ID*format.portraits.stride
						allowed = []int{at, at + 4, at + 8}
						if index != 4 {
							allowed = append(allowed, at+12)
						}
						if ref.ID == 3 {
							allowed = []int{
								at + format.portraits.cardPointer,
								at + 8,
							}
						}
					}
					for _, address := range allowed {
						copy(
							preserved[address:address+4],
							before.Data[address:address+4],
						)
					}
					if !bytes.Equal(preserved, before.Data) ||
						len(after.Data) <= len(before.Data) {
						t.Fatal(
							"replacement overwrote original resources, unrelated fields or portrait coordinates",
						)
					}
					unchangedAlias, err := s.AssetPNG(aliasRef)
					if err != nil || !bytes.Equal(alias, unchangedAlias) {
						t.Fatal("an unrelated asset ID changed")
					}
					if !s.Document.Undo() ||
						!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
						!s.Document.Redo() {
						t.Fatal(
							"image history did not restore all data and file length",
						)
					}
					if err := s.ApplyAssetEdit(
						edit,
					); !errors.Is(
						err,
						rom.ErrStale,
					) {
						t.Fatal("used preview was accepted")
					}
					if err := s.Document.Save(); err != nil {
						t.Fatal(err)
					}
					reopened, err := Open(s.Document.State().Path)
					if err != nil {
						t.Fatal(err)
					}
					continued, err := reopened.PlanAssetEdit(ref, original)
					if err != nil {
						t.Fatal(err)
					}
					if err := reopened.ApplyAssetEdit(continued); err != nil {
						t.Fatal(err)
					}
				},
			)
		}
	}
}

func TestAssetImportRejections(t *testing.T) {
	s := openFixture(t, assetFixture(t, 0))
	icon := image.NewRGBA(image.Rect(0, 0, 16, 16))
	icon.SetRGBA(0, 0, color.RGBA{0, 255, 0, 255})
	valid := imagePNG(t, icon)
	before := s.Document.Snapshot()
	wrongSize := image.NewRGBA(image.Rect(0, 0, 24, 16))
	unknownColor := image.NewRGBA(icon.Bounds())
	unknownColor.SetRGBA(0, 0, color.RGBA{255, 255, 0, 255})
	partialAlpha := image.NewNRGBA(image.Rect(0, 0, 128, 112))
	partialAlpha.SetNRGBA(16, 0, color.NRGBA{255, 0, 0, 128})
	partialAlpha16 := image.NewNRGBA64(partialAlpha.Bounds())
	partialAlpha16.SetNRGBA64(16, 0, color.NRGBA64{0xFFFF, 0, 0, 0xFFFE})
	unmapped := image.NewRGBA(partialAlpha.Bounds())
	unmapped.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	manyColors := image.NewRGBA(partialAlpha.Bounds())
	for i := range uint8(16) {
		manyColors.SetRGBA(16+int(i), 0, color.RGBA{i * 16, 0, 0, 255})
	}
	unchanged, err := s.AssetPNG(AssetRef{ItemIconAsset, 0})
	if err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct {
		name string
		ref  AssetRef
		png  []byte
		want error
	}{
		{"size", AssetRef{ItemIconAsset, 0}, imagePNG(t, wrongSize), ErrAssetSize},
		{"shared palette", AssetRef{ItemIconAsset, 0}, imagePNG(t, unknownColor), ErrAssetColors},
		{"alpha", AssetRef{PortraitAsset, 1}, imagePNG(t, partialAlpha), ErrAssetColors},
		{"16-bit alpha", AssetRef{PortraitAsset, 1}, imagePNG(t, partialAlpha16), ErrAssetColors},
		{"unused pixels", AssetRef{PortraitAsset, 1}, imagePNG(t, unmapped), ErrAssetPixels},
		{"colors", AssetRef{PortraitAsset, 1}, imagePNG(t, manyColors), ErrAssetColors},
		{"unchanged", AssetRef{ItemIconAsset, 0}, unchanged, ErrValue},
		{"reserved icon", AssetRef{ItemIconAsset, 255}, valid, ErrImage},
		{"reserved portrait", AssetRef{PortraitAsset, 0}, valid, ErrImage},
		{"truncated PNG", AssetRef{ItemIconAsset, 0}, valid[:24], ErrAssetSize},
		{"file limit", AssetRef{ItemIconAsset, 0}, make([]byte, MaxAssetPNG+1), ErrAssetSize},
	} {
		t.Run(sample.name, func(t *testing.T) {
			if _, err := s.PlanAssetEdit(
				sample.ref,
				sample.png,
			); !errors.Is(
				err,
				sample.want,
			) {
				t.Fatalf("got %v, want %v", err, sample.want)
			}
			if s.Document.State().Revision != before.Revision ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("rejected import changed the document")
			}
		})
	}
	edit, err := s.PlanAssetEdit(AssetRef{ItemIconAsset, 0}, valid)
	if err != nil {
		t.Fatal(err)
	}
	other := openFixture(t, assetFixture(t, 0))
	if err := other.ApplyAssetEdit(edit); !errors.Is(err, rom.ErrStale) {
		t.Fatal("foreign preview was accepted")
	}
	layout := s.Table(Items).Layout
	if err := s.UseLayout(Items, layout); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyAssetEdit(edit); !errors.Is(err, rom.ErrStale) {
		t.Fatal("preview survived a layout change")
	}
	full := make([]byte, rom.MaxSize)
	copy(full, before.Data)
	if _, err := fe8uFormat().PlanAssetEdit(full, AssetRef{ItemIconAsset, 0}, icon); !errors.Is(
		err,
		space.ErrCapacity,
	) {
		t.Fatalf("full ROM: %v", err)
	}
	// A valid-looking portrait pointer may still target another live table.
	corrupt := assetFixture(t, 0)
	corrupt.WritePointer(0x5524, corrupt.GetPosition("units"))
	at := corrupt.GetPosition("units") + 28
	corrupt.WritePointer(at, assetFace)
	corrupt.WritePointer(at+4, assetMini)
	corrupt.WritePointer(at+8, assetPalette)
	corrupt.WritePointer(at+12, assetMouth)
	bad := openFixture(t, corrupt)
	portrait := image.NewRGBA(image.Rect(0, 0, 128, 112))
	portrait.SetRGBA(16, 0, color.RGBA{0, 255, 0, 255})
	if _, err := bad.PlanAssetEdit(
		AssetRef{PortraitAsset, 1},
		imagePNG(t, portrait),
	); !errors.Is(
		err,
		ErrLayout,
	) {
		t.Fatalf("overlapping live table: %v", err)
	}
}
