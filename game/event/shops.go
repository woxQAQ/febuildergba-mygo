package event

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

// ShopKind is the native chapter shop category, independent of tile command IDs.
type ShopKind uint8

const (
	Armory ShopKind = iota
	Vendor
	SecretShop
)

func (f Format) shopCommand(kind ShopKind) int {
	if f.kind == eventFE8 {
		return 0x16 + int(kind)
	}
	return 0x13 + int(kind)
}

func (d *ConditionDraft) Shop(index int) (*shop.Draft, error) {
	if d.original.Table != LocationEvents || index < 0 ||
		index >= len(d.records) {
		return nil, edit.ErrField
	}
	record := d.records[index]
	if record.inventoryErr != nil {
		return nil, record.inventoryErr
	}
	if record.inventory == nil {
		return nil, edit.ErrField
	}
	return record.inventory, nil
}

// SetShopKind changes only confirmed command bits and preserves FE6/7's high byte.
func (d *ConditionDraft) SetShopKind(index int, kind ShopKind) error {
	if d.original.Table != LocationEvents || index < 0 ||
		index >= len(d.records) {
		return edit.ErrField
	}
	location, _ := d.original.format.location(d.records[index].raw)
	if location.Kind != Shop {
		return edit.ErrField
	}
	if kind > SecretShop {
		return edit.ErrValue
	}
	d.records[index].raw[10] = byte(
		d.original.format.shopCommand(kind),
	) // #nosec G115 -- Native commands fit in a byte.
	return nil
}

// InventoryChange describes one independently allocated shop in the frozen plan.
type InventoryChange struct {
	Location, Before, After, Count int
	Uses                           []shop.Reference
}

func (e *ConditionEdit) Inventories() []InventoryChange {
	result := slices.Clone(e.inventories)
	for i := range result {
		result[i].Uses = slices.Clone(result[i].Uses)
	}
	return result
}

func (s *Editor) inventoryUses(address int) []shop.Reference {
	if address == 0 {
		return nil
	}
	if s.source.InventoryUses != nil {
		return s.source.InventoryUses(s.image.Data, address)
	}
	table := s.records.Table(records.Chapters)
	if table == nil || table.Err != nil {
		return nil
	}
	var result []shop.Reference
	for _, row := range table.Rows {
		conditions, err := s.Conditions(row.ID, LocationEvents)
		if err != nil {
			continue
		}
		for i, condition := range conditions.Events {
			if condition.OperandKind != ShopTarget {
				continue
			}
			if at, valid := gba.Pointer(
				s.image.Data,
				condition.Address+4,
			); valid &&
				at == address {
				result = append(
					result,
					shop.Reference{
						Key: shop.Key{
							Origin: shop.Chapter,
							ID:     row.ID,
							Slot:   i,
						},
						Pointer: condition.Address + 4,
						Name:    row.Name,
						Kind:    int(condition.Location.ShopKind),
					},
				)
			}
		}
	}
	return result
}
