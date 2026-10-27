//go:build integration

package integration

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
)

func TestRealAssetReplacements(t *testing.T) {
	classCards := map[string]int{"FE8U": 0x8B, "FE8J": 0x8C, "FE7U": 0xD7, "FE7J": 0xD7, "FE6": 0xB2}
	for _, fixture := range realROMFixtures {
		for _, sample := range []struct {
			ref  game.AssetRef
			size image.Point
		}{
			{game.AssetRef{Kind: game.ItemIconAsset, ID: 0}, image.Pt(16, 16)},
			{game.AssetRef{Kind: game.PortraitAsset, ID: 1}, image.Pt(128, 112)},
			{game.AssetRef{Kind: game.PortraitAsset, ID: classCards[fixture.id]}, image.Pt(80, 72)},
		} {
			ref := sample.ref
			t.Run(fmt.Sprintf("%s/%s/%04X", fixture.id, ref.Kind, ref.ID), func(t *testing.T) {
				s := openRealROM(t, fixture.filename)
				exported, err := s.AssetPNG(ref)
				if err != nil {
					t.Fatal(err)
				}
				original, err := png.Decode(bytes.NewReader(exported))
				if err != nil {
					t.Fatal(err)
				}
				wantSize := sample.size
				if wantSize == image.Pt(80, 72) {
					value, err := s.Value(game.Classes, 1, "portrait_id")
					if err != nil || value != int64(ref.ID) {
						t.Fatalf("class 01 no longer references fixed card ID %04X: %v", ref.ID, err)
					}
				}
				if original.Bounds().Size() != wantSize {
					t.Fatalf("unexpected native sheet dimensions: %v", original.Bounds())
				}
				var ink color.RGBA
				for y := range wantSize.Y {
					for x := range wantSize.X {
						pixel := color.RGBAModel.Convert(original.At(x, y)).(color.RGBA)
						if pixel.A == 255 {
							ink = pixel
						}
					}
				}
				if ink.A == 0 {
					t.Fatal("fixed native asset has no opaque pixels")
				}
				replacement := image.NewRGBA(original.Bounds())
				replacement.SetRGBA(0, 0, ink)
				if wantSize == image.Pt(128, 112) {
					replacement.SetRGBA(0, 0, color.RGBA{})
					for _, point := range []image.Point{{16, 0}, {96, 16}, {0, 80}, {96, 80}} {
						replacement.SetRGBA(point.X, point.Y, ink)
					}
					if fixture.id != "FE6" {
						replacement.SetRGBA(96, 48, ink)
					}
				}
				var content bytes.Buffer
				if err := png.Encode(&content, replacement); err != nil {
					t.Fatal(err)
				}
				before := s.Document.Snapshot()
				edit, err := s.PlanAssetEdit(ref, content.Bytes())
				if err != nil {
					t.Fatal(err)
				}
				if s.Document.State().Revision != before.Revision || !bytes.Equal(edit.Image().Pix, replacement.Pix) {
					t.Fatal("planning changed the document or lost image components")
				}
				if err := s.ApplyAssetEdit(edit); err != nil {
					t.Fatal(err)
				}
				if !s.Document.Undo() || !bytes.Equal(before.Data, s.Document.Snapshot().Data) || !s.Document.Redo() {
					t.Fatal("asset replacement did not undo and redo completely")
				}
				if err := s.Document.Save(); err != nil {
					t.Fatal(err)
				}
				reopened, err := game.Open(s.Document.State().Path)
				if err != nil {
					t.Fatal(err)
				}
				saved, err := reopened.AssetPNG(ref)
				if err != nil {
					t.Fatal(err)
				}
				decoded, err := png.Decode(bytes.NewReader(saved))
				if err != nil {
					t.Fatal(err)
				}
				for y := range wantSize.Y {
					for x := range wantSize.X {
						if color.RGBAModel.Convert(decoded.At(x, y)) != replacement.At(x, y) {
							t.Fatalf("saved image changed at %d,%d", x, y)
						}
					}
				}
				continued, err := reopened.PlanAssetEdit(ref, exported)
				if err != nil {
					t.Fatal(err)
				}
				if err := reopened.ApplyAssetEdit(continued); err != nil {
					t.Fatal(err)
				}
				t.Logf("fixed asset ID %04X: complete sheet, history, save/reopen and continued editing", ref.ID)
			})
		}
	}
}
