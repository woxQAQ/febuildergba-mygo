package records

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
)

// RecordTrait describes one native attribute bit. Labels and help are locale
// keys; unknown bits retain their encoding and have no structured write action.
type RecordTrait struct {
	Bit         int
	Label, Help string
	Group       string
	Known       bool
}

const unknownTrait = "unknown"

var commonTraits = []string{
	"mounted_aid",
	"canto",
	"steal",
	"thief",
	"dance",
	"play",
	"critical",
	"ballista",
	"promoted",
	"supply",
	"cavalry_icon",
	"wyvern_icon",
	"pegasus_icon",
	"lord",
	"female",
	"boss",
	"weapon_lock_1",
	"sword_lock",
	"monster_lock",
	"max_level_10",
	"unselectable",
	"triangle_pegasus",
	"triangle_armor",
	unknownTrait,
	"no_experience",
	"lethality",
	"magic_seal",
	"summon",
	"eirika_lock",
	"ephraim_lock",
	"lyn_lock",
	"athos_lock",
}

func (s *Editor) RecordTraits(kind Kind) []RecordTrait {
	s.Refresh()
	switch kind {
	case Units, Classes:
		return slices.Clone(s.definition.Traits)
	case Items:
		return slices.Clone(s.definition.ItemTraits)
	default:
		return nil
	}
}

func commonTraitTable() []RecordTrait {
	result := make([]RecordTrait, len(commonTraits))
	for bit, name := range commonTraits {
		result[bit] = RecordTrait{
			Bit:   bit,
			Label: "trait_" + name,
			Help:  "trait_" + name + "_help",
			Known: name != unknownTrait,
		}
	}
	return result
}

func renameTraits(table []RecordTrait, names map[int]string) {
	for bit, name := range names {
		table[bit].Label = "trait_" + name
		table[bit].Help = "trait_" + name + "_help"
		table[bit].Known = name != unknownTrait
	}
}

// FE6Traits is the FE6 flag-bit vocabulary, including its distinct critical
// help and the absent FE8 lock bits.
func FE6Traits() []RecordTrait {
	result := commonTraitTable()
	renameTraits(result, map[int]string{
		16: "roy_lock", 18: "dragon_lock", 19: "zephiel_lock",
		25: unknownTrait, 26: unknownTrait, 27: unknownTrait, 28: unknownTrait,
		29: unknownTrait, 30: unknownTrait, 31: unknownTrait,
	})
	result[6].Help = "trait_critical_fe6_help"
	return result
}

// FE7Traits is the FE7 flag-bit vocabulary.
func FE7Traits() []RecordTrait {
	result := commonTraitTable()
	renameTraits(result, map[int]string{
		18: "dragon_lock", 19: "morph", 27: "drop_item",
		28: "eliwood_lock", 29: "hector_lock",
	})
	return result
}

// FE8Traits is the shared FE8 flag-bit vocabulary.
func FE8Traits() []RecordTrait { return commonTraitTable() }

// SetRecordTrait changes a confirmed bit and preserves the remaining 31 bits.
func (s *Editor) SetRecordTrait(kind Kind, id, bit int, on bool) error {
	s.Refresh()
	traits := s.RecordTraits(kind)
	if bit < 0 || bit >= len(traits) || !traits[bit].Known {
		return edit.ErrField
	}
	flags, err := s.Value(kind, id, "flags")
	if err != nil {
		return err
	}
	mask := int64(1) << bit
	if on {
		flags |= mask
	} else {
		flags &^= mask
	}
	return s.Set(kind, []int{id}, "flags", flags)
}
