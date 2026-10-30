//go:build integration

package integration

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/portrait"
)

func editablePortrait(t *testing.T, s *game.Session) *portrait.Portrait {
	t.Helper()
	entries, err := s.Portraits.Entries()
	if err != nil {
		t.Skipf("portrait table unavailable: %v", err)
	}
	for _, entry := range entries {
		if entry.Err != nil || entry.Empty || entry.ClassCard {
			continue
		}
		portrait, err := s.Portraits.Read(entry.ID)
		if err == nil && len(portrait.Fields) > 0 && portrait.SheetErr == nil &&
			portrait.Sheet != nil && len(sheetColors(portrait.Sheet)) == 2 {
			t.Logf("portrait ID=%d", entry.ID)
			return portrait
		}
	}
	t.Skip("no editable native portrait")
	return nil
}

func editPortrait(t *testing.T, s *game.Session) {
	portrait := editablePortrait(t, s)
	before := s.Document.Snapshot()
	field := portrait.Fields[0]
	value := field.Value(before.Data, portrait.Entry.Address)
	if err := s.Portraits.Set(
		portrait,
		map[string]int64{field.ID: value ^ 1},
	); err != nil {
		t.Fatal(err)
	}
	expected := bytes.Clone(before.Data)
	expected[portrait.Entry.Address+field.Offset] ^= 1
	if !bytes.Equal(s.Document.Snapshot().Data, expected) {
		t.Fatal("portrait coordinate edit changed unrelated bytes")
	}
	reopened := historyAndReopen(t, s, before)
	result, err := reopened.Portraits.Read(portrait.Entry.ID)
	if err != nil ||
		field.Value(
			reopened.Document.Snapshot().Data,
			result.Entry.Address,
		) != value^1 {
		t.Fatalf("portrait coordinate after reopening: %v", err)
	}
}

func replacePNG(t *testing.T, s *game.Session) {
	portrait := editablePortrait(t, s)
	ref := graphics.Ref{Kind: graphics.Portrait, ID: portrait.Entry.ID}
	bitmap := portrait.Sheet
	colors := sheetColors(bitmap)
	expected := image.NewRGBA(bitmap.Bounds())
	copy(expected.Pix, bitmap.Pix)
	for y := bitmap.Bounds().Min.Y; y < bitmap.Bounds().Max.Y; y++ {
		for x := bitmap.Bounds().Min.X; x < bitmap.Bounds().Max.X; x++ {
			switch bitmap.RGBAAt(x, y) {
			case colors[0]:
				expected.SetRGBA(x, y, colors[1])
			case colors[1]:
				expected.SetRGBA(x, y, colors[0])
			}
		}
	}
	var input bytes.Buffer
	if err := png.Encode(&input, expected); err != nil {
		t.Fatal(err)
	}
	before := s.Document.Snapshot()
	edit, err := s.Images.Plan(ref, input.Bytes())
	checkPlan(t, s, before, err)
	if err := s.Images.Apply(edit); err != nil {
		t.Fatal(err)
	}
	reopened := historyAndReopen(t, s, before)
	saved, err := reopened.Images.PNG(ref)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(saved))
	if err != nil || decoded.Bounds() != expected.Bounds() {
		t.Fatalf("PNG dimensions after reopening: %v", err)
	}
	for y := expected.Bounds().Min.Y; y < expected.Bounds().Max.Y; y++ {
		for x := expected.Bounds().Min.X; x < expected.Bounds().Max.X; x++ {
			if color.RGBAModel.Convert(
				decoded.At(x, y),
			) != expected.RGBAAt(
				x,
				y,
			) {
				t.Fatalf("PNG pixel changed at (%d, %d)", x, y)
			}
		}
	}
}

func sheetColors(bitmap *image.RGBA) []color.RGBA {
	var colors []color.RGBA
	for y := bitmap.Bounds().Min.Y; y < bitmap.Bounds().Max.Y; y++ {
		for x := bitmap.Bounds().Min.X; x < bitmap.Bounds().Max.X; x++ {
			pixel := bitmap.RGBAAt(x, y)
			if pixel.A == 0 || len(colors) > 0 && pixel == colors[0] {
				continue
			}
			colors = append(colors, pixel)
			if len(colors) == 2 {
				return colors
			}
		}
	}
	return colors
}
