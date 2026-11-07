package records_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestItemEffectEditingWorkflow(t *testing.T) {
	for _, version := range testutil.TextVersions {
		t.Run(version.ID, func(t *testing.T) {
			fixture := testutil.TextFixtureFor(version)
			address := fixture.GetPosition("items")
			fixture.WriteU8(address+30, 0xF0)
			fixture.WriteU8(address+31, 0xF1)
			fixture.WriteU32(address+8, 0xA0401001)
			fixture.WriteAt(
				address+12,
				[]byte{0xDE, 0xAD, 0x11, 0x08, 0xFF, 0x12, 0x34, 0x08},
			)
			session := testutil.Open(t, fixture)
			before := session.Document.Snapshot()
			use, err := session.Records.ItemChoices("use_effect")
			if err != nil {
				t.Fatal(err)
			}
			weapon, err := session.Records.ItemChoices("weapon_effect")
			if err != nil {
				t.Fatal(err)
			}
			seen := make(map[int64]bool)
			for _, option := range use {
				if seen[option.Value] || option.Help == "" {
					t.Fatal(
						"ambiguous effect code or missing dispatch explanation",
						option,
					)
				}
				seen[option.Value] = true
			}
			type effectChange struct {
				field, name string
				offset      int
				value       byte
			}
			heal := byte(1)
			eclipse := "item_weapon_effect_halve_hp"
			if version.ID == "FE6" {
				heal = 2
				eclipse = "item_weapon_effect_leave_one_hp"
			}
			changes := []effectChange{
				{"use_effect", "item_use_effect_heal", 30, heal},
				{"weapon_effect", "item_weapon_effect_poison", 31, 1},
				{"weapon_effect", "item_weapon_effect_drain", 31, 2},
				{"weapon_effect", "item_weapon_effect_devil", 31, 4},
				{"weapon_effect", eclipse, 31, 3},
			}
			switch version.ID {
			case "FE7U", "FE7J":
				if !seen[0x2F] || seen[0x33] ||
					slices.ContainsFunc(
						weapon,
						func(option records.ItemChoice) bool { return option.Value == 5 },
					) {
					t.Fatal(
						"FE7 effect vocabulary mixed with FE8 or uncertain entries",
					)
				}
				index := slices.IndexFunc(
					use,
					func(option records.ItemChoice) bool { return option.Value == 0x2A },
				)
				if index < 0 ||
					use[index].Name != "item_use_effect_fillas_might" {
					t.Fatal("FE7 dancer-ring code shifted")
				}
			case "FE8U", "FE8J":
				if seen[0x2F] || seen[0x33] || seen[0x37] || seen[0x38] {
					t.Fatal(
						"uncertain or patch-only effect became a native choice",
					)
				}
				index := slices.IndexFunc(
					use,
					func(option records.ItemChoice) bool { return option.Value == 0x29 },
				)
				if index < 0 ||
					use[index].Name != "item_use_effect_fillas_might" {
					t.Fatal("FE8 dancer-ring code shifted")
				}
				changes = append(
					changes,
					effectChange{
						"weapon_effect",
						"item_weapon_effect_petrify",
						31,
						5,
					},
				)
			}
			for _, change := range changes {
				options, err := session.Records.ItemChoices(change.field)
				if err != nil {
					t.Fatal(err)
				}
				index := slices.IndexFunc(
					options,
					func(option records.ItemChoice) bool { return option.Name == change.name },
				)
				if index < 0 || options[index].Value != int64(change.value) {
					t.Fatal("native effect mapping", change, options)
				}
				if change.field == "use_effect" &&
					options[index].Help != "item_use_effect_scope" {
					t.Fatal("use code presented as an independent handler")
				}
				options[index].Name = "changed copy"
				fresh, err := session.Records.ItemChoices(change.field)
				if err != nil || fresh[index].Name != change.name {
					t.Fatal(
						"returned effect definitions alias the session",
						err,
					)
				}
				if !bytes.Equal(before.Data, session.Document.Snapshot().Data) {
					t.Fatal("effect discovery rewrote a custom code")
				}
				if err := session.Records.Set(
					records.Items,
					[]int{0},
					change.field,
					int64(change.value),
				); err != nil {
					t.Fatal(err)
				}
				expected := bytes.Clone(before.Data)
				expected[address+change.offset] = change.value
				if !bytes.Equal(expected, session.Document.Snapshot().Data) {
					t.Fatal(
						"effect selection modified item ID, flags, pointer or independent effect",
						change,
					)
				}
				if !session.Document.Undo() ||
					!bytes.Equal(
						before.Data,
						session.Document.Snapshot().Data,
					) {
					t.Fatal("effect undo")
				}
				if !session.Document.Redo() ||
					!bytes.Equal(expected, session.Document.Snapshot().Data) {
					t.Fatal("effect redo")
				}
				if !session.Document.Undo() {
					t.Fatal("restore")
				}
				session.Refresh()
			}
			for _, value := range []int64{-1, 256} {
				if err := session.Records.Set(
					records.Items,
					[]int{0},
					"use_effect",
					value,
				); !errors.Is(
					err,
					edit.ErrValue,
				) {
					t.Fatal("invalid effect accepted", value, err)
				}
				if !bytes.Equal(before.Data, session.Document.Snapshot().Data) {
					t.Fatal("failed effect edit modified bytes")
				}
			}
			if err := session.Records.Set(
				records.Items,
				[]int{0},
				"use_effect",
				int64(heal),
			); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "effects.gba")
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
				"weapon_effect",
			); err != nil ||
				value != 0xF1 {
				t.Fatal("saved custom independent effect", value, err)
			}
			if value, err := reopened.Records.Value(
				records.Items,
				0,
				"use_effect",
			); err != nil ||
				value != int64(heal) {
				t.Fatal("saved selected effect", value, err)
			}
		})
	}
}
