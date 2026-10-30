package records

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/version"
)

// RecordTrait describes one native attribute bit. Labels and help are locale
// keys; unknown bits retain their encoding and have no structured write action.
type RecordTrait struct {
	Bit         int
	Label, Help string
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
	if kind != Units && kind != Classes {
		return nil
	}
	names := slices.Clone(commonTraits)
	switch s.definition.Version {
	case version.FE6:
		names[16], names[18], names[19] = "roy_lock", "dragon_lock", "zephiel_lock"
		for bit := 25; bit < 32; bit++ {
			names[bit] = unknownTrait
		}
	case version.FE7U, version.FE7J:
		names[18], names[19], names[27] = "dragon_lock", "morph", "drop_item"
		names[28], names[29] = "eliwood_lock", "hector_lock"
	}
	result := make([]RecordTrait, len(names))
	for bit, name := range names {
		help := "trait_" + name + "_help"
		if bit == 6 && s.definition.Version == version.FE6 {
			help = "trait_critical_fe6_help"
		}
		result[bit] = RecordTrait{
			Bit:   bit,
			Label: "trait_" + name,
			Help:  help,
			Known: name != unknownTrait,
		}
	}
	return result
}

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
