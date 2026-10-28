//go:build integration

package integration

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
)

func TestRealRecordEditing(t *testing.T) {
	samples := map[string]struct {
		address, class, sword, classHP, classSword int
		estimatedHP                                int64
	}{
		"FE8U": {0x803D64, 2, 1, 23, 31, 2930},
		"FE8J": {0x8582BC, 2, 1, 23, 31, 2930},
		"FE7U": {0xBDCE4C, 1, 71, 18, 31, 3320},
		"FE7J": {0xC4C158, 1, 71, 18, 31, 3320},
		"FE6":  {0x6076D0, 1, 0, 15, 51, 3320},
	}
	for _, fixture := range realROMFixtures {
		t.Run(fixture.id, func(t *testing.T) {
			s := openRealROM(t, fixture.filename)
			sample := samples[fixture.id]
			if s.Table(game.Units).Rows[0].Address != sample.address {
				t.Fatal("fixed character address")
			}
			class, err := s.Value(game.Units, 1, "class_id")
			if err != nil || int(class) != sample.class {
				t.Fatal("fixed native class")
			}
			sword, err := s.Value(game.Units, 1, "weapon.sword")
			if err != nil || int(sword) != sample.sword {
				t.Fatal("fixed character weapon EXP")
			}
			hp, err := s.Value(game.Classes, 7, "base.hp")
			if err != nil || int(hp) != sample.classHP {
				t.Fatal("fixed native class base")
			}
			rank, err := s.Value(game.Classes, 7, "weapon.sword")
			if err != nil || int(rank) != sample.classSword {
				t.Fatal("fixed class weapon EXP")
			}
			portraitID, err := s.Value(game.Units, 1, "portrait_id")
			if err != nil {
				t.Fatal(err)
			}
			portrait, err := s.PortraitEntry(int(portraitID))
			if err != nil || portrait.Empty || portrait.ClassCard {
				t.Fatal("native portrait table reference", err)
			}
			cardID, err := s.Value(game.Classes, 7, "portrait_id")
			if err != nil {
				t.Fatal(err)
			}
			card, err := s.PortraitEntry(int(cardID))
			if err != nil || card.Empty {
				t.Fatal("native class image table reference", err)
			}
			before := s.Document.Snapshot()
			estimated, err := s.ReferenceAbilities(game.Units, 1, 20, true)
			if err != nil ||
				estimated.Values[0].Hundredths != sample.estimatedHP {
				t.Fatal("fixed native growth reference", err)
			}
			if !bytes.Equal(s.Document.Snapshot().Data, before.Data) ||
				s.Document.State().Dirty {
				t.Fatal("reference modified ROM")
			}
			if err := s.SetRecordTrait(game.Units, 1, 1, true); err != nil {
				t.Fatal(err)
			}
			expected := bytes.Clone(before.Data)
			expected[sample.address+40] |= 2
			if !bytes.Equal(expected, s.Document.Snapshot().Data) {
				t.Fatal("trait changed adjacent data")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("trait undo")
			}
			s.Refresh()
			if !s.Document.Redo() {
				t.Fatal("trait redo")
			}
			s.Refresh()
			if err := s.Set(
				game.Classes,
				[]int{7},
				"base.power",
				-1,
			); err != nil {
				t.Fatal(err)
			}
			classAddress := s.Table(game.Classes).Rows[7].Address
			if s.Document.Snapshot().Data[classAddress+12] != 0xFF {
				t.Fatal("signed native class write")
			}
			if err := s.SetWeaponRank(
				game.Units,
				1,
				"weapon.sword",
				"A",
			); err != nil {
				t.Fatal(err)
			}
			minimum := int64(181)
			if fixture.id == "FE6" {
				minimum = 201
			}
			if value, _ := s.Value(
				game.Units,
				1,
				"weapon.sword",
			); value != minimum {
				t.Fatal("native rank minimum")
			}
			links, err := s.RecordResources(game.Classes, 7)
			if err != nil || len(links) < 5 || links[0].Err != nil ||
				links[1].Err != nil {
				t.Fatal("native resource navigation", err)
			}
			output := filepath.Join(t.TempDir(), "record-edit.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			if value, _ := reopened.Value(
				game.Classes,
				7,
				"base.power",
			); value != -1 {
				t.Fatal("signed field after save")
			}
			skills, err := reopened.WeaponSkills(game.Units, 1)
			if err != nil || skills[0].Rank != "A" ||
				skills[0].Value != minimum {
				t.Fatal("weapon rank after save")
			}
			if err := reopened.SetWeaponRank(
				game.Units,
				1,
				"weapon.sword",
				"none",
			); err != nil {
				t.Fatal(err)
			}
			skills, err = reopened.WeaponSkills(game.Units, 1)
			if err != nil || !skills[0].Inherited {
				t.Fatal("continued inheritance editing")
			}
		})
	}
}
