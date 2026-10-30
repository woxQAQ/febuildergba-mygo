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
	"github.com/woxQAQ/febuildergba-mygo/game/version"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestRecordAbilityWorkflow(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewRecordFixture(fixture.Name)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			initial, err := s.Records.ReferenceAbilities(
				records.Units,
				1,
				0,
				false,
			)
			if err != nil {
				t.Fatal(err)
			}
			expected := []records.AbilityValue{
				{Stat: records.StatHP, Hundredths: 1700},
				{Stat: records.StatPower, Hundredths: 400},
				{Stat: records.StatSkill, Hundredths: 0},
				{Stat: records.StatSpeed, Hundredths: 0},
				{Stat: records.StatDefense, Hundredths: 0},
				{Stat: records.StatResistance, Hundredths: 0},
				{Stat: records.StatLuck, Hundredths: 700},
				{Stat: records.StatConstitution, Hundredths: 400},
				{Stat: records.StatMovement, Hundredths: 600},
			}
			if initial.ClassID != 1 || !slices.Equal(initial.Values, expected) {
				t.Fatalf("native reference: %+v", initial)
			}
			estimated, err := s.Records.ReferenceAbilities(
				records.Units,
				1,
				5,
				true,
			)
			if err != nil {
				t.Fatal(err)
			}
			if estimated.GrowthSteps != 2 ||
				estimated.Values[0].Hundredths != 1850 {
				t.Fatal("exact growth reference")
			}
			class, err := s.Records.ReferenceAbilities(
				records.Classes,
				1,
				3,
				true,
			)
			if err != nil {
				t.Fatal(err)
			}
			if class.Values[0].Hundredths != 1985 ||
				slices.ContainsFunc(
					class.Values,
					func(value records.AbilityValue) bool { return value.Stat == records.StatLuck },
				) {
				t.Fatal("signed class growth or absent base")
			}
			for _, target := range []int{2, 256, -1} {
				if _, err := s.Records.ReferenceAbilities(
					records.Units,
					1,
					target,
					true,
				); !errors.Is(
					err,
					edit.ErrValue,
				) {
					t.Fatal("invalid level estimate", target, err)
				}
			}
			if !bytes.Equal(s.Document.Snapshot().Data, before.Data) ||
				s.Document.State().Dirty {
				t.Fatal("reference created ROM history")
			}
			for _, change := range []struct {
				kind   records.Kind
				field  string
				value  int64
				offset int
				raw    byte
			}{
				{records.Classes, "base.power", -128, f.GetPosition("record_class") + 12, 0x80},
				{records.Classes, "maximum.hp", 127, f.GetPosition("record_class") + 19, 0x7F},
				{records.Classes, "growth.hp", -10, f.GetPosition("record_class") + 27, 0xF6},
				{records.Classes, "experience", -128, f.GetPosition("record_class") + 26, 0x80},
				{records.Units, "level", -1, f.GetPosition("record_unit") + 11, 0xFF},
			} {
				if err := s.Records.Set(
					change.kind,
					[]int{1},
					change.field,
					change.value,
				); err != nil {
					t.Fatal(err)
				}
				if s.Document.Snapshot().Data[change.offset] != change.raw {
					t.Fatal("native signed field encoding")
				}
				if !s.Document.Undo() {
					t.Fatal("signed undo")
				}
				s.Refresh()
			}
			switch fixture.Name {
			case version.FE6:
				if err := s.Records.Set(
					records.Classes,
					[]int{1},
					"promotion.hp",
					200,
				); !errors.Is(
					err,
					edit.ErrField,
				) {
					t.Fatal("FE6 promotion field invented")
				}
			default:
				value, err := s.Records.Value(
					records.Classes,
					1,
					"promotion.hp",
				)
				if err != nil || value != 200 {
					t.Fatal("unsigned promotion byte")
				}
				if err := s.Records.Set(
					records.Classes,
					[]int{1},
					"promotion.hp",
					255,
				); err != nil {
					t.Fatal(err)
				}
				if s.Document.Snapshot().Data[f.GetPosition("record_class")+34] != 0xFF {
					t.Fatal("promotion byte encoding")
				}
				s.Document.Undo()
				s.Refresh()
			}
			if err := s.Records.Set(
				records.Units,
				[]int{1},
				"class_id",
				2,
			); err != nil {
				t.Fatal(err)
			}
			changed, err := s.Records.ReferenceAbilities(
				records.Units,
				1,
				0,
				false,
			)
			if err != nil || changed.Values[0].Hundredths != 2700 ||
				changed.ClassID != 2 {
				t.Fatal("reference did not follow class")
			}
			output := filepath.Join(t.TempDir(), "record-reference.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			again, err := reopened.Records.ReferenceAbilities(
				records.Units,
				1,
				5,
				true,
			)
			if err != nil || again.Values[0].Hundredths != 2850 {
				t.Fatal("reference after save/reopen")
			}
		})
	}
}

func TestRecordTraitsAndWeapons(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewRecordFixture(fixture.Name)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			if err := s.Records.SetRecordTrait(
				records.Units,
				1,
				1,
				true,
			); err != nil {
				t.Fatal(err)
			}
			expected := bytes.Clone(before.Data)
			expected[f.GetPosition("record_unit")+40] |= 2
			if !bytes.Equal(expected, s.Document.Snapshot().Data) {
				t.Fatal("trait changed unknown or adjacent bits")
			}
			stable := s.Document.Snapshot()
			if err := s.Records.SetRecordTrait(
				records.Units,
				1,
				23,
				false,
			); !errors.Is(err, edit.ErrField) ||
				!bytes.Equal(stable.Data, s.Document.Snapshot().Data) {
				t.Fatal("unknown trait editable")
			}
			traits := s.Records.RecordTraits(records.Classes)
			switch fixture.Name {
			case "FE6":
				if traits[31].Known ||
					traits[19].Label != "trait_zephiel_lock" {
					t.Fatal("FE6 traits")
				}
			case "FE7U", "FE7J":
				if traits[27].Label != "trait_drop_item" {
					t.Fatal("FE7 traits")
				}
			default:
				if traits[27].Label != "trait_summon" {
					t.Fatal("FE8 traits")
				}
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("trait atomic undo")
			}
			s.Refresh()
			if !s.Document.Redo() {
				t.Fatal("trait redo")
			}
			s.Refresh()
			skills, err := s.Records.WeaponSkills(records.Units, 1)
			if err != nil {
				t.Fatal(err)
			}
			rank := "C"
			if fixture.Name == "FE6" {
				rank = "D"
			}
			if !skills[0].Inherited || skills[0].Value != 0 ||
				skills[0].Effective != 95 ||
				skills[0].EffectiveRank != rank {
				t.Fatal("weapon inheritance")
			}
			if err := s.Records.Set(
				records.Units,
				[]int{1},
				"weapon.sword",
				41,
			); err != nil {
				t.Fatal(err)
			}
			skills, err = s.Records.WeaponSkills(records.Units, 1)
			if err != nil || skills[0].Inherited || skills[0].Effective != 41 {
				t.Fatal("character EXP did not replace class EXP")
			}
			revision := s.Document.State().Revision
			if err := s.Records.SetWeaponRank(
				records.Units,
				1,
				"weapon.sword",
				skills[0].Rank,
			); err != nil ||
				s.Document.State().Revision != revision {
				t.Fatal("same rank reset partial progress")
			}
			minimum := int64(181)
			if fixture.Name == "FE6" {
				minimum = 201
			}
			if err := s.Records.SetWeaponRank(
				records.Units,
				1,
				"weapon.sword",
				"A",
			); err != nil {
				t.Fatal(err)
			}
			if value, _ := s.Records.Value(
				records.Units,
				1,
				"weapon.sword",
			); value != minimum {
				t.Fatal("version rank minimum")
			}
			if err := s.Records.SetWeaponRank(
				records.Units,
				1,
				"weapon.sword",
				"none",
			); err != nil {
				t.Fatal(err)
			}
			skills, _ = s.Records.WeaponSkills(records.Units, 1)
			if !skills[0].Inherited || skills[0].Effective != 95 {
				t.Fatal("explicit inherit")
			}
			if err := s.Records.Set(
				records.Units,
				[]int{1},
				"class_id",
				255,
			); err != nil {
				t.Fatal(err)
			}
			skills, err = s.Records.WeaponSkills(records.Units, 1)
			if err != nil || skills[0].ReferenceError == nil {
				t.Fatal("invalid class silently treated as inherited zero")
			}
			if err := s.Records.SetWeaponRank(
				records.Units,
				1,
				"weapon.sword",
				"D",
			); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(t.TempDir(), "record-weapons.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Records.SetRecordTrait(
				records.Units,
				1,
				1,
				false,
			); err != nil {
				t.Fatal(err)
			}
			if value, _ := reopened.Records.Value(
				records.Units,
				1,
				"flags",
			); value != 0x80800080 {
				t.Fatal("trait save/reopen unknown bits")
			}
		})
	}
}

func TestRecordLayoutsAndResources(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewRecordFixture(fixture.Name)
			s := testutil.Open(t, f)
			links, err := s.Records.RecordResources(records.Classes, 1)
			if err != nil {
				t.Fatal(err)
			}
			count := 7
			if fixture.Name == "FE6" {
				count = 5
			}
			if len(links) != count || links[0].Address != 0x261000 ||
				links[1].Address != 0x262000 ||
				links[0].Err != nil {
				t.Fatal("native resource pointers")
			}
			if !links[2].Empty {
				t.Fatal("zero resource not distinct")
			}
			stable := s.Document.Snapshot()
			if _, err := s.Records.RecordResources(
				records.Units,
				1,
			); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(stable.Data, s.Document.Snapshot().Data) {
				t.Fatal("resource browsing wrote document")
			}
			move, err := s.Records.PlanTableMove(records.Classes, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Records.ApplyTableMove(move); err != nil {
				t.Fatal(err)
			}
			after, err := s.Records.RecordResources(records.Classes, 1)
			if err != nil || after[0].Address != links[0].Address ||
				after[0].Pointer == links[0].Pointer {
				t.Fatal("resources ignored relocated table")
			}
			reference, err := s.Records.ReferenceAbilities(
				records.Units,
				1,
				0,
				false,
			)
			if err != nil || reference.Values[0].Hundredths != 1700 {
				t.Fatal("reference ignored relocated table")
			}
			table := s.Records.Table(records.Classes)
			if err := s.Records.UseLayout(
				records.Classes,
				records.Layout{
					Address: table.Address + table.Stride,
					Stride:  table.Stride,
					Count:   1,
				},
			); err != nil {
				t.Fatal(err)
			}
			// The selected manual row now has ID 0; the character's class 1 is absent.
			if _, err := s.Records.ReferenceAbilities(
				records.Units,
				1,
				0,
				false,
			); err == nil {
				t.Fatal("reference bypassed manual class layout")
			}
			if _, err := s.Records.RecordResources(
				records.Items,
				0,
			); !errors.Is(
				err,
				edit.ErrField,
			) {
				t.Fatal("unsupported resource owner")
			}
		})
	}
}
