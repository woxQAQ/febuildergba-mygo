// Package space plans resource writes and moves using declared free ranges and verified references.
package space

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"errors"
	"slices"
	"strings"

	"github.com/woxQAQ/febuildergba-mygo/rom"
)

var (
	ErrRange     = errors.New("space.invalid_range")
	ErrCapacity  = errors.New("space.no_capacity")
	ErrReference = errors.New("space.invalid_reference")
)

// Range is a half-open ROM interval [Start, End).
type Range struct {
	Start, End int
}

func (r Range) valid(size int) bool {
	return r.Start >= 0 && r.Start < r.End && r.End <= size
}

func (r Range) overlaps(other Range) bool {
	return r.Start < other.End && other.Start < r.End
}

// FreeRange carries the evidence that permits reusing an existing interval.
// A run of zeroes or FF bytes alone is not evidence of unused space.
type FreeRange struct {
	Range
	Source string
}

// Reference identifies a pointer to a byte within the moved resource.
// Offset is relative to the source, and Address is the pointer's ROM location.
type Reference struct {
	Address, Offset int
}

type Move struct {
	Source     Range
	References []Reference
	Available  []FreeRange
	Reserved   []Range
}

// Plan contains a complete document edit. Planning never changes the input bytes.
type Plan struct {
	Destination Range
	Edit        rom.Edit
}

// PlanAppend allocates aligned space at the ROM end without reusing existing bytes.
func PlanAppend(size int, content []byte) (Plan, error) {
	if size < 1 || size > rom.MaxSize || len(content) == 0 {
		return Plan{}, ErrRange
	}
	destination, err := allocate(size, len(content), nil, nil)
	if err != nil {
		return Plan{}, err
	}
	return Plan{Destination: destination, Edit: rom.Edit{Size: destination.End, Changes: []rom.Change{{Offset: destination.Start, Data: bytes.Clone(content)}}}}, nil
}

// PlanMove preserves the source and updates only explicitly selected references.
// Internal references are rewritten in the copy, leaving the original intact.
func PlanMove(data []byte, move Move) (Plan, error) {
	if len(data) > rom.MaxSize || !move.Source.valid(len(data)) || move.Source.Start%4 != 0 {
		return Plan{}, ErrRange
	}
	if len(move.References) == 0 {
		return Plan{}, ErrReference
	}
	reserved := append([]Range{move.Source}, move.Reserved...)
	for _, region := range reserved {
		if !region.valid(len(data)) {
			return Plan{}, ErrRange
		}
	}
	references := slices.Clone(move.References)
	slices.SortFunc(references, func(a, b Reference) int { return cmp.Compare(a.Address, b.Address) })
	previous := -1
	for _, ref := range references {
		if !validReference(data, move.Source, ref) || ref.Address == previous {
			return Plan{}, ErrReference
		}
		previous = ref.Address
		reserved = append(reserved, Range{ref.Address, ref.Address + 4})
	}
	destination, err := allocate(len(data), move.Source.End-move.Source.Start, move.Available, reserved)
	if err != nil {
		return Plan{}, err
	}
	content := bytes.Clone(data[move.Source.Start:move.Source.End])
	edit := rom.Edit{Size: max(len(data), destination.End)}
	for _, ref := range references {
		value := uint32(0x08000000 + destination.Start + ref.Offset) // #nosec G115 -- bounded by MaxSize.
		if ref.Address >= move.Source.Start && ref.Address < move.Source.End {
			binary.LittleEndian.PutUint32(content[ref.Address-move.Source.Start:], value)
			continue
		}
		encoded := binary.LittleEndian.AppendUint32(nil, value)
		edit.Changes = append(edit.Changes, rom.Change{Offset: ref.Address, Data: encoded})
	}
	edit.Changes = append(edit.Changes, rom.Change{Offset: destination.Start, Data: content})
	return Plan{Destination: destination, Edit: edit}, nil
}

func validReference(data []byte, source Range, ref Reference) bool {
	if ref.Address < 0 || ref.Address%4 != 0 || ref.Address > len(data)-4 ||
		ref.Offset < 0 || ref.Offset >= source.End-source.Start {
		return false
	}
	// A pointer cannot straddle the resource boundary.
	location := Range{ref.Address, ref.Address + 4}
	if location.overlaps(source) && (location.Start < source.Start || location.End > source.End) {
		return false
	}
	want := uint32(0x08000000 + source.Start + ref.Offset) // #nosec G115 -- source lies inside MaxSize.
	return binary.LittleEndian.Uint32(data[ref.Address:]) == want
}

func allocate(size, length int, available []FreeRange, reserved []Range) (Range, error) {
	free := slices.Clone(available)
	slices.SortFunc(free, func(a, b FreeRange) int { return cmp.Compare(a.Start, b.Start) })
	end := 0
	for _, region := range free {
		if !region.valid(size) || region.Start < end || strings.TrimSpace(region.Source) == "" {
			return Range{}, ErrRange
		}
		end = region.End
		if slices.ContainsFunc(reserved, region.overlaps) {
			return Range{}, ErrRange
		}
	}
	for _, region := range free {
		start := (region.Start + 3) &^ 3
		if length <= region.End-start {
			return Range{start, start + length}, nil
		}
	}
	start := (size + 3) &^ 3
	if length > rom.MaxSize-start {
		return Range{}, ErrCapacity
	}
	return Range{start, start + length}, nil
}
