package portrait_test

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func TestPortraitTableWorkflow(t *testing.T) {
	for index, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			s := testutil.Open(t, testutil.ImageFixture(t, index))
			before := s.Document.Snapshot()
			entries, err := s.Portraits.Entries()
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 4 || !entries[0].Empty || entries[1].ClassCard ||
				!entries[3].ClassCard ||
				len(entries[1].Uses) != 3 {
				t.Fatalf("native portrait table: %+v", entries)
			}
			entry, err := s.Portraits.Entry(1)
			if err != nil || entry.Address != testutil.ImageTable+entry.Stride {
				t.Fatal("portrait ID address")
			}
			entries[1].Uses[0].ID = 999
			fresh, err := s.Portraits.Entry(1)
			if err != nil || fresh.Uses[0].ID == 999 {
				t.Fatal("mutable portrait index")
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				s.Document.State().Dirty {
				t.Fatal("table browsing changed ROM")
			}
			if err := s.Records.Set(
				records.Units,
				[]int{1},
				"portrait_id",
				3,
			); err != nil {
				t.Fatal(err)
			}
			card, err := s.Portraits.Entry(3)
			if err != nil || len(card.Uses) != 1 || card.Uses[0].ID != 1 {
				t.Fatal("reference index did not refresh")
			}
			if !s.Document.Undo() {
				t.Fatal("portrait reference undo")
			}
			s.Refresh()
			restored, err := s.Portraits.Entry(1)
			if err != nil || len(restored.Uses) != 3 {
				t.Fatal("portrait uses after undo")
			}
			format := testutil.ImageLayouts[index]
			const relocated = 0x350000
			image := s.Document.Snapshot()
			table := bytes.Clone(
				image.Data[testutil.ImageTable : testutil.ImageTable+4*format.PortraitStride],
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
					{Offset: format.PortraitPointer, Data: pointerBytes},
				},
			); err != nil {
				t.Fatal(err)
			}
			moved, err := s.Portraits.Entry(1)
			if err != nil || moved.Address != relocated+format.PortraitStride {
				t.Fatal("index ignored live table pointer")
			}
			originalPNG, err := s.Images.PNG(
				graphics.Ref{Kind: graphics.Portrait, ID: 1},
			)
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
			reopened, err := game.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			reopenedEntry, err := reopened.Portraits.Entry(3)
			if err != nil || !reopenedEntry.ClassCard {
				t.Fatal("portrait table after save")
			}
			if err := reopened.Records.Set(
				records.Classes,
				[]int{1},
				"portrait_id",
				3,
			); err != nil {
				t.Fatal(err)
			}
			card, err = reopened.Portraits.Entry(3)
			if err != nil || len(card.Uses) != 1 ||
				card.Uses[0].Kind != string(records.Classes) {
				t.Fatal("continued portrait reference editing")
			}
		})
	}
}
