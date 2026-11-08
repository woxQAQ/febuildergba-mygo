// Package arena edits native arena opponents, weapons and link restrictions.
package arena

import (
	"bytes"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

// Kind identifies a native resource, independent of the selected game version.
type Kind uint8

const (
	Melee Kind = iota
	Ranged
	Magic
	BaseWeapons
	Upgrades
	LinkUnits
)

const (
	MaxClasses  = 256
	baseSize    = 8
	upgradeSize = 26
	linkSize    = 22
)

var (
	ErrData      = fault.Code{ID: "game.invalid_arena", Kind: fault.InvalidData}
	ErrReference = fault.Code{
		ID:   "game.arena_reference_conflict",
		Kind: fault.Conflict,
	}
	ErrClasses = fault.Code{
		ID:   "game.arena_class_pool",
		Kind: fault.InvalidInput,
	}
	ErrWeapons = fault.Code{
		ID:   "game.arena_weapon_chain",
		Kind: fault.InvalidInput,
	}
)

// Format contains only confirmed native consumers. Base generation and fallback
// share a resource; separated consumers cannot be silently merged on write.
type Format struct {
	Classes     [3]int
	BaseWeapons []int
	Upgrades    int
	LinkUnits   int
}

func (k Kind) Owner() romwrite.Owner { return romwrite.Owner{Resource: "arena", ID: int(k)} }
func (k Kind) ClassList() bool       { return k <= Magic }

func (f Format) Groups() []Kind {
	groups := []Kind{Melee, Ranged, Magic, BaseWeapons}
	if f.LinkUnits != 0 {
		groups = append(groups, LinkUnits)
	}
	return groups
}

func (f Format) pointers(kind Kind) []int {
	switch {
	case kind.ClassList():
		return []int{f.Classes[kind]}
	case kind == BaseWeapons:
		return slices.Clone(f.BaseWeapons)
	case kind == Upgrades:
		return []int{f.Upgrades}
	case kind == LinkUnits && f.LinkUnits != 0:
		return []int{f.LinkUnits}
	default:
		return nil
	}
}

type table struct {
	kind          Kind
	address       int
	pointers      []int
	original, raw []byte
	keys          []int
	nextKey       int
}

func readPayload(data []byte, kind Kind, at int) ([]byte, error) {
	if at < 0xC0 || !gba.Within(data, at, 1) {
		return nil, ErrData
	}
	if kind.ClassList() {
		end := min(len(data), at+MaxClasses+1)
		if n := bytes.IndexByte(data[at:end], 0); n >= 0 {
			return bytes.Clone(data[at : at+n+1]), nil
		}
		return bytes.Clone(data[at:end]), ErrData
	}
	size := baseSize
	if kind == Upgrades {
		size = upgradeSize
	}
	if kind == LinkUnits {
		size = linkSize
		if at%2 != 0 {
			return nil, ErrData
		}
	}
	if !gba.Within(data, at, size) {
		return nil, ErrData
	}
	raw := bytes.Clone(data[at : at+size])
	if kind == Upgrades && bytes.IndexByte(raw, 0xFF) < 0 {
		return raw, ErrData
	}
	return raw, nil
}

func (f Format) read(data []byte, kind Kind) (*table, error) {
	sites := f.pointers(kind)
	if len(sites) == 0 {
		return nil, ErrData
	}
	var at int
	for i, site := range sites {
		target, valid := gba.Pointer(data, site)
		if site < 0xC0 || site%4 != 0 || !valid {
			return nil, ErrData
		}
		if i > 0 && target != at {
			return nil, ErrReference
		}
		at = target
	}
	raw, err := readPayload(data, kind, at)
	if err != nil {
		return nil, err
	}
	t := &table{
		kind:     kind,
		address:  at,
		pointers: sites,
		original: raw,
		raw:      bytes.Clone(raw),
	}
	count := t.count()
	t.keys = make([]int, count)
	for i := range t.keys {
		t.keys[i] = i
	}
	t.nextKey = count
	return t, nil
}

func (t *table) count() int {
	switch {
	case t.kind.ClassList():
		return len(t.raw) - 1
	case t.kind == LinkUnits:
		return len(t.raw) / 2
	case t.kind == Upgrades:
		return bytes.IndexByte(t.raw, 0xFF) + 1
	default:
		return len(t.raw)
	}
}

// Claims includes each consumer even when the shared table is split or invalid.
// Every independently readable payload is protected; equality is not ownership.
func (f Format) Claims(data []byte) []romwrite.Claim {
	var result []romwrite.Claim
	for kind := Melee; kind <= LinkUnits; kind++ {
		for _, site := range f.pointers(kind) {
			if site < 0xC0 || !gba.Within(data, site, 4) {
				continue
			}
			result = append(
				result,
				romwrite.Claim{
					Start: site, End: site + 4,
					Owner: kind.Owner(),
					Site:  true,
				},
			)
			at, valid := gba.Pointer(data, site)
			if !valid {
				continue
			}
			raw, _ := readPayload(data, kind, at)
			if len(raw) > 0 {
				result = append(
					result,
					romwrite.Claim{
						Start: at, End: at + len(raw),
						Owner: kind.Owner(),
					},
				)
			}
		}
	}
	return result
}
