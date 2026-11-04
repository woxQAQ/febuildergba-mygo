//go:build integration

package integration

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
)

func importMapGraphicsAndPalette(t *testing.T, s *game.Session) {
	t.Helper()
	chapters := requireTable(t, s, records.Chapters)
	for _, row := range chapters.Rows {
		for _, slot := range []maps.GraphicsSlot{maps.FirstGraphics, maps.SecondGraphics} {
			source, err := s.Maps.Graphics(row.ID, slot)
			if err != nil || source.PaletteCount() != 10 {
				continue
			}
			content, err := source.PNG(0)
			if err != nil {
				continue
			}
			decoded, err := png.Decode(bytes.NewReader(content))
			if err != nil {
				t.Fatal(err)
			}
			bitmap := decoded.(*image.Paletted)
			index := bitmap.ColorIndexAt(0, 0) ^ 1
			bitmap.SetColorIndex(0, 0, index)
			original := color.NRGBAModel.Convert(bitmap.Palette[index]).(color.NRGBA)
			replacement := color.NRGBA{G: original.G, B: original.B, A: 255}
			if original.R == 0 {
				replacement.R = 255
			}
			bitmap.Palette[index] = replacement
			var output bytes.Buffer
			if err := png.Encode(&output, bitmap); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "map-graphics.png")
			if err := os.WriteFile(path, output.Bytes(), 0o600); err != nil {
				t.Fatal(err)
			}
			input, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			before := s.Document.Snapshot()
			draft, err := source.ImportPNG(
				input,
				maps.GraphicsImport{WithPalette: true},
			)
			if err != nil {
				t.Fatal(err)
			}
			plan, err := s.Maps.PlanGraphics(draft)
			checkPlan(t, s, before, err)
			if len(plan.ChangedTiles()) == 0 ||
				len(plan.PaletteChanges()) != 1 {
				t.Fatal("missing joint changes")
			}
			if err := s.Maps.ApplyGraphics(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			preserved := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				preserved[plan.Pointer():plan.Pointer()+4],
				before.Data[plan.Pointer():plan.Pointer()+4],
			)
			for _, use := range plan.PaletteUses() {
				if use.Updated {
					copy(
						preserved[use.Pointer:use.Pointer+4],
						before.Data[use.Pointer:use.Pointer+4],
					)
				}
			}
			if !bytes.Equal(preserved, before.Data) {
				t.Fatal(
					"joint import changed old resources or independent entries",
				)
			}
			reopened := historyAndReopen(t, s, before)
			loaded, err := reopened.Maps.Graphics(row.ID, slot)
			if err != nil {
				t.Fatal(err)
			}
			result, err := loaded.Image(0)
			if err != nil || result.ColorIndexAt(0, 0) != index ||
				color.NRGBAModel.Convert(result.Palette[index]) != replacement {
				t.Fatal("joint import changed after save/reopen", err)
			}
			t.Logf(
				"chapter=%02X, slot=%d, graphics uses=%d, palette uses=%d",
				row.ID,
				slot,
				len(plan.Uses()),
				len(plan.PaletteUses()),
			)
			return
		}
	}
	t.Skip("no readable existing map graphics and native palette")
}
