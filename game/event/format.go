package event

import (
	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

const HeaderField = "chapter_events_id"

type deploymentSlot struct {
	label           string
	offset, faction int
}

// Layout is the native event structure of one generation: unit record shape,
// header offsets and deployment slots. Addresses stay in the version files.
type Layout struct {
	units           deployment.Format
	opening, ending int
	conditions      conditionLayout
	locations       locationLayout
	scripts         *scriptSet
	slots           []deploymentSlot
}

func FE8Layout() Layout {
	return Layout{
		units:   deployment.Format{Stride: 20},
		opening: 72,
		ending:  76,
		conditions: conditionLayout{
			talkKind: 3, talkStride: 16, turnStride: 12, talkModeWidth: 2,
			triggerWidth: 4, routKind: 1, routFlag: 6, reversedArea: true,
			talkRequired: []int{9},
		},
		locations: locationLayout{
			visit:          0x10,
			seize:          0x11,
			chest:          0x14,
			door:           0x12,
			center:         0x20,
			shop:           0x16,
			goldItem:       0x77,
			goldMinimum:    256,
			itemMaximum:    255,
			commandWidth:   2,
			indirectChest:  RandomChest,
			indirectTarget: RandomRewardTarget,
		},
		scripts: fe8Scripts(),
		slots: []deploymentSlot{
			{"deploy_player_normal", 40, 0},
			{"deploy_player_hard", 44, 0},
			{"deploy_skirmish_player_1", 48, 0},
			{"deploy_skirmish_player_2", 52, 0},
			{"deploy_skirmish_player_3", 56, 0},
			{"deploy_skirmish_enemy_1", 60, 2},
			{"deploy_skirmish_enemy_2", 64, 2},
			{"deploy_skirmish_enemy_3", 68, 2},
		},
	}
}

func fe7Layout(scripts *scriptSet) Layout {
	return Layout{
		units:   deployment.Format{Stride: 16},
		opening: 56,
		ending:  60,
		conditions: conditionLayout{
			talkKind: 3, talkStride: 16, turnStride: 16, talkModeWidth: 1,
			triggerWidth: 2, routKind: 14, talkRequired: []int{9},
		},
		locations: wordLocations(0x76),
		scripts:   scripts,
		slots: []deploymentSlot{
			{"deploy_eliwood_enemy_normal", 24, 2},
			{"deploy_eliwood_enemy_hard", 28, 2},
			{"deploy_hector_enemy_normal", 32, 2},
			{"deploy_hector_enemy_hard", 36, 2},
			{"deploy_eliwood_player_normal", 40, 0},
			{"deploy_eliwood_player_hard", 44, 0},
			{"deploy_hector_player_normal", 48, 0},
			{"deploy_hector_player_hard", 52, 0},
		},
	}
}

func FE7JLayout() Layout { return fe7Layout(fe7Scripts(0, 0xCC, 0xAD, fe7jInstructionWords)) }

func FE7ULayout() Layout { return fe7Layout(fe7Scripts(2, 0xCB, 0xAC, fe7uInstructionWords)) }

func FE6Layout() Layout {
	return Layout{
		units:   deployment.Format{Stride: 16},
		opening: -1,
		ending:  24,
		conditions: conditionLayout{
			talkKind: 4, talkStride: 12, turnStride: 12,
			triggerWidth: 2, routKind: 13, talkRequired: []int{8, 9},
		},
		locations: wordLocations(0x6F),
		scripts:   fe6Scripts(),
		slots: []deploymentSlot{
			{"deploy_enemy", 16, 2}, {"deploy_player", 20, 0},
		},
	}
}

// Format combines a generation's layout with the version's confirmed
// addresses: the event list root, the load helper, and the defeat/rout code
// sites used by native conditions.
type Format struct {
	Layout

	Root           int
	LoadHelper     int
	Defeat, Rout   int
	WorldMapEvents WorldMapEventsFormat
}

// Opening is the header offset of the opening script pointer, or -1 where the
// version has none.
func (f Format) Opening() int { return f.opening }

// EndingOffset is the header offset of the ending script pointer.
func (f Format) EndingOffset() int { return f.ending }

// The owner identities below are shared with the chapter coordinator, which
// declares the matching claims; writers pass them to the write guard. Chapter
// resources are identified by their native pointer site, so chapters aliasing
// one event list declare identical claims instead of conflicting.

// RootOwner owns the chapter event list root pointer.
func RootOwner() romwrite.Owner { return romwrite.Owner{Resource: "events", ID: -1} }

// HeaderOwner owns one event header. The identity is a native position: the
// header address for the header itself, or the entry position for the event
// list entry when the header is unreadable.
func HeaderOwner(header int) romwrite.Owner {
	return romwrite.Owner{Resource: "events", ID: header}
}

// ConditionsOwner owns one condition table pointer and payload.
func ConditionsOwner(site int, table Table) romwrite.Owner {
	return romwrite.Owner{
		Resource: "event.conditions",
		ID:       site,
		Slot:     int(table),
	}
}

// ScriptCodeOwner owns decoded instruction bytes across shared and interior
// entries. Entry pointers retain independent owners for relocation.
func ScriptCodeOwner() romwrite.Owner {
	return romwrite.Owner{Resource: "event.script_code"}
}

// ScriptOwner owns one script pointer site.
func ScriptOwner(site int) romwrite.Owner {
	return romwrite.Owner{Resource: "event.script", ID: site}
}

func (f Format) HeaderSize() int {
	last := f.slots[len(f.slots)-1].offset + 4
	if f.opening >= 0 {
		last = f.opening + 8
	}
	return max(last, f.ending+4)
}
