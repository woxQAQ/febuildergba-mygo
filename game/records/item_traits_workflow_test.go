package records_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestItemTraitEditingWorkflow(t *testing.T) {
	for _, version := range testutil.TextVersions {
		t.Run(version.ID, func(t *testing.T) {
			fixture := testutil.TextFixtureFor(version)
			address := fixture.GetPosition("items")
			const original = uint32(0xA0409001)
			fixture.WriteU32(address+8, original)
			fixture.WriteU8(address+28, 31)
			fixture.WriteAt(
				address+12,
				[]byte{0xDE, 0xAD, 0x11, 0x08, 0xFF, 0x12, 0x34, 0x08},
			)
			session := testutil.Open(t, fixture)
			before := session.Document.Snapshot()
			traits := session.Records.RecordTraits(records.Items)
			if len(traits) != 32 || traits[5].Label != "item_trait_brave" ||
				traits[5].Group != records.ItemTraitCombat || !traits[5].Known {
				t.Fatal(
					"item battle behavior is not independently modeled",
					traits,
				)
			}
			for bit, trait := range traits {
				if bit != trait.Bit || trait.Known && trait.Group == "" {
					t.Fatal("trait cannot be addressed or grouped", trait)
				}
			}
			character := session.Records.RecordTraits(records.Units)
			if character[5].Label != "trait_play" || character[5].Group != "" {
				t.Fatal("item traits replaced character traits")
			}
			wantLock := "item_trait_display_prf"
			wantDragon := "item_trait_dragon_lock"
			wantLeader := "item_trait_eliwood_lock"
			switch version.ID {
			case "FE6":
				wantLock = "item_trait_zephiel_lock"
				if traits[11].Label != "item_trait_roy_lock" ||
					traits[15].Known ||
					traits[18].Known {
					t.Fatal("later-generation bits were assigned FE6 meanings")
				}
			case "FE8U", "FE8J":
				wantDragon, wantLeader = "item_trait_monster_lock", "item_trait_eirika_lock"
				if traits[20].Label != "item_trait_weapon_lock_6" ||
					traits[21].Label != "item_trait_weapon_lock_7" {
					t.Fatal("FE7 character names leaked into unused FE8 locks")
				}
			}
			if traits[13].Label != wantLock || traits[10].Label != wantDragon ||
				version.ID != "FE6" && traits[18].Label != wantLeader {
				t.Fatal(
					"generation-specific lock vocabulary",
					traits[10],
					traits[13],
					traits[18],
				)
			}
			traits[5].Known = false
			if !session.Records.RecordTraits(records.Items)[5].Known ||
				!bytes.Equal(before.Data, session.Document.Snapshot().Data) {
				t.Fatal(
					"reading or changing returned metadata modified the session",
				)
			}
			if err := session.Records.SetRecordTrait(
				records.Items,
				0,
				5,
				true,
			); err != nil {
				t.Fatal(err)
			}
			expected := bytes.Clone(before.Data)
			binary.LittleEndian.PutUint32(expected[address+8:], original|0x20)
			if !bytes.Equal(expected, session.Document.Snapshot().Data) {
				t.Fatal(
					"brave toggle changed unknown bits, pointers or unrelated fields",
				)
			}
			revision := session.Document.State().Revision
			if err := session.Records.SetRecordTrait(
				records.Items,
				0,
				5,
				true,
			); err != nil ||
				session.Document.State().Revision != revision {
				t.Fatal("unchanged trait created history", err)
			}
			if !session.Document.Undo() ||
				!bytes.Equal(before.Data, session.Document.Snapshot().Data) {
				t.Fatal("trait toggle did not undo as one edit")
			}
			if !session.Document.Redo() ||
				!bytes.Equal(expected, session.Document.Snapshot().Data) {
				t.Fatal("trait redo")
			}
			// Item classification flags remain independent; enabling staff does
			// not clear the existing weapon or swordfighter flags.
			if err := session.Records.SetRecordTrait(
				records.Items,
				0,
				2,
				true,
			); err != nil {
				t.Fatal(err)
			}
			binary.LittleEndian.PutUint32(expected[address+8:], original|0x24)
			if !bytes.Equal(expected, session.Document.Snapshot().Data) {
				t.Fatal("classification flags were normalized")
			}
			for _, bit := range []int{-1, 22, 31, 32} {
				if err := session.Records.SetRecordTrait(
					records.Items,
					0,
					bit,
					false,
				); !errors.Is(
					err,
					edit.ErrField,
				) {
					t.Fatal("unconfirmed bit accepted", bit, err)
				}
				if !bytes.Equal(expected, session.Document.Snapshot().Data) {
					t.Fatal("failed trait change modified bytes")
				}
			}
			if version.ID == "FE6" {
				if err := session.Records.SetRecordTrait(
					records.Items,
					0,
					15,
					false,
				); !errors.Is(
					err,
					edit.ErrField,
				) {
					t.Fatal("FE6 critical-negation bit was invented", err)
				}
			} else {
				if err := session.Records.SetRecordTrait(
					records.Items,
					0,
					13,
					true,
				); err != nil {
					t.Fatal(err)
				}
				binary.LittleEndian.PutUint32(
					expected[address+8:],
					original|0x2024,
				)
				if !bytes.Equal(expected, session.Document.Snapshot().Data) {
					t.Fatal(
						"Prf display flag rewrote rank or actual equipment locks",
					)
				}
			}
			path := filepath.Join(t.TempDir(), "item-traits.gba")
			if err := session.Document.SaveAs(path, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(expected, reopened.Document.Snapshot().Data) {
				t.Fatal("saved item flags or unrelated data changed")
			}
			if err := reopened.Records.SetRecordTrait(
				records.Items,
				0,
				5,
				false,
			); err != nil {
				t.Fatal("continued trait editing", err)
			}
			if value, err := reopened.Records.Value(
				records.Items,
				0,
				"flags",
			); err != nil ||
				value != int64(
					binary.LittleEndian.Uint32(expected[address+8:])&^0x20,
				) {
				t.Fatal("continued editing lost preserved bits", value, err)
			}
		})
	}
}

func TestItemTraitsFollowManualLayout(t *testing.T) {
	fixture := testutil.TextFixtureFor(testutil.TextVersions[0])
	original := fixture.GetPosition("items")
	const manual = 0x318000
	fixture.WriteU32(original+8, 0xA0401001)
	fixture.WriteU32(manual+8, 0x80401001)
	session := testutil.Open(t, fixture)
	before := session.Document.Snapshot()
	if err := session.Records.UseLayout(
		records.Items,
		records.Layout{Address: manual, Stride: 36, Count: 2},
	); err != nil {
		t.Fatal(err)
	}
	if err := session.Records.SetRecordTrait(
		records.Items,
		0,
		5,
		true,
	); err != nil {
		t.Fatal(err)
	}
	expected := bytes.Clone(before.Data)
	binary.LittleEndian.PutUint32(expected[manual+8:], 0x80401021)
	if !bytes.Equal(expected, session.Document.Snapshot().Data) {
		t.Fatal("trait write ignored the selected manual layout")
	}
	if !session.Document.Undo() ||
		!bytes.Equal(before.Data, session.Document.Snapshot().Data) {
		t.Fatal("manual trait history")
	}
}
