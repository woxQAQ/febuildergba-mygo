package itemicon_test

import (
	"bytes"
	"errors"
	"image/color"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestItemIconTableWorkflow(t *testing.T) {
	for index, version := range testutil.Games() {
		t.Run(version.Name, func(t *testing.T) {
			fixture := testutil.ImageFixture(t, index)
			address := fixture.GetPosition("items")
			stride := 36
			if version.Name == "FE6" {
				stride = 32
			}
			data := fixture.Bytes()
			for _, id := range []uint8{2, 3} {
				fixture.WriteAt(
					address+int(id)*stride,
					data[address+stride:address+2*stride],
				)
				fixture.WriteU8(address+int(id)*stride+6, id)
			}
			fixture.WriteU32(address+4*stride+12, 1)
			fixture.WriteU8(address+2*stride+29, 1)
			session := testutil.Open(t, fixture)
			before := session.Document.Snapshot()
			table := session.ItemIcons.Table()
			if table.Err != nil || table.Count != 255 ||
				!table.Current(session.ItemIcons) {
				t.Fatal("relocated native icon window", table)
			}
			for id, expected := range []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}} {
				preview, err := session.ItemIcons.Preview(id)
				if err != nil || preview.Image == nil ||
					preview.Image.RGBAAt(8, 8) != expected {
					t.Fatal("fixed native icon pixels", id, err)
				}
			}
			if _, err := session.ItemIcons.Preview(
				255,
			); !errors.Is(
				err,
				graphics.ErrImage,
			) {
				t.Fatal("reserved icon exposed", err)
			}
			uses, err := session.Records.ItemIconUses()
			if err != nil || len(uses[0]) != 2 || uses[0][0].ID != 1 ||
				uses[0][1].ID != 3 ||
				len(uses[1]) != 1 ||
				uses[1][0].ID != 2 {
				t.Fatal(
					"icon zero lost valid users or included the no-item sentinel",
					uses,
					err,
				)
			}
			references := session.Records.ImageReferences(
				before.Data,
				graphics.Ref{Kind: graphics.ItemIcon, ID: 0},
			)
			if len(references.Uses) != 2 || references.Uses[0].ID != 1 ||
				references.Uses[1].ID != 3 {
				t.Fatal(
					"PNG sharing disagrees with icon users",
					references.Uses,
				)
			}
			uses[1][0].ID = 99
			fresh, err := session.Records.ItemIconUses()
			if err != nil || fresh[1][0].ID != 2 ||
				!bytes.Equal(before.Data, session.Document.Snapshot().Data) {
				t.Fatal("icon browsing modified record data", err)
			}
			if err := session.Records.Set(
				records.Items,
				[]int{3},
				"icon_id",
				1,
			); err != nil {
				t.Fatal(err)
			}
			expected := bytes.Clone(before.Data)
			expected[address+3*stride+29] = 1
			if !bytes.Equal(expected, session.Document.Snapshot().Data) {
				t.Fatal("icon reference changed unrelated bytes")
			}
			if table.Current(session.ItemIcons) {
				t.Fatal("icon table binding survived a document revision")
			}
			fresh, err = session.Records.ItemIconUses()
			if err != nil || len(fresh[1]) != 2 || len(fresh[0]) != 1 {
				t.Fatal("reference change did not refresh uses", fresh, err)
			}
			if !session.Document.Undo() ||
				!bytes.Equal(before.Data, session.Document.Snapshot().Data) {
				t.Fatal("icon reference history")
			}
			session.Refresh()
			if err := session.Records.Set(
				records.Items,
				[]int{0},
				"icon_id",
				1,
			); err != nil {
				t.Fatal(err)
			}
			fresh, err = session.Records.ItemIconUses()
			if err != nil || len(fresh[1]) != 1 || fresh[1][0].ID != 2 {
				t.Fatal("no-item data became an icon user", fresh, err)
			}
			if !session.Document.Undo() ||
				!bytes.Equal(before.Data, session.Document.Snapshot().Data) {
				t.Fatal("no-item data was normalized")
			}
			table = session.ItemIcons.Table()
			if err := session.Records.UseLayout(
				records.Items,
				records.Layout{
					Address: address + stride,
					Stride:  stride,
					Count:   3,
				},
			); err != nil {
				t.Fatal(err)
			}
			if table.Current(session.ItemIcons) {
				t.Fatal("source did not follow interpretation changes")
			}
			fresh, err = session.Records.ItemIconUses()
			if err != nil || len(fresh[1]) != 1 || fresh[1][0].ID != 1 ||
				len(fresh[0]) != 1 ||
				fresh[0][0].ID != 2 {
				t.Fatal("manual item table references", fresh, err)
			}
			other := testutil.Open(t, testutil.ImageFixture(t, index))
			if table.Current(other.ItemIcons) {
				t.Fatal("icon source accepted another document")
			}
		})
	}
}

func TestOriginalIconTableBounds(t *testing.T) {
	for index, original := range []struct{ address, count int }{
		{0x5926F4, 224}, {0x5BA470, 224}, {0xC5EA4, 173}, {0xC12F4, 173}, {0xF9D80, 160},
	} {
		t.Run(testutil.Games()[index].Name, func(t *testing.T) {
			fixture := testutil.ImageFixture(t, index)
			fixture.WritePointer(
				testutil.ImageLayouts[index].IconPointer,
				original.address,
			)
			fixture.WriteAt(
				original.address,
				bytes.Repeat([]byte{0x11}, original.count*128),
			)
			session := testutil.Open(t, fixture)
			table := session.ItemIcons.Table()
			if table.Err != nil || table.Count != original.count {
				t.Fatal("original table maximum", table)
			}
			if _, err := session.ItemIcons.Preview(
				original.count - 1,
			); err != nil {
				t.Fatal("last original icon", err)
			}
			if _, err := session.ItemIcons.Preview(
				original.count,
			); !errors.Is(
				err,
				graphics.ErrImage,
			) {
				t.Fatal("original table read past its declared maximum", err)
			}
			broken := testutil.ImageFixture(t, index)
			broken.WriteU32(testutil.ImageLayouts[index].IconPointer, 0)
			bad := testutil.Open(t, broken)
			if table := bad.ItemIcons.Table(); !errors.Is(
				table.Err,
				graphics.ErrImage,
			) ||
				table.Count != 0 {
				t.Fatal("damaged table exposed entries", table)
			}
		})
	}
}
