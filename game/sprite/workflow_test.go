package sprite_test

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func TestRecordSpriteWorkflow(t *testing.T) {
	for _, version := range []string{"FE6", "FE7U", "FE7J", "FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewQuoteFixture(version)
			testutil.AddSpriteResources(f, version)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			colors := []color.RGBA{
				{255, 0, 0, 255},
				{0, 255, 0, 255},
				{0, 0, 255, 255},
				{255, 255, 255, 255},
				{255, 255, 0, 255},
				{255, 0, 255, 255},
				{0, 255, 255, 255},
				{132, 132, 132, 255},
			}
			for id, bounds := range []image.Rectangle{image.Rect(0, 0, 16, 16), image.Rect(0, 0, 16, 32), image.Rect(0, 0, 32, 32)} {
				preview := s.Sprites.Standing(id)
				if preview.Err != nil || preview.Image == nil {
					t.Fatalf("standing %d: %v", id, preview.Err)
				}
				if preview.Image.Bounds() != bounds ||
					preview.Image.RGBAAt(1, 0) != colors[id] ||
					preview.Image.RGBAAt(0, 0).A != 0 {
					t.Fatal("standing frame, geometry or transparency differs")
				}
				fixed := s.Sprites.Standing(id | 0x80)
				if fixed.Err != nil ||
					!bytes.Equal(fixed.Image.Pix, preview.Image.Pix) {
					t.Fatal("no-animation flag changed sprite identity")
				}
			}
			for id, want := range colors {
				icon := s.Sprites.Weapon(id)
				if icon.Err != nil || icon.Image == nil {
					t.Fatalf("weapon %d: %v", id, icon.Err)
				}
				if icon.Image.Bounds() != image.Rect(0, 0, 16, 16) ||
					icon.Image.RGBAAt(0, 0) != want ||
					icon.Image.RGBAAt(15, 15) != want {
					t.Fatalf("wrong weapon crop %d", id)
				}
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				before.Revision != s.Document.State().Revision {
				t.Fatal("preview changed ROM")
			}
			old := s.Sprites.Standing(0)
			if old.Current(testutil.Open(t, f).Sprites) {
				t.Fatal("preview accepted a different document")
			}
			at := f.GetPosition("standing_palette") + 2
			if err := s.Document.Apply(
				before.Revision,
				"palette",
				[]rom.Change{{Offset: at, Data: []byte{0xE0, 3}}},
			); err != nil {
				t.Fatal(err)
			}
			if old.Current(s.Sprites) {
				t.Fatal("palette edit did not invalidate preview")
			}
			changed := s.Sprites.Standing(0)
			if changed.Err != nil || changed.Image.RGBAAt(1, 0) != colors[1] {
				t.Fatal("palette edit not reflected")
			}
			if !s.Document.Undo() {
				t.Fatal("undo unavailable")
			}
			restored := s.Sprites.Standing(0)
			if restored.Err != nil ||
				!bytes.Equal(old.Image.Pix, restored.Image.Pix) {
				t.Fatal("undo preview differs")
			}
			table := s.Records.Table(records.Classes)
			if err := s.Records.UseLayout(
				records.Classes,
				table.Layout,
			); err != nil {
				t.Fatal(err)
			}
			if restored.Current(s.Sprites) {
				t.Fatal("layout change retained cached interpretation")
			}
			for _, change := range []rom.Change{
				{Offset: f.GetPosition("standing_table") + 2, Data: []byte{3, 0}},
				{Offset: f.GetPosition("standing_table") + 4, Data: []byte{0, 0, 0, 3}},
				{Offset: f.GetPosition("weapon_data"), Data: []byte{0x10, 0xFF, 0xFF, 0x7F}},
			} {
				if err := s.Document.Apply(
					s.Document.State().Revision,
					"damage",
					[]rom.Change{change},
				); err != nil {
					t.Fatal(err)
				}
				standing, weapon := s.Sprites.Standing(0), s.Sprites.Weapon(0)
				if change.Offset == f.GetPosition("weapon_data") {
					if weapon.Err == nil || weapon.Image != nil {
						t.Fatal("oversize stream accepted")
					}
				} else if standing.Err == nil || standing.Image != nil {
					t.Fatal("unsupported standing resource accepted")
				}
				s.Document.Undo()
			}
			for _, id := range []int{-1, 256} {
				if s.Sprites.Standing(id).Err == nil {
					t.Fatal("invalid sprite ID accepted")
				}
			}
			for _, id := range []int{-1, 8} {
				if s.Sprites.Weapon(id).Err == nil {
					t.Fatal("invalid weapon ID accepted")
				}
			}
		})
	}
}
