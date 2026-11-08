// Package worldmap reads and edits native world-map nodes and routes.
package worldmap

import (
	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

const (
	Resource  = "worldmap"
	NodeCount = 29
	NodeSize  = 32
)

var ErrNodes = fault.Code{
	ID:   "game.invalid_worldmap_nodes",
	Kind: fault.InvalidData,
}

type Format struct {
	Pointer, RoutePointer int
	Scene                 SceneFormat
}

type Node struct {
	ID, Address int
	Name        uint16
	Shops       [3]int
	raw         [NodeSize]byte
}

func (n Node) Owner() romwrite.Owner { return romwrite.Owner{Resource: Resource, ID: n.ID} }

func (f Format) Nodes(data []byte) ([]Node, error) {
	if f.Pointer == 0 {
		return nil, nil
	}
	at, valid := gba.Pointer(data, f.Pointer)
	if !valid || at < 0xC0 || at%4 != 0 ||
		!gba.Within(data, at, NodeCount*NodeSize) {
		return nil, fault.Wrap(
			ErrNodes,
			"worldmap.read_nodes",
			fault.Context{
				Range: &fault.Range{
					Offset: f.Pointer,
					Length: 4,
					Size:   len(data),
				},
			},
		)
	}
	nodes := make([]Node, NodeCount)
	for i := range nodes {
		address := at + i*NodeSize
		nodes[i] = Node{
			ID:      i,
			Address: address,
			Name:    gba.Uint16(data, address+28),
			Shops:   [3]int{address + 12, address + 16, address + 20},
			raw:     [NodeSize]byte(data[address : address+NodeSize]),
		}
	}
	return nodes, nil
}

// Claims owns nodes, routes and confirmed graphics, including hosted pointers.
// Each inventory remains independently editable through its shop owner.
func (f Format) Claims(data []byte) []romwrite.Claim {
	result := append(f.routeClaims(data), f.Scene.claims(data)...)
	if f.Pointer < 0xC0 || !gba.Within(data, f.Pointer, 4) {
		return result
	}
	result = append(result, romwrite.Claim{
		Start: f.Pointer, End: f.Pointer + 4,
		Owner: romwrite.Owner{Resource: Resource, ID: -1}, Site: true,
	})
	nodes, err := f.Nodes(data)
	if err != nil {
		return result
	}
	for _, node := range nodes {
		result = append(result, romwrite.Claim{
			Start: node.Address,
			End:   node.Address + NodeSize,
			Owner: node.Owner(),
		})
		for slot, site := range node.Shops {
			owner := shop.Key{
				Origin: shop.WorldMap,
				ID:     node.ID,
				Slot:   slot,
			}.Owner()
			result = append(result, romwrite.Claim{
				Start: site, End: site + 4,
				Owner: owner, Host: node.Owner(), Site: true,
			})
			if at, valid := gba.Pointer(data, site); valid && at >= 0xC0 &&
				at%2 == 0 && gba.Within(data, at, shop.PayloadSize) {
				result = append(result, romwrite.Claim{
					Start: at, End: at + shop.PayloadSize,
					Owner: owner,
				})
			}
		}
	}
	return result
}

// Ranges protects bounded resources without proving exclusive ownership.
func (f Format) Ranges(data []byte) []romwrite.Range {
	return romwrite.ClaimRanges(f.Claims(data))
}

func (f Format) TextUses(data []byte, id uint16) []text.Use {
	var result []text.Use
	nodes, _ := f.Nodes(data)
	for _, node := range nodes {
		if node.Name == id {
			result = append(
				result,
				text.Use{Kind: Resource, ID: node.ID, Field: "name"},
			)
		}
	}
	return result
}
