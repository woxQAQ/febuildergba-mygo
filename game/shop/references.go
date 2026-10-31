package shop

import (
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// Origin identifies the owner of an inventory reference.
type Origin uint8

const (
	Chapter Origin = iota
	Preparation
	Default
	WorldMap
)

// Key uses native IDs, independently of pointer relocation or translated names.
// Slot is the location-event index for chapters and 0/1/2 for world-map shops.
type Key struct {
	Origin   Origin
	ID, Slot int
}

// Reference describes a confirmed pointer site. Chapter references are edited
// with their location table by the event editor.
type Reference struct {
	Key     Key
	Pointer int
	Name    string
	Kind    int
}

// Format holds the loader default and optional preparation inventory literals.
// Zero means that the entry does not exist in this version.
type Format struct {
	DefaultPointer, PreparationPointer int
}

func (f Format) References() []Reference {
	var refs []Reference
	if f.PreparationPointer != 0 {
		refs = append(
			refs,
			Reference{
				Key:     Key{Origin: Preparation},
				Pointer: f.PreparationPointer,
			},
		)
	}
	if f.DefaultPointer != 0 {
		refs = append(
			refs,
			Reference{Key: Key{Origin: Default}, Pointer: f.DefaultPointer},
		)
	}
	return refs
}

func (f Format) Default(data []byte) (*Draft, error) {
	if address, valid := gba.Pointer(
		data,
		f.DefaultPointer,
	); f.DefaultPointer != 0 &&
		valid {
		return Read(data, address)
	}
	return nil, ErrInventory
}

func (f Format) Ranges(data []byte) []space.Range {
	var result []space.Range
	for _, ref := range f.References() {
		result = append(
			result,
			space.Range{Start: ref.Pointer, End: ref.Pointer + 4},
		)
		if at, valid := gba.Pointer(
			data,
			ref.Pointer,
		); valid && at >= 0xC0 && at%2 == 0 &&
			gba.Within(data, at, PayloadSize) {
			result = append(
				result,
				space.Range{Start: at, End: at + PayloadSize},
			)
		}
	}
	return result
}
