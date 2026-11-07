package records

func useEffect(value int64, name string) ItemChoice {
	return ItemChoice{
		Value: value, Name: "item_use_effect_" + name,
		Help: "item_use_effect_scope",
	}
}

// FE6ItemUseEffects describes the native field vocabulary. Actual item use
// still follows item-ID-based engine dispatch; these are not handler pointers.
func FE6ItemUseEffects() []ItemChoice {
	return []ItemChoice{
		useEffect(0x00, "none"),
		useEffect(0x01, "binding_blade"),
		useEffect(0x02, "heal"),
		useEffect(0x03, "mend"),
		useEffect(0x04, "recover"),
		useEffect(0x05, "physic"),
		useEffect(0x06, "fortify"),
		useEffect(0x07, "warp"),
		useEffect(0x08, "rescue"),
		useEffect(0x09, "restore"),
		useEffect(0x0A, "silence"),
		useEffect(0x0B, "sleep"),
		useEffect(0x0C, "torch_staff"),
		useEffect(0x0D, "hammerne"),
		{
			Value: 0x0E,
			Name:  "item_use_effect_watch",
			Help:  "item_use_effect_unused",
		},
		useEffect(0x0F, "berserk"),
		useEffect(0x10, "unlock"),
		useEffect(0x11, "barrier"),
		useEffect(0x12, "secret_book"),
		useEffect(0x13, "goddess_icon"),
		useEffect(0x14, "angelic_robe"),
		useEffect(0x15, "dragonshield"),
		useEffect(0x16, "energy_ring"),
		useEffect(0x17, "speedwing"),
		useEffect(0x18, "talisman"),
		useEffect(0x19, "boots"),
		useEffect(0x1A, "body_ring"),
		useEffect(0x1B, "hero_crest"),
		useEffect(0x1C, "knight_crest"),
		useEffect(0x1D, "orions_bolt"),
		useEffect(0x1E, "elysian_whip"),
		useEffect(0x1F, "guiding_ring"),
		useEffect(0x20, "chest_key"),
		useEffect(0x21, "door_key"),
		useEffect(0x22, "bridge_key"),
		useEffect(0x23, "lockpick"),
		useEffect(0x24, "vulnerary"),
		useEffect(0x25, "elixir"),
		useEffect(0x26, "pure_water"),
		useEffect(0x27, "torch_item"),
		useEffect(0x28, "antitoxin"),
		useEffect(0x29, "tina_staff"),
	}
}

func laterItemUseEffects() []ItemChoice {
	return []ItemChoice{
		useEffect(0x00, "none"),
		useEffect(0x01, "heal"),
		useEffect(0x02, "mend"),
		useEffect(0x03, "recover"),
		useEffect(0x04, "physic"),
		useEffect(0x05, "fortify"),
		useEffect(0x06, "restore"),
		useEffect(0x07, "silence"),
		useEffect(0x08, "sleep"),
		useEffect(0x09, "berserk"),
		useEffect(0x0A, "warp"),
		useEffect(0x0B, "rescue"),
		useEffect(0x0C, "torch_staff"),
		useEffect(0x0D, "hammerne"),
		useEffect(0x0E, "unlock"),
		useEffect(0x0F, "barrier"),
		useEffect(0x10, "angelic_robe"),
		useEffect(0x11, "energy_ring"),
		useEffect(0x12, "secret_book"),
		useEffect(0x13, "speedwing"),
		useEffect(0x14, "goddess_icon"),
		useEffect(0x15, "dragonshield"),
		useEffect(0x16, "talisman"),
		useEffect(0x17, "boots"),
		useEffect(0x18, "body_ring"),
		useEffect(0x19, "hero_crest"),
		useEffect(0x1A, "knight_crest"),
		useEffect(0x1B, "orions_bolt"),
		useEffect(0x1C, "elysian_whip"),
		useEffect(0x1D, "guiding_ring"),
		useEffect(0x1E, "chest_key"),
		useEffect(0x1F, "door_key"),
		useEffect(0x20, "lockpick"),
		useEffect(0x21, "vulnerary"),
		useEffect(0x22, "elixir"),
		useEffect(0x23, "pure_water"),
		useEffect(0x24, "antitoxin"),
		useEffect(0x25, "torch_item"),
		useEffect(0x26, "chest_key"),
		useEffect(0x27, "mine"),
		useEffect(0x28, "light_rune"),
	}
}

// FE7ItemUseEffects excludes unconfirmed 33–35 entries from named choices.
func FE7ItemUseEffects() []ItemChoice {
	return append(laterItemUseEffects(),
		useEffect(0x29, "iron_rune"),
		useEffect(0x2A, "fillas_might"),
		useEffect(0x2B, "niniss_grace"),
		useEffect(0x2C, "thors_ire"),
		useEffect(0x2D, "sets_litany"),
		useEffect(0x2E, "earth_seal"),
		useEffect(0x2F, "afas_drops"),
		useEffect(0x30, "heaven_seal"),
		useEffect(0x31, "fell_contract"),
		useEffect(0x32, "ocean_seal"),
	)
}

// FE8ItemUseEffects retains FE8's shifted rings and distinct promotion items.
// Unconfirmed 2F/33–35 and patch-only Latona/skill-book codes stay custom.
func FE8ItemUseEffects() []ItemChoice {
	return append(laterItemUseEffects(),
		useEffect(0x29, "fillas_might"),
		useEffect(0x2A, "niniss_grace"),
		useEffect(0x2B, "thors_ire"),
		useEffect(0x2C, "sets_litany"),
		useEffect(0x2D, "master_seal"),
		useEffect(0x2E, "metis_tome"),
		useEffect(0x30, "ocean_seal"),
		useEffect(0x31, "lunar_brace"),
		useEffect(0x32, "solar_brace"),
		useEffect(0x36, "juna_fruit"),
	)
}

func weaponEffect(value int64, name string) ItemChoice {
	if value == 0 {
		return ItemChoice{Value: value, Name: "item_weapon_effect_none"}
	}
	return ItemChoice{
		Value: value,
		Name:  "item_weapon_effect_" + name,
		Help:  "item_weapon_effect_" + name + "_help",
	}
}

// FE6ItemWeaponEffects keeps FE6 Eclipse's leave-one-HP rule separate.
func FE6ItemWeaponEffects() []ItemChoice {
	return []ItemChoice{
		weaponEffect(0, "none"),
		weaponEffect(1, "poison"),
		weaponEffect(2, "drain"),
		weaponEffect(3, "leave_one_hp"),
		weaponEffect(4, "devil"),
	}
}

func FE7ItemWeaponEffects() []ItemChoice {
	return []ItemChoice{
		weaponEffect(0, "none"),
		weaponEffect(1, "poison"),
		weaponEffect(2, "drain"),
		weaponEffect(3, "halve_hp"),
		weaponEffect(4, "devil"),
	}
}

func FE8ItemWeaponEffects() []ItemChoice {
	return append(FE7ItemWeaponEffects(), weaponEffect(5, "petrify"))
}
