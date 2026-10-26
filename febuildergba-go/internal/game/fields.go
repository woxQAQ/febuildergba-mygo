// Package game decodes supported ROM layouts and applies schema-checked edits.
package game

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

func field(id, group string, offset, width int) Field {
	return Field{ID: id, Group: group, Offset: offset, Width: width}
}

func assetField(id string, offset, width int, kind AssetKind) Field {
	f := field(id, GroupGeneral, offset, width)
	f.Asset = kind
	return f
}

func reference(id string, offset int, kind Kind) Field {
	f := field(id, GroupGeneral, offset, 1)
	f.Reference = kind
	return f
}

func stats(group string, offset int, names []string, signed bool) []Field {
	var fields []Field
	for i, name := range names {
		f := field(group+"."+name, group, offset+i, 1)
		f.Signed = signed
		fields = append(fields, f)
	}
	return fields
}

func flags(offset int) Field { f := field("flags", GroupAdvanced, offset, 4); f.Hex = true; return f }

func common() []Field {
	return []Field{field("name_id", GroupGeneral, 0, 2), field("description_id", GroupGeneral, 2, 2)}
}

var weaponNames = []string{"sword", "lance", "axe", "bow", "staff", "anima", "light", "dark"}

func unitFields() []Field {
	f := append(common(), field("id", GroupGeneral, 4, 1), reference("class_id", 5, Classes), assetField("portrait_id", 6, 2, PortraitAsset), field("mini_portrait", GroupGeneral, 8, 1), field("affinity", GroupGeneral, 9, 1), field("sort_order", GroupGeneral, 10, 1), field("level", GroupBase, 11, 1))
	f = append(f, stats(GroupBase, 12, []string{StatHP, StatPower, StatSkill, StatSpeed, StatDefense, StatResistance, StatLuck, StatConstitution}, true)...)
	f = append(f, stats(GroupWeapon, 20, weaponNames, false)...)
	f = append(f, stats(GroupGrowth, 28, []string{StatHP, StatPower, StatSkill, StatSpeed, StatDefense, StatResistance, StatLuck}, false)...)
	return append(f, flags(40))
}

func classBaseFields() []Field {
	f := append(common(), field("id", GroupGeneral, 4, 1), reference("promotion_id", 5, Classes), field("map_sprite", GroupGeneral, 6, 1), field("movement_speed", GroupGeneral, 7, 1), assetField("portrait_id", 8, 2, PortraitAsset), field("sort_order", GroupGeneral, 10, 1))
	f = append(f, stats(GroupBase, 11, []string{StatHP, StatPower, StatSkill, StatSpeed, StatDefense, StatResistance, StatConstitution, StatMovement}, false)...)
	f = append(f, stats(GroupMaximum, 19, []string{StatHP, StatPower, StatSkill, StatSpeed, StatDefense, StatResistance, StatConstitution}, false)...)
	f = append(f, field("experience", GroupGeneral, 26, 1))
	return append(f, stats(GroupGrowth, 27, []string{StatHP, StatPower, StatSkill, StatSpeed, StatDefense, StatResistance, StatLuck}, false)...)
}

func classFields() []Field {
	f := classBaseFields()
	f = append(f, stats(GroupPromotion, 34, []string{StatHP, StatPower, StatSkill, StatSpeed, StatDefense, StatResistance}, true)...)
	f = append(f, stats(GroupWeapon, 44, weaponNames, false)...)
	return append(f, flags(40))
}

func itemBaseFields() []Field {
	f := append(common(), field("use_description_id", GroupGeneral, 4, 2), field("id", GroupGeneral, 6, 1), field("weapon_type", GroupGeneral, 7, 1))
	f = append(f, stats(GroupCombat, 20, []string{"uses", "might", "hit", "weight", "critical", "range"}, false)...)
	f = append(f, field("price", GroupCombat, 26, 2), field("weapon_rank", GroupCombat, 28, 1), assetField("icon_id", 29, 1, ItemIconAsset), field("use_effect", GroupCombat, 30, 1), field("weapon_effect", GroupCombat, 31, 1))
	return f
}

func itemFields() []Field {
	return append(itemBaseFields(), field("weapon_experience", GroupCombat, 32, 1), flags(8))
}

func fe6ItemFields() []Field {
	return append(itemBaseFields(), flags(8))
}

func fe6ClassFields() []Field {
	f := append(classBaseFields(), stats(GroupWeapon, 40, weaponNames, false)...)
	return append(f, flags(36))
}
