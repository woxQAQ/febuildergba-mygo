//go:build integration

package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
)

func TestRealChapterMaps(t *testing.T) {
	// Chapter 01 is fixed for every version. English and Japanese releases
	// of the same game share these map pixels; FE6 also needs both tile sets.
	samples := map[string]struct {
		height, mapID, graphicsRoot int
		first                       game.MapCell
		hash                        string
	}{
		"FE8U": {
			10,
			8,
			0x19968,
			game.MapCell{Tile: 0x18C, Terrain: 1},
			"9313001faf13de779daf50fbd6a473e65559f48c5993e1c2069ca16f1472a859",
		},
		"FE8J": {
			10,
			8,
			0x19640,
			game.MapCell{Tile: 0x18C, Terrain: 1},
			"9313001faf13de779daf50fbd6a473e65559f48c5993e1c2069ca16f1472a859",
		},
		"FE7U": {
			10,
			8,
			0x19230,
			game.MapCell{Tile: 0xD0C, Terrain: 0x1A},
			"31b225a5b5b342eac811abfd958a3eee5e0bbcf87009dc469d0543b0b960e51d",
		},
		"FE7J": {
			10,
			8,
			0x19618,
			game.MapCell{Tile: 0xD0C, Terrain: 0x1A},
			"31b225a5b5b342eac811abfd958a3eee5e0bbcf87009dc469d0543b0b960e51d",
		},
		"FE6": {
			21,
			10,
			0x18AE4,
			game.MapCell{Tile: 0x98, Terrain: 1},
			"0eb23eb2510c32fa6f2a6cf199e1f132e910250cc10e2262ffe874f472078d31",
		},
	}
	for _, fixture := range realROMFixtures {
		t.Run(fixture.id, func(t *testing.T) {
			sample := samples[fixture.id]
			s := openRealROM(t, fixture.filename)
			check := func(s *game.Session) {
				t.Helper()
				preview, err := s.ChapterMap(1)
				if err != nil || preview.Width != 15 ||
					preview.Height != sample.height ||
					preview.Cells[0] != sample.first {
					t.Fatalf("fixed chapter map: %+v, %v", preview, err)
				}
				hash := sha256.Sum256(preview.Image.Pix)
				if hex.EncodeToString(hash[:]) != sample.hash {
					t.Fatalf("native map pixels changed: %x", hash)
				}
				if value, err := s.Value(
					game.Chapters,
					1,
					"map_id",
				); err != nil ||
					value != int64(sample.mapID) {
					t.Fatalf("fixed map resource ID: %d, %v", value, err)
				}
			}
			check(s)
			root, err := s.ReadRegion(sample.graphicsRoot, 4)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyRegion(root, []byte{0, 0, 0, 0}); err != nil {
				t.Fatal(err)
			}
			if _, err := s.ChapterMap(1); !errors.Is(err, game.ErrMap) {
				t.Fatalf("bad graphics pointer was not rejected: %v", err)
			}
			if !s.Document.Undo() {
				t.Fatal("undo map resource pointer")
			}
			s.Refresh()
			check(s)
			move, err := s.PlanTableMove(game.Chapters, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyTableMove(move); err != nil {
				t.Fatal(err)
			}
			check(s)
			if !s.Document.Undo() || !s.Document.Redo() {
				t.Fatal("chapter map relocation history")
			}
			s.Refresh()
			check(s)
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			check(reopened)
			t.Logf(
				"chapter 01: 15x%d cells, map ID %02X, fixed pixel digest %s",
				sample.height,
				sample.mapID,
				sample.hash,
			)
		})
	}
}
