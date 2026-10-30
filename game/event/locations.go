package event

import (
	"bytes"
	"encoding/binary"
	"image"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// ConditionTarget describes the native meaning of the word at offset 4.
// Only ConditionScript participates in script discovery and relocation.
type ConditionTarget uint8

const (
	ConditionScript ConditionTarget = iota
	RewardTarget
	ShopTarget
	RandomRewardTarget
	NoTarget
	UnknownTarget
)

type LocationKind string

const (
	House         LocationKind = "location_house"
	Village       LocationKind = "location_village"
	VillageCenter LocationKind = "location_village_center"
	Seize         LocationKind = "location_seize"
	Chest         LocationKind = "location_chest"
	Door          LocationKind = "location_door"
	Shop          LocationKind = "location_shop"
	RandomChest   LocationKind = "location_random_chest"
	ScriptChest   LocationKind = "location_script_chest"
)

const (
	LocationX  = "location_x"
	LocationY  = "location_y"
	ChestItem  = "chest_item"
	ChestMoney = "chest_money"
)

// Location is a decoded tile trigger. Command IDs are version-specific; Kind
// describes confirmed combinations of condition type and tile command.
type Location struct {
	Kind                               LocationKind
	Position                           image.Point
	Command, Item, Money               int
	GoldItem, GoldMinimum, ItemMaximum int
}

func (f Format) goldItem() int {
	switch f.kind {
	case eventFE6:
		return 0x6F
	case eventFE8:
		return 0x77
	default:
		return 0x76
	}
}

func (f Format) location(raw []byte) (Location, ConditionTarget) {
	location := Location{
		Position: image.Pt(int(raw[8]), int(raw[9])),
		Command:  int(raw[10]),
		Item:     int(gba.Uint16(raw, 4)), Money: int(gba.Uint16(raw, 6)),
		GoldItem: f.goldItem(), ItemMaximum: 65535,
	}
	kind := gba.Uint16(raw, 0)
	visit, seize, chest, door, center, shop := 0x0E, 0x0F, 0x12, 0x10, 0x1D, 0x13
	if f.kind == eventFE8 {
		location.GoldMinimum, location.ItemMaximum = 256, 255
		visit, seize, chest, door, center, shop = 0x10, 0x11, 0x14, 0x12, 0x20, 0x16
		// LOCA, VILL, CHES and SHOP read a halfword; DOOR reads a byte.
		if kind != 8 {
			location.Command = int(gba.Uint16(raw, 10))
		}
	}
	command := location.Command
	switch {
	case (kind == 5 || kind == 6) && command == visit:
		location.Kind = House
		if kind == 6 {
			location.Kind = Village
		}
		return location, ConditionScript
	case (kind == 5 || kind == 6) && command == seize:
		location.Kind = Seize
		return location, ConditionScript
	case (kind == 5 || kind == 6) && command == center:
		location.Kind = VillageCenter
		return location, NoTarget
	case kind == 5 && command == chest:
		if f.kind != eventFE8 {
			location.Kind = ScriptChest
			return location, ConditionScript
		}
		location.Kind = RandomChest
		return location, RandomRewardTarget
	case kind == 7 && command == chest:
		location.Kind = Chest
		return location, RewardTarget
	case kind == 8 && command == door:
		location.Kind = Door
		return location, ConditionScript
	case kind == 10 && command >= shop && command <= shop+2:
		location.Kind = Shop
		return location, ShopTarget
	default:
		return location, UnknownTarget
	}
}

func (d *ConditionDraft) locationFields(index int) []records.Field {
	location, _ := d.original.format.location(d.records[index].raw)
	switch location.Kind {
	case House, Village, VillageCenter, Seize, Chest, Door, ScriptChest:
	default:
		return nil
	}
	fields := []records.Field{
		{ID: LocationX, Offset: 8, Width: 1},
		{ID: LocationY, Offset: 9, Width: 1},
		{ID: EventFlag, Offset: 2, Width: 2, Hex: true},
	}
	if location.Kind == Chest {
		fields = append(fields,
			records.Field{ID: ChestItem, Offset: 4, Width: 2, Hex: true},
			records.Field{ID: ChestMoney, Offset: 6, Width: 2})
	}
	return fields
}

// AddLocation creates a native trigger. Visits own an ordinary dialog draft;
// seize references the chapter ending, and doors use the native map-change marker.
func (d *ConditionDraft) AddLocation(s *Editor, kind LocationKind) error {
	if !d.Current(s) {
		return rom.ErrStale
	}
	if d.original.Table != LocationEvents {
		return edit.ErrField
	}
	if len(d.records) >= maxEventConditions {
		return edit.ErrValue
	}
	f := d.original.format
	raw := make([]byte, 12)
	raw[0], raw[10] = 5, 0x0E
	if f.kind == eventFE8 {
		raw[10] = 0x10
	}
	record := eventRecord{raw: raw}
	switch kind {
	case House, Village:
		if kind == Village {
			raw[0] = 6
		}
		record.message = new(uint16)
	case Seize:
		raw[10]++
		raw[2] = 3
		for _, ref := range d.scripts {
			if ref.Kind == ChapterEndingScript {
				record.event.Script = ref.Address
				binary.LittleEndian.PutUint32(
					raw[4:],
					uint32(0x08000000+ref.Address),
				) // #nosec G115 -- References are bounded to ROM.
				break
			}
		}
		if record.event.Script == 0 {
			return ErrEventConditions
		}
	case Chest:
		raw[0], raw[4], raw[10] = 7, 1, 0x12
		if f.kind == eventFE8 {
			raw[10] = 0x14
		}
	case Door:
		raw[0], raw[4], raw[10] = 8, 1, 0x10
		if f.kind == eventFE8 {
			raw[10] = 0x12
		}
		record.event.Script = 1
	default:
		return edit.ErrValue
	}
	record.event.decode(raw, f, LocationEvents)
	d.records = append(d.records, record)
	return nil
}

// CopyLocation preserves the complete record and independently owns any new
// dialog draft. Existing referenced resources remain shared.
func (d *ConditionDraft) CopyLocation(index int) error {
	if d.original.Table != LocationEvents || index < 0 ||
		index >= len(d.records) {
		return edit.ErrField
	}
	if len(d.records) >= maxEventConditions {
		return edit.ErrValue
	}
	record := d.records[index]
	record.raw = bytes.Clone(record.raw)
	record.event.Address = 0
	if record.message != nil {
		record.message = new(*record.message)
	}
	d.records = append(d.records, record)
	return nil
}

// SetLocationPosition validates both coordinates before changing either byte.
func (d *ConditionDraft) SetLocationPosition(
	index int,
	position image.Point,
) error {
	if d.original.Table != LocationEvents || index < 0 ||
		index >= len(d.records) ||
		len(d.locationFields(index)) == 0 {
		return edit.ErrField
	}
	if position.X < 0 || position.X > 255 || position.Y < 0 ||
		position.Y > 255 {
		return edit.ErrValue
	}
	d.records[index].raw[8], d.records[index].raw[9] = byte(
		position.X,
	), byte(
		position.Y,
	) // #nosec G115 -- Coordinates are bounded above.
	return nil
}

// SetChestReward changes both reward halfwords as one draft operation. Zero
// selects a different native execution path and is not a direct chest reward.
func (d *ConditionDraft) SetChestReward(index, item, money int) error {
	if d.original.Table != LocationEvents || index < 0 ||
		index >= len(d.records) {
		return edit.ErrField
	}
	location, _ := d.original.format.location(d.records[index].raw)
	if location.Kind != Chest {
		return edit.ErrField
	}
	if item <= 0 || item > location.ItemMaximum || money < 0 || money > 65535 ||
		(item == location.GoldItem && money < location.GoldMinimum) {
		return edit.ErrValue
	}
	binary.LittleEndian.PutUint16(
		d.records[index].raw[4:],
		uint16(item),
	) // #nosec G115 -- Reward halfwords are bounded above.
	binary.LittleEndian.PutUint16(
		d.records[index].raw[6:],
		uint16(money),
	) // #nosec G115 -- Reward halfwords are bounded above.
	return nil
}

func (d *ConditionDraft) validateLocation(record eventRecord) error {
	if d.original.Table != LocationEvents {
		return nil
	}
	location, target := d.original.format.location(record.raw)
	if target == RewardTarget &&
		!bytes.Equal(record.raw[4:8], record.event.Raw[4:8]) {
		if location.Item == 0 || location.Item > location.ItemMaximum ||
			(location.Item == location.GoldItem && location.Money < location.GoldMinimum) {
			return edit.ErrValue
		}
	}
	return nil
}

// Non-script resources are protected only after their own bounded terminator
// is found. Unknown targets remain opaque and never enter the script scanner.
func locationResources(data []byte, condition *Condition) []space.Range {
	if condition.OperandKind != ShopTarget &&
		condition.OperandKind != RandomRewardTarget {
		return nil
	}
	at, ok := gba.Pointer(data, condition.Address+4)
	if !ok || at < 0xC0 || at%2 != 0 {
		condition.Warnings = append(condition.Warnings, ErrEventConditions)
		return nil
	}
	for i := 0; i < 256 && gba.Within(data, at+i*2, 2); i++ {
		if data[at+i*2] == 0 {
			return []space.Range{{Start: at, End: at + i*2 + 2}}
		}
	}
	condition.Warnings = append(condition.Warnings, ErrEventConditions)
	return nil
}
