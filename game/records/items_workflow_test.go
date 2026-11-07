package records_test

import (
	"bytes"
	"path/filepath"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestItemPresetEditingWorkflow(t *testing.T) {
	for _, version := range testutil.TextVersions {
		t.Run(version.ID, func(t *testing.T) {
			fixture := testutil.TextFixtureFor(version)
			address := fixture.GetPosition("items")
			fixture.WriteU8(address+7, 0xA5)
			fixture.WriteU8(address+25, 0x45)
			fixture.WriteU8(address+28, 50)
			fixture.WriteU32(address+8, 0x80801001)
			fixture.WriteAt(
				address+12,
				[]byte{0xDE, 0xAD, 0x11, 0x08, 0xFF, 0x12, 0x34, 0x08},
			)
			session := testutil.Open(t, fixture)
			before := session.Document.Snapshot()
			rank := byte(31)
			if version.ID == "FE6" {
				rank = 51
			}
			for _, change := range []struct {
				field, name string
				offset      int
				want        byte
			}{
				{"weapon_type", "bow", 7, 3},
				{"weapon_type", "item_type_item", 7, 9},
				{"weapon_rank", "D", 28, rank},
				{"combat.range", "1–2", 25, 0x12},
				{"combat.range", "item_range_magic", 25, 0x10},
				{"combat.range", "item_range_all", 25, 0xFF},
			} {
				options, err := session.Records.ItemChoices(change.field)
				if err != nil {
					t.Fatal(err)
				}
				index := slices.IndexFunc(
					options,
					func(option records.ItemChoice) bool { return option.Name == change.name },
				)
				if index < 0 || options[index].Value != int64(change.want) {
					t.Fatalf(
						"native preset %s / %s: %+v",
						change.field,
						change.name,
						options,
					)
				}
				if !bytes.Equal(before.Data, session.Document.Snapshot().Data) {
					t.Fatal("reading choices normalized a custom value")
				}
				if err := session.Records.Set(
					records.Items,
					[]int{0},
					change.field,
					options[index].Value,
				); err != nil {
					t.Fatal(err)
				}
				expected := bytes.Clone(before.Data)
				expected[address+change.offset] = change.want
				if !bytes.Equal(expected, session.Document.Snapshot().Data) {
					t.Fatal(
						"preset changed unrelated flags, pointers or bytes",
						change.field,
					)
				}
				if !session.Document.Undo() ||
					!bytes.Equal(
						before.Data,
						session.Document.Snapshot().Data,
					) {
					t.Fatal("preset did not undo atomically", change.field)
				}
				if !session.Document.Redo() ||
					!bytes.Equal(expected, session.Document.Snapshot().Data) {
					t.Fatal("preset redo", change.field)
				}
				if !session.Document.Undo() {
					t.Fatal("restore")
				}
				session.Refresh()
			}
			if err := session.Records.Set(
				records.Items,
				[]int{0},
				"combat.range",
				0x12,
			); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "item.gba")
			if err := session.Document.SaveAs(path, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if value, err := reopened.Records.Value(
				records.Items,
				0,
				"combat.range",
			); err != nil ||
				value != 0x12 {
				t.Fatal("saved preset", value, err)
			}
			for _, custom := range []struct {
				field string
				want  int64
			}{{"weapon_type", 0xA5}, {"weapon_rank", 50}, {"flags", 0x80801001}} {
				if value, err := reopened.Records.Value(
					records.Items,
					0,
					custom.field,
				); err != nil ||
					value != custom.want {
					t.Fatal(
						"saved independent custom value",
						custom.field,
						value,
						err,
					)
				}
			}
		})
	}
}
