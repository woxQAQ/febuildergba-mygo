package game

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestChapterMapWorkflow(t *testing.T) {
	for _, version := range []string{"FE8U", "FE8J", "FE7U", "FE7J", "FE6"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewMapFixture(version)
			s := openFixture(t, f)
			original := s.Document.Snapshot()
			m, err := s.ChapterMap(0)
			if err != nil || m.Width != 2 || m.Height != 1 || m.Image.Bounds() != image.Rect(0, 0, 32, 16) {
				t.Fatalf("map dimensions: %+v, %v", m, err)
			}
			for _, sample := range []struct {
				x, y int
				want color.RGBA
			}{
				{0, 0, color.RGBA{255, 0, 0, 255}},
				{1, 0, color.RGBA{0, 255, 255, 255}},
				{7, 0, color.RGBA{0, 255, 0, 255}},
				{0, 7, color.RGBA{0, 0, 255, 255}},
				{7, 7, color.RGBA{255, 255, 255, 255}},
				{8, 0, color.RGBA{0, 255, 0, 255}},
				{15, 0, color.RGBA{247, 8, 0, 255}},
				{15, 7, color.RGBA{0, 0, 255, 255}},
				{0, 8, color.RGBA{0, 0, 255, 255}},
				{0, 15, color.RGBA{239, 16, 0, 255}},
				{8, 8, color.RGBA{255, 255, 255, 255}},
				{15, 15, color.RGBA{231, 24, 0, 255}},
				{16, 0, color.RGBA{255, 0, 255, 255}},
			} {
				if got := m.Image.RGBAAt(sample.x, sample.y); got != sample.want {
					t.Fatalf("pixel (%d,%d): %v, want %v", sample.x, sample.y, got, sample.want)
				}
			}
			if cell, ok := m.Cell(1, 0); !ok || cell != (MapCell{Tile: 4, Terrain: 0x21}) {
				t.Fatalf("native tile and terrain: %+v, %v", cell, ok)
			}
			for _, point := range []image.Point{{X: -1}, {Y: -1}, {X: 2}, {Y: 1}} {
				if _, ok := m.Cell(point.X, point.Y); ok {
					t.Fatal("out-of-map selection was accepted")
				}
			}
			if !bytes.Equal(original.Data, s.Document.Snapshot().Data) || s.Document.State().Dirty {
				t.Fatal("viewing the map changed the document")
			}
			if err := s.Set(Chapters, []int{0}, "map_palette_id", 7); err != nil {
				t.Fatal(err)
			}
			changed, err := s.ChapterMap(0)
			if err != nil || changed.Image.RGBAAt(0, 0) != (color.RGBA{0, 255, 0, 255}) ||
				m.Image.RGBAAt(0, 0) != (color.RGBA{255, 0, 0, 255}) {
				t.Fatalf("palette edit or previous snapshot isolation: %v", err)
			}
			if !s.Document.Undo() {
				t.Fatal("undo palette edit")
			}
			s.Refresh()
			restored, err := s.ChapterMap(0)
			if err != nil || !bytes.Equal(restored.Image.Pix, m.Image.Pix) || !s.Document.Redo() {
				t.Fatalf("map did not follow undo: %v", err)
			}
			s.Refresh()
			if err := s.Set(Chapters, []int{0}, "map_id", 6); err != nil {
				t.Fatal(err)
			}
			vertical, err := s.ChapterMap(0)
			if err != nil || vertical.Width != 1 || vertical.Height != 2 || vertical.Cells[0].Terrain != 0x21 {
				t.Fatalf("map ID did not select a new layout: %+v, %v", vertical, err)
			}
			if err := s.Set(Chapters, []int{0}, "map_object_id", 5); err != nil {
				t.Fatal(err)
			}
			if _, err := s.ChapterMap(0); !errors.Is(err, ErrMap) {
				t.Fatal("missing second tile was rendered silently")
			}
			if err := s.Set(Chapters, []int{0}, "map_object_alt_id", 8); err != nil {
				t.Fatal(err)
			}
			combined, err := s.ChapterMap(0)
			if err != nil || !bytes.Equal(combined.Image.Pix, vertical.Image.Pix) {
				t.Fatalf("second graphics resource did not restore the map: %v", err)
			}
			move, err := s.PlanTableMove(Chapters, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyTableMove(move); err != nil {
				t.Fatal(err)
			}
			moved, err := s.ChapterMap(0)
			if err != nil || !bytes.Equal(moved.Image.Pix, combined.Image.Pix) {
				t.Fatalf("relocated chapter map: %v", err)
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			saved, err := reopened.ChapterMap(0)
			if err != nil || !bytes.Equal(saved.Image.Pix, combined.Image.Pix) {
				t.Fatalf("map changed after save and reopen: %v", err)
			}
			manual := Layout{Address: reopened.Table(Chapters).Rows[1].Address, Stride: reopened.Table(Chapters).Stride, Count: 1}
			if err := reopened.UseLayout(Chapters, manual); err != nil {
				t.Fatal(err)
			}
			selected, err := reopened.ChapterMap(0)
			if err != nil || selected.Width != 1 || selected.Height != 2 || selected.Image.RGBAAt(0, 16) != (color.RGBA{255, 0, 0, 255}) {
				t.Fatalf("manual chapter layout ignored: %v", err)
			}
			if _, err := reopened.ChapterMap(99); !errors.Is(err, ErrField) {
				t.Fatal("nonexistent chapter accepted")
			}
		})
	}
}

func TestChapterMapRejectsDamagedResources(t *testing.T) {
	for _, sample := range []struct {
		name, resource string
		damage         func(*testutil.FixtureBuilder)
	}{
		{"graphics root", "map_object_id", func(f *testutil.FixtureBuilder) { f.WriteU32(f.GetPosition("map_graphics_pointer"), 0x02000000) }},
		{"graphics entry", "map_object_id", func(f *testutil.FixtureBuilder) { f.WriteU32(f.GetPosition("map_graphics_list")+4, 0xFFFFFFFF) }},
		{"graphics stream", "map_object_id", func(f *testutil.FixtureBuilder) { f.WriteU32(f.GetPosition("map_graphics"), 0xFFFFF010) }},
		{"partial tile", "map_object_id", func(f *testutil.FixtureBuilder) { f.WriteU8(f.GetPosition("map_graphics")+1, 63) }},
		{"palette range", "map_palette_id", func(f *testutil.FixtureBuilder) { f.WritePointer(f.GetPosition("map_palette_list")+2*4, (16<<20)-128) }},
		{"config stream", "map_tileset_id", func(f *testutil.FixtureBuilder) { f.WriteU32(f.GetPosition("map_config"), 0x00240110) }},
		{"missing terrain", "map_tileset_id", func(f *testutil.FixtureBuilder) { f.WriteU32(f.GetPosition("map_config"), 0x001FFF10) }},
		{"map dimensions", "map_id", func(f *testutil.FixtureBuilder) { f.WriteU8(f.GetPosition("map_layout")+5, 0) }},
		{"truncated layout", "map_id", func(f *testutil.FixtureBuilder) { f.WriteU8(f.GetPosition("map_layout")+1, 5) }},
		{"oversized map", "map_id", func(f *testutil.FixtureBuilder) { f.WriteU32(f.GetPosition("map_layout"), 0x000C0310) }},
		{"invalid metatile", "map_tileset_id", func(f *testutil.FixtureBuilder) { f.WriteU16(f.GetPosition("map_layout")+7, 0xFFFF) }},
	} {
		t.Run(sample.name, func(t *testing.T) {
			f := testutil.NewMapFixture("FE8U")
			sample.damage(f)
			s := openFixture(t, f)
			before := s.Document.Snapshot()
			_, err := s.ChapterMap(0)
			var damaged *MapError
			if !errors.Is(err, ErrMap) || !errors.As(err, &damaged) || damaged.Resource != sample.resource {
				t.Fatalf("resource error: %v, want %s", err, sample.resource)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) || s.Document.State().Dirty {
				t.Fatal("failed preview modified the ROM")
			}
			if err := s.Set(Chapters, []int{0}, "weather", 2); err != nil {
				t.Fatalf("map failure disabled chapter editing: %v", err)
			}
		})
	}
}
