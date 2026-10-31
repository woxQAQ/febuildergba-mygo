// Package worldmap reads native world-map node resources.
package worldmap

import (
	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

const (
	NodeCount = 29
	NodeSize  = 32
)

var ErrNodes = fault.Code{
	ID:   "game.invalid_worldmap_nodes",
	Kind: fault.InvalidData,
}

type Format struct{ Pointer int }

type Node struct {
	ID, Address int
	Name        uint16
	Shops       [3]int
}

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
		}
	}
	return nodes, nil
}

// Ranges protects node records and every bounded inventory read, including
// empty inventories. A protected read extent is not proof of ownership.
func (f Format) Ranges(data []byte) []space.Range {
	if f.Pointer == 0 {
		return nil
	}
	result := []space.Range{{Start: f.Pointer, End: f.Pointer + 4}}
	nodes, err := f.Nodes(data)
	if err != nil {
		return result
	}
	for _, node := range nodes {
		result = append(
			result,
			space.Range{Start: node.Address, End: node.Address + NodeSize},
		)
		for _, site := range node.Shops {
			if at, valid := gba.Pointer(
				data,
				site,
			); valid && at >= 0xC0 && at%2 == 0 &&
				gba.Within(data, at, shop.PayloadSize) {
				result = append(
					result,
					space.Range{Start: at, End: at + shop.PayloadSize},
				)
			}
		}
	}
	return result
}
