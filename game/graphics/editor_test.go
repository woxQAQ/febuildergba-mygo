package graphics_test

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestAssetReplacementWorkflows(t *testing.T) {
	for index, fixture := range testutil.Games() {
		for _, ref := range []graphics.Ref{{Kind: graphics.ItemIcon, ID: 0}, {Kind: graphics.Portrait, ID: 1}, {Kind: graphics.Portrait, ID: 3}} {
			t.Run(
				fmt.Sprintf("%s/%s/%d", fixture.Name, ref.Kind, ref.ID),
				func(t *testing.T) {
					s := testutil.Open(t, testutil.ImageFixture(t, index))
					format := testutil.ImageLayouts[index]
					original, err := s.Images.PNG(ref)
					if err != nil {
						t.Fatal(err)
					}
					aliasRef := graphics.Ref{Kind: graphics.Portrait, ID: 2}
					if ref.Kind == graphics.ItemIcon {
						aliasRef = graphics.Ref{Kind: graphics.ItemIcon, ID: 1}
					}
					alias, err := s.Images.PNG(aliasRef)
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
					if ref.Kind == graphics.Portrait {
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
					plan, err := s.Images.Plan(
						ref,
						testutil.ImagePNG(t, replacement),
					)
					if err != nil {
						t.Fatal(err)
					}
					if len(plan.Uses()) != wantUses ||
						!bytes.Equal(plan.Image().Pix, expected.Pix) ||
						!bytes.Equal(s.Document.Snapshot().Data, before.Data) {
						t.Fatal(
							"preview lost colors, component placement, sharing, or changed ROM bytes",
						)
					}
					imageCopy := plan.Image()
					clear(imageCopy.Pix)
					if !bytes.Equal(plan.Image().Pix, expected.Pix) {
						t.Fatal("preview image is mutable")
					}
					if err := s.Images.Apply(plan); err != nil {
						t.Fatal(err)
					}
					after := s.Document.Snapshot()
					preserved := bytes.Clone(after.Data[:len(before.Data)])
					allowed := []int{format.IconPointer}
					if ref.Kind == graphics.Portrait {
						at := testutil.ImageTable + ref.ID*format.PortraitStride
						allowed = []int{at, at + 4, at + 8}
						if index != 4 {
							allowed = append(allowed, at+12)
						}
						if ref.ID == 3 {
							allowed = []int{
								at + format.CardOffset,
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
					unchangedAlias, err := s.Images.PNG(aliasRef)
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
					if err := s.Images.Apply(
						plan,
					); !errors.Is(
						err,
						rom.ErrStale,
					) {
						t.Fatal("used preview was accepted")
					}
					if err := s.Document.Save(); err != nil {
						t.Fatal(err)
					}
					reopened, err := game.Open(s.Document.State().Path)
					if err != nil {
						t.Fatal(err)
					}
					continued, err := reopened.Images.Plan(ref, original)
					if err != nil {
						t.Fatal(err)
					}
					if err := reopened.Images.Apply(continued); err != nil {
						t.Fatal(err)
					}
				},
			)
		}
	}
}

func TestAssetImportRejections(t *testing.T) {
	s := testutil.Open(t, testutil.ImageFixture(t, 0))
	icon := image.NewRGBA(image.Rect(0, 0, 16, 16))
	icon.SetRGBA(0, 0, color.RGBA{0, 255, 0, 255})
	valid := testutil.ImagePNG(t, icon)
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
	unchanged, err := s.Images.PNG(graphics.Ref{Kind: graphics.ItemIcon, ID: 0})
	if err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct {
		name string
		ref  graphics.Ref
		png  []byte
		want error
	}{
		{"size", graphics.Ref{Kind: graphics.ItemIcon, ID: 0}, testutil.ImagePNG(t, wrongSize), graphics.ErrSize},
		{"shared palette", graphics.Ref{Kind: graphics.ItemIcon, ID: 0}, testutil.ImagePNG(t, unknownColor), graphics.ErrColors},
		{"alpha", graphics.Ref{Kind: graphics.Portrait, ID: 1}, testutil.ImagePNG(t, partialAlpha), graphics.ErrColors},
		{"16-bit alpha", graphics.Ref{Kind: graphics.Portrait, ID: 1}, testutil.ImagePNG(t, partialAlpha16), graphics.ErrColors},
		{"unused pixels", graphics.Ref{Kind: graphics.Portrait, ID: 1}, testutil.ImagePNG(t, unmapped), graphics.ErrPixels},
		{"colors", graphics.Ref{Kind: graphics.Portrait, ID: 1}, testutil.ImagePNG(t, manyColors), graphics.ErrColors},
		{"unchanged", graphics.Ref{Kind: graphics.ItemIcon, ID: 0}, unchanged, edit.ErrValue},
		{"reserved icon", graphics.Ref{Kind: graphics.ItemIcon, ID: 255}, valid, graphics.ErrImage},
		{"reserved portrait", graphics.Ref{Kind: graphics.Portrait, ID: 0}, valid, graphics.ErrImage},
		{"truncated PNG", graphics.Ref{Kind: graphics.ItemIcon, ID: 0}, valid[:24], graphics.ErrImage},
		{"file limit", graphics.Ref{Kind: graphics.ItemIcon, ID: 0}, make([]byte, graphics.MaxPNG+1), graphics.ErrSize},
	} {
		t.Run(sample.name, func(t *testing.T) {
			if _, err := s.Images.Plan(
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
	plan, err := s.Images.Plan(
		graphics.Ref{Kind: graphics.ItemIcon, ID: 0},
		valid,
	)
	if err != nil {
		t.Fatal(err)
	}
	other := testutil.Open(t, testutil.ImageFixture(t, 0))
	if err := other.Images.Apply(plan); !errors.Is(err, rom.ErrStale) {
		t.Fatal("foreign preview was accepted")
	}
	layout := s.Records.Table(records.Items).Layout
	if err := s.Records.UseLayout(records.Items, layout); err != nil {
		t.Fatal(err)
	}
	if err := s.Images.Apply(plan); !errors.Is(err, rom.ErrStale) {
		t.Fatal("preview survived a layout change")
	}
	full := testutil.NewFixture("FE8U", rom.MaxSize)
	full.WriteAt(0, before.Data)
	fullSession := testutil.Open(t, full)
	if _, err := fullSession.Images.Plan(
		graphics.Ref{Kind: graphics.ItemIcon, ID: 0},
		valid,
	); !errors.Is(
		err,
		space.ErrCapacity,
	) {
		t.Fatalf("full ROM: %v", err)
	}
	// A valid-looking portrait pointer may still target another live table.
	corrupt := testutil.ImageFixture(t, 0)
	corrupt.WritePointer(0x5524, corrupt.GetPosition("units"))
	at := corrupt.GetPosition("units") + 28
	corrupt.WritePointer(at, testutil.ImageFace)
	corrupt.WritePointer(at+4, testutil.ImageMini)
	corrupt.WritePointer(at+8, testutil.ImagePalette)
	corrupt.WritePointer(at+12, testutil.ImageMouth)
	bad := testutil.Open(t, corrupt)
	portrait := image.NewRGBA(image.Rect(0, 0, 128, 112))
	portrait.SetRGBA(16, 0, color.RGBA{0, 255, 0, 255})
	if _, err := bad.Images.Plan(
		graphics.Ref{Kind: graphics.Portrait, ID: 1},
		testutil.ImagePNG(t, portrait),
	); !errors.Is(
		err,
		edit.ErrLayout,
	) {
		t.Fatalf("overlapping live table: %v", err)
	}
}
