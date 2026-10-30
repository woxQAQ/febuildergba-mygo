package records

import (
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/game/version"
)

// Field groups describe presentation-independent sections of a record schema.
const (
	GroupGeneral   = "general"
	GroupBase      = "base"
	GroupGrowth    = "growth"
	GroupMaximum   = "maximum"
	GroupPromotion = "promotion"
	GroupCombat    = "combat"
	GroupWeapon    = "weapon"
	GroupAdvanced  = "advanced"
)

const (
	StatHP           = "hp"
	StatPower        = "power"
	StatMagic        = "magic"
	StatSkill        = "skill"
	StatSpeed        = "speed"
	StatLuck         = "luck"
	StatDefense      = "defense"
	StatResistance   = "resistance"
	StatConstitution = "constitution"
	StatMovement     = "movement"
)

func NumericField(id, group string, offset, width int) Field {
	return Field{ID: id, Group: group, Offset: offset, Width: width}
}

func signedField(id, group string, offset int) Field {
	f := NumericField(id, group, offset, 1)
	f.Signed = true
	return f
}

func assetField(id string, offset, width int, kind graphics.Kind) Field {
	f := NumericField(id, GroupGeneral, offset, width)
	f.Asset = kind
	return f
}

func reference(id string, offset int, kind Kind) Field {
	f := NumericField(id, GroupGeneral, offset, 1)
	f.Reference = kind
	return f
}

func stats(group string, offset int, names []string, signed bool) []Field {
	var fields []Field
	for i, name := range names {
		f := NumericField(group+"."+name, group, offset+i, 1)
		f.Signed = signed
		fields = append(fields, f)
	}
	return fields
}

func Flags(
	offset int,
) Field {
	f := NumericField("flags", GroupAdvanced, offset, 4)
	f.Hex = true
	return f
}

func common() []Field {
	name := NumericField("name_id", GroupGeneral, 0, 2)
	name.Text = text.Name
	description := NumericField("description_id", GroupGeneral, 2, 2)
	description.Text = text.Description
	return []Field{name, description}
}

var WeaponNames = []string{
	"sword",
	"lance",
	"axe",
	"bow",
	"staff",
	"anima",
	"light",
	"dark",
}

func UnitFields(versionID string) []Field {
	f := append(
		common(),
		NumericField("id", GroupGeneral, 4, 1),
		reference("class_id", 5, Classes),
		assetField("portrait_id", 6, 2, graphics.Portrait),
		NumericField("mini_portrait", GroupGeneral, 8, 1),
		NumericField("affinity", GroupGeneral, 9, 1),
		NumericField("sort_order", GroupGeneral, 10, 1),
		signedField("level", GroupBase, 11),
	)
	f = append(
		f,
		stats(
			GroupBase,
			12,
			[]string{
				StatHP,
				StatPower,
				StatSkill,
				StatSpeed,
				StatDefense,
				StatResistance,
				StatLuck,
				StatConstitution,
			},
			true,
		)...)
	f = append(f, stats(GroupWeapon, 20, WeaponNames, false)...)
	f = append(
		f,
		stats(
			GroupGrowth,
			28,
			[]string{
				StatHP,
				StatPower,
				StatSkill,
				StatSpeed,
				StatDefense,
				StatResistance,
				StatLuck,
			},
			false,
		)...)
	if versionID == version.FE8U || versionID == version.FE8J {
		f = append(f, NumericField("visit_group", GroupGeneral, 48, 1))
	}
	if versionID == version.FE7U || versionID == version.FE7J {
		f = append(f, NumericField("talk_tone", GroupGeneral, 48, 1))
	}
	if versionID == version.FE6 || versionID == version.FE7U ||
		versionID == version.FE7J {
		f = append(
			f,
			NumericField("animation_palette_a", GroupGeneral, 35, 1),
			NumericField("animation_palette_b", GroupGeneral, 36, 1),
		)
	}
	return append(f, Flags(40))
}

func classBaseFields() []Field {
	f := append(
		common(),
		NumericField("id", GroupGeneral, 4, 1),
		reference("promotion_id", 5, Classes),
		NumericField("map_sprite", GroupGeneral, 6, 1),
		NumericField("movement_speed", GroupGeneral, 7, 1),
		assetField("portrait_id", 8, 2, graphics.Portrait),
		NumericField("sort_order", GroupGeneral, 10, 1),
	)
	f = append(
		f,
		stats(
			GroupBase,
			11,
			[]string{
				StatHP,
				StatPower,
				StatSkill,
				StatSpeed,
				StatDefense,
				StatResistance,
				StatConstitution,
				StatMovement,
			},
			true,
		)...)
	f = append(
		f,
		stats(
			GroupMaximum,
			19,
			[]string{
				StatHP,
				StatPower,
				StatSkill,
				StatSpeed,
				StatDefense,
				StatResistance,
				StatConstitution,
			},
			true,
		)...)
	f = append(f, signedField("experience", GroupGeneral, 26))
	return append(
		f,
		stats(
			GroupGrowth,
			27,
			[]string{
				StatHP,
				StatPower,
				StatSkill,
				StatSpeed,
				StatDefense,
				StatResistance,
				StatLuck,
			},
			true,
		)...)
}

func ClassFields() []Field {
	f := classBaseFields()
	f = append(
		f,
		stats(
			GroupPromotion,
			34,
			[]string{
				StatHP,
				StatPower,
				StatSkill,
				StatSpeed,
				StatDefense,
				StatResistance,
			},
			false,
		)...)
	f = append(f, stats(GroupWeapon, 44, WeaponNames, false)...)
	return append(f, Flags(40))
}

func itemBaseFields() []Field {
	use := NumericField("use_description_id", GroupGeneral, 4, 2)
	use.Text = text.Description
	f := append(
		common(),
		use,
		NumericField("id", GroupGeneral, 6, 1),
		NumericField("weapon_type", GroupGeneral, 7, 1),
	)
	f = append(
		f,
		stats(
			GroupCombat,
			20,
			[]string{"uses", "might", "hit", "weight", "critical", "range"},
			false,
		)...)
	f = append(
		f,
		NumericField("price", GroupCombat, 26, 2),
		NumericField("weapon_rank", GroupCombat, 28, 1),
		assetField("icon_id", 29, 1, graphics.ItemIcon),
		NumericField("use_effect", GroupCombat, 30, 1),
		NumericField("weapon_effect", GroupCombat, 31, 1),
	)
	return f
}

func ItemFields() []Field {
	return append(
		itemBaseFields(),
		NumericField("weapon_experience", GroupCombat, 32, 1),
		Flags(8),
	)
}

func FE6ItemFields() []Field {
	return append(itemBaseFields(), Flags(8))
}

func FE6ClassFields() []Field {
	f := append(
		classBaseFields(),
		stats(GroupWeapon, 40, WeaponNames, false)...)
	return append(f, Flags(36))
}
