//go:build integration

package integration

import (
	"bytes"
	"image"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
)

func TestRealPortraitEditing(t *testing.T) {
	samples := map[string]struct{ address, mouthY, eyesY int }{
		"FE8U": {0x8ACBFC, 6, 4}, "FE8J": {0x901154, 6, 4},
		"FE7U": {0xC965BC, 5, 3}, "FE7J": {0xD5E274, 5, 3},
		"FE6": {0x66076C, 6, 0},
	}
	for _, fixture := range realROMFixtures {
		t.Run(fixture.id, func(t *testing.T) {
			s := openRealROM(t, fixture.filename)
			baseline := s.Document.Snapshot()
			sample := samples[fixture.id]
			p, err := s.Portrait(2)
			if err != nil || p.Entry.Address != sample.address || p.Mouth != image.Pt(2, sample.mouthY) || p.Main == nil || p.Mini == nil || p.Sheet == nil || p.MainErr != nil || p.MiniErr != nil || p.SheetErr != nil {
				t.Fatalf("native portrait: %+v %v", p, err)
			}
			if fixture.id != "FE6" && (p.Eyes != image.Pt(3, sample.eyesY) || p.Blink != 1) {
				t.Fatal("native eye fields")
			}
			static, err := s.Preview(game.AssetRef{Kind: game.PortraitAsset, ID: 2})
			if err != nil {
				t.Fatal(err)
			}
			original, err := p.Frame(-1, 0)
			if err != nil || !bytes.Equal(original.Pix, static.Image.Pix) {
				t.Fatal("native base rendering")
			}
			mouth, err := p.Frame(0, 2)
			if err != nil || bytes.Equal(original.Pix, mouth.Pix) {
				t.Fatal("native mouth frame")
			}
			fields := map[string]int64{"portrait_mouth_x": 3, "portrait_mouth_y": int64(sample.mouthY + 1)}
			expected := bytes.Clone(baseline.Data)
			offset := 20
			if fixture.id == "FE6" {
				offset = 12
			}
			expected[sample.address+offset], expected[sample.address+offset+1] = 3, byte(sample.mouthY+1)
			if p.HasEyes {
				fields["portrait_blink"] = 6
				expected[sample.address+24] = 6
			}
			if err := s.SetPortrait(p, fields); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(expected, s.Document.Snapshot().Data) {
				t.Fatal("native position edit changed other bytes")
			}
			if !s.Document.Undo() || !bytes.Equal(baseline.Data, s.Document.Snapshot().Data) {
				t.Fatal("native portrait undo")
			}
			if !s.Document.Redo() {
				t.Fatal("native portrait redo")
			}
			s.Refresh()
			if err := s.Set(game.Units, []int{1}, "mini_portrait", 1); err != nil {
				t.Fatal(err)
			}
			r, err := s.RecordPortrait(game.Units, 1)
			if err != nil || r.GenericMini != 1 || r.MiniErr != nil || r.Mini.Image == nil || len(r.MiniResources) != 2 {
				t.Fatal("native generic miniature", err, r.MiniErr)
			}
			path := filepath.Join(t.TempDir(), "portrait-edit.gba")
			if err := s.Document.SaveAs(path, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			p, err = reopened.Portrait(2)
			if err != nil || p.Mouth != image.Pt(3, sample.mouthY+1) {
				t.Fatal("native portrait save/reopen")
			}
			if err := reopened.SetPortrait(p, map[string]int64{"portrait_mouth_x": 2}); err != nil {
				t.Fatal("continued portrait editing", err)
			}
		})
	}
}
