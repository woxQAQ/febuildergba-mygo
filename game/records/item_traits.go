package records

// Item trait groups describe use rules, battle behavior and equipment locks.
const (
	ItemTraitUsage        = "item_traits_usage"
	ItemTraitCombat       = "item_traits_combat"
	ItemTraitRestrictions = "item_traits_restrictions"
)

func itemTrait(bit int, name, group string) RecordTrait {
	return RecordTrait{
		Bit: bit, Group: group, Known: true,
		Label: "item_trait_" + name,
		Help:  "item_trait_" + name + "_help",
	}
}

func commonItemTraits() []RecordTrait {
	traits := make([]RecordTrait, 32)
	for bit := range traits {
		traits[bit] = RecordTrait{
			Bit: bit, Label: "trait_unknown", Help: "trait_unknown_help",
		}
	}
	for _, trait := range []RecordTrait{
		itemTrait(0, "weapon", ItemTraitUsage),
		itemTrait(1, "magic", ItemTraitUsage),
		itemTrait(2, "staff", ItemTraitUsage),
		itemTrait(3, "unbreakable", ItemTraitUsage),
		itemTrait(4, "unsellable", ItemTraitUsage),
		itemTrait(5, "brave", ItemTraitCombat),
		itemTrait(6, "magic_sword", ItemTraitCombat),
		itemTrait(7, "no_counter", ItemTraitCombat),
		itemTrait(8, "reaver", ItemTraitCombat),
		itemTrait(9, "unrepairable", ItemTraitUsage),
		itemTrait(12, "sword_lock", ItemTraitRestrictions),
		itemTrait(14, "negate_flying", ItemTraitCombat),
	} {
		traits[trait.Bit] = trait
	}
	return traits
}

// FE6ItemTraits retains FE6's character locks and leaves bits 15–31 unknown.
func FE6ItemTraits() []RecordTrait {
	traits := commonItemTraits()
	traits[10] = itemTrait(10, "dragon_lock", ItemTraitRestrictions)
	traits[11] = itemTrait(11, "roy_lock", ItemTraitRestrictions)
	traits[13] = itemTrait(13, "zephiel_lock", ItemTraitRestrictions)
	return traits
}

// FE7ItemTraits models Prf display separately from actual equipment locks.
// Bit 16 uses the engine's per-character unsealing check.
func FE7ItemTraits() []RecordTrait {
	traits := commonItemTraits()
	for _, trait := range []RecordTrait{
		itemTrait(10, "dragon_lock", ItemTraitRestrictions),
		itemTrait(11, "weapon_lock_1", ItemTraitRestrictions),
		itemTrait(13, "display_prf", ItemTraitRestrictions),
		itemTrait(15, "negate_critical", ItemTraitCombat),
		itemTrait(16, "sealed", ItemTraitRestrictions),
		itemTrait(17, "negate_defense", ItemTraitCombat),
		itemTrait(18, "eliwood_lock", ItemTraitRestrictions),
		itemTrait(19, "hector_lock", ItemTraitRestrictions),
		itemTrait(20, "lyn_lock", ItemTraitRestrictions),
		itemTrait(21, "athos_lock", ItemTraitRestrictions),
	} {
		traits[trait.Bit] = trait
	}
	return traits
}

// FE8ItemTraits replaces the FE7 character locks with FE8's native meanings.
// The last two locks are consumed by the engine but unused by ordinary items.
func FE8ItemTraits() []RecordTrait {
	traits := FE7ItemTraits()
	traits[10] = itemTrait(10, "monster_lock", ItemTraitRestrictions)
	traits[18] = itemTrait(18, "eirika_lock", ItemTraitRestrictions)
	traits[19] = itemTrait(19, "ephraim_lock", ItemTraitRestrictions)
	traits[20] = itemTrait(20, "weapon_lock_6", ItemTraitRestrictions)
	traits[21] = itemTrait(21, "weapon_lock_7", ItemTraitRestrictions)
	return traits
}
