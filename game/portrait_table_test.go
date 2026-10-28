package game

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func TestPortraitTableWorkflow(t *testing.T) {
	for index, fixture := range gameFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			s := openFixture(t, assetFixture(t, index))
			before := s.Document.Snapshot()
			entries, err := s.PortraitEntries()
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 4 || !entries[0].Empty || entries[1].ClassCard ||
				!entries[3].ClassCard ||
				len(entries[1].Uses) != 3 {
				t.Fatalf("native portrait table: %+v", entries)
			}
			entry, err := s.PortraitEntry(1)
			if err != nil || entry.Address != assetTable+entry.Stride {
				t.Fatal("portrait ID address")
			}
			entries[1].Uses[0].ID = 999
			fresh, err := s.PortraitEntry(1)
			if err != nil || fresh.Uses[0].ID == 999 {
				t.Fatal("mutable portrait index")
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				s.Document.State().Dirty {
				t.Fatal("table browsing changed ROM")
			}
			if err := s.Set(Units, []int{1}, "portrait_id", 3); err != nil {
				t.Fatal(err)
			}
			card, err := s.PortraitEntry(3)
			if err != nil || len(card.Uses) != 1 || card.Uses[0].ID != 1 {
				t.Fatal("reference index did not refresh")
			}
			if !s.Document.Undo() {
				t.Fatal("portrait reference undo")
			}
			s.Refresh()
			restored, err := s.PortraitEntry(1)
			if err != nil || len(restored.Uses) != 3 {
				t.Fatal("portrait uses after undo")
			}
			format := s.Adapter.(gameFormat).portraits
			const relocated = 0x350000
			image := s.Document.Snapshot()
			table := bytes.Clone(
				image.Data[assetTable : assetTable+4*format.stride],
			)
			pointerBytes := binary.LittleEndian.AppendUint32(
				nil,
				0x08000000+relocated,
			)
			if err := s.Document.Apply(
				image.Revision,
				"relocated portrait fixture",
				[]rom.Change{
					{Offset: relocated, Data: table},
					{Offset: format.pointer, Data: pointerBytes},
				},
			); err != nil {
				t.Fatal(err)
			}
			moved, err := s.PortraitEntry(1)
			if err != nil || moved.Address != relocated+format.stride {
				t.Fatal("index ignored live table pointer")
			}
			originalPNG, err := s.AssetPNG(AssetRef{PortraitAsset, 1})
			if err != nil {
				t.Fatal(err)
			}
			if len(originalPNG) == 0 {
				t.Fatal("portrait table export")
			}
			output := filepath.Join(t.TempDir(), "portrait-table.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(output)
			if err != nil {
				t.Fatal(err)
			}
			reopenedEntry, err := reopened.PortraitEntry(3)
			if err != nil || !reopenedEntry.ClassCard {
				t.Fatal("portrait table after save")
			}
			if err := reopened.Set(
				Classes,
				[]int{1},
				"portrait_id",
				3,
			); err != nil {
				t.Fatal(err)
			}
			card, err = reopened.PortraitEntry(3)
			if err != nil || len(card.Uses) != 1 ||
				card.Uses[0].Kind != Classes {
				t.Fatal("continued portrait reference editing")
			}
		})
	}
}
