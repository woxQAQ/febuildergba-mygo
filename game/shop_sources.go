package game

import (
	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func (s *Session) shopSource() shop.Source {
	return shop.Source{
		References: s.shopReferences,
		ProtectPointer: func(data []byte, owner space.Owner, site int) error {
			return s.guardPointer(data, owner, site, edit.ErrLayout)
		},
	}
}

func (s *Session) shopReferences(data []byte) ([]shop.Reference, []error) {
	g := s.format
	var result []shop.Reference
	var warnings []error
	config := g.chapterConfig()
	for _, table := range s.Records.Inspect(data).Tables {
		if table.Kind != records.Chapters {
			continue
		}
		if table.Err != nil {
			warnings = append(warnings, table.Err)
			continue
		}
		for _, row := range table.Rows {
			conditions, err := config.Conditions(
				data,
				row.ID,
				row.Address,
				event.LocationEvents,
			)
			if err != nil {
				warnings = append(
					warnings,
					fault.Wrap(
						err,
						"game.shop_chapter_references",
						fault.Context{
							Resource: "chapter",
							Field:    "locations",
							RecordID: &row.ID,
						},
					),
				)
				continue
			}
			for i, condition := range conditions.Events {
				if condition.OperandKind != event.ShopTarget {
					continue
				}
				result = append(result, shop.Reference{
					Key: shop.Key{
						Origin: shop.Chapter,
						ID:     row.ID,
						Slot:   i,
					},
					Pointer: condition.Address + 4,
					Name:    row.Name,
					Kind:    int(condition.Location.ShopKind),
				})
			}
		}
	}
	nodes, err := g.worldmap.Nodes(data)
	if err != nil {
		warnings = append(warnings, err)
	}
	var aliases map[byte]string
	if g.features != nil {
		aliases, _ = g.features(data)
	}
	for _, node := range nodes {
		name, _ := g.text.ReadName(data, node.Name, aliases)
		for kind, site := range node.Shops {
			result = append(result, shop.Reference{
				Key: shop.Key{
					Origin: shop.WorldMap,
					ID:     node.ID,
					Slot:   kind,
				},
				Pointer: site,
				Name:    name,
				Kind:    kind,
			})
		}
	}
	return result, warnings
}

// inventoryUses combines native owners without scanning pointer-shaped bytes.
func (s *Session) inventoryUses(data []byte, address int) []shop.Reference {
	refs, _ := s.shopReferences(data)
	refs = append(s.format.shops.References(), refs...)
	var result []shop.Reference
	defaults, _ := s.format.shops.Default(data)
	for _, ref := range refs {
		at, valid := gba.Pointer(data, ref.Pointer)
		if gba.Within(data, ref.Pointer, 4) &&
			gba.Uint32(data, ref.Pointer) == 0 &&
			(ref.Key.Origin == shop.Chapter || ref.Key.Origin == shop.Preparation) &&
			defaults != nil {
			at, valid = defaults.Address(), true
		}
		if valid && at == address {
			result = append(result, ref)
		}
	}
	return result
}
