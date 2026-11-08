package romwrite

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"slices"
	"strings"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

var (
	ErrRange = fault.Code{
		ID:   "space.invalid_range",
		Kind: fault.InvalidInput,
	}
	ErrCapacity  = fault.Code{ID: "space.no_capacity", Kind: fault.Capacity}
	ErrReference = fault.Code{
		ID:   "space.invalid_reference",
		Kind: fault.Conflict,
	}
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
// Offset is relative to the source, and Address is the pointer's file offset.
type Reference struct {
	// Owner authorizes an external pointer update. Internal references are
	// rewritten only in the copy and do not write their original sites.
	Owner           Owner
	Address, Offset int
}

// Move declares the source, selected references and allocation evidence for
// a relocation. Owner authorizes reuse of an existing destination interval.
type Move struct {
	Owner      Owner
	Source     Range
	References []Reference
	Available  []FreeRange
	Reserved   []Range
}

// WritePlan describes resolved writes and allocation without binding them to a
// document or revision. The result owns its byte slices but remains mutable;
// the caller freezes Edit with its original input stamp at the commit boundary.
type WritePlan struct {
	// Destination bounds all allocated payloads. Gaps can contain retained bytes
	// and do not grant write access. It is zero for patch/repoint-only plans.
	Destination Range
	Edit        rom.Edit
}

// planAppend allocates aligned space at the ROM end without reusing existing
// bytes.
func planAppend(size int, content []byte) (result WritePlan, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"space.append",
			fault.Context{
				Range: &fault.Range{
					Offset: size,
					Length: len(content),
					Size:   size,
				},
			},
		)
	}()
	if size < 1 || size > rom.MaxSize || len(content) == 0 {
		return WritePlan{}, ErrRange
	}
	destination, err := allocate(size, len(content), nil, nil)
	if err != nil {
		return WritePlan{}, err
	}
	return WritePlan{
		Destination: destination,
		Edit: rom.Edit{
			Size: destination.End,
			Changes: []rom.Change{
				{Offset: destination.Start, Data: bytes.Clone(content)},
			},
		},
	}, nil
}

// Relocate copies a declared resource and redirects selected references.
// The original source remains protected from other writes in this builder.
// Reusing an existing interval also requires the range protection callback.
func (b *Builder) Relocate(move Move) (result Range, err error) {
	data := b.data
	defer func() {
		err = fault.Wrap(err, "space.move", fault.Context{
			Range: &fault.Range{
				Offset: move.Source.Start,
				Length: move.Source.End - move.Source.Start,
				Size:   len(data),
			},
		})
	}()
	if b.encoding || len(data) > rom.MaxSize || !move.Source.valid(len(data)) ||
		move.Source.Start%4 != 0 {
		return Range{}, ErrRange
	}
	if len(move.References) == 0 {
		return Range{}, ErrReference
	}
	reserved := append([]Range{move.Source}, move.Reserved...)
	for _, region := range reserved {
		if !region.valid(len(data)) {
			return Range{}, ErrRange
		}
	}
	for _, region := range move.Available {
		if !region.valid(len(data)) {
			return Range{}, ErrRange
		}
	}
	reserved = append(reserved, b.sources...)
	for _, change := range b.plan.Edit.Changes {
		region := Range{change.Offset, change.Offset + len(change.Data)}
		if region.overlaps(move.Source) {
			return Range{}, ErrRange
		}
		reserved = append(reserved, region)
	}
	references := slices.Clone(move.References)
	slices.SortFunc(
		references,
		func(a, b Reference) int { return cmp.Compare(a.Address, b.Address) },
	)
	previous := -1
	for _, ref := range references {
		if !validReference(data, move.Source, ref) || ref.Address == previous {
			return Range{}, fault.Wrap(
				ErrReference,
				"space.verify_reference",
				fault.Context{
					Range: &fault.Range{
						Offset: ref.Address,
						Length: 4,
						Size:   len(data),
					},
				},
			)
		}
		previous = ref.Address
		reserved = append(reserved, Range{ref.Address, ref.Address + 4})
		if ref.Address < move.Source.Start || ref.Address >= move.Source.End {
			if err = b.checkPointer(PointerSite{
				Owner:   ref.Owner,
				Address: ref.Address,
				ExpectedWord: uint32( // #nosec G115 -- Source and reference offsets were bounded by ROM MaxSize.
					0x08000000 + move.Source.Start + ref.Offset,
				),
			}); err != nil {
				return Range{}, err
			}
		}
	}
	destination, err := allocate(
		b.plan.Edit.Size,
		move.Source.End-move.Source.Start,
		move.Available,
		reserved,
	)
	if err != nil {
		return Range{}, err
	}
	if destination.Start < len(data) {
		if b.protection.Range == nil {
			return Range{}, ErrClaim
		}
		if err = b.protection.Range(data, move.Owner, destination); err != nil {
			return Range{}, err
		}
	}
	content := bytes.Clone(data[move.Source.Start:move.Source.End])
	edit := rom.Edit{Size: max(b.plan.Edit.Size, destination.End)}
	for _, ref := range references {
		value := uint32(
			0x08000000 + destination.Start + ref.Offset,
		) // #nosec G115 -- allocation and reference offsets are bounded by ROM MaxSize.
		if ref.Address >= move.Source.Start && ref.Address < move.Source.End {
			binary.LittleEndian.PutUint32(
				content[ref.Address-move.Source.Start:],
				value,
			)
			continue
		}
		edit.Changes = append(edit.Changes, rom.Change{
			Offset: ref.Address,
			Data:   binary.LittleEndian.AppendUint32(nil, value),
		})
	}
	edit.Changes = append(
		edit.Changes,
		rom.Change{Offset: destination.Start, Data: content},
	)
	if err = b.add(edit); err != nil {
		return Range{}, err
	}
	b.sources = append(b.sources, move.Source)
	b.allocated(destination)
	return destination, nil
}

func validReference(data []byte, source Range, ref Reference) bool {
	if ref.Address < 0 || ref.Address%4 != 0 || ref.Address > len(data)-4 ||
		ref.Offset < 0 || ref.Offset >= source.End-source.Start {
		return false
	}
	// A pointer cannot straddle the resource boundary.
	location := Range{ref.Address, ref.Address + 4}
	if location.overlaps(source) &&
		(location.Start < source.Start || location.End > source.End) {
		return false
	}
	want := uint32(
		0x08000000 + source.Start + ref.Offset,
	) // #nosec G115 -- source lies inside MaxSize.
	return binary.LittleEndian.Uint32(data[ref.Address:]) == want
}

func allocate(
	size, length int,
	available []FreeRange,
	reserved []Range,
) (result Range, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"space.allocate",
			fault.Context{
				Range: &fault.Range{
					Offset: size,
					Length: length,
					Size:   rom.MaxSize,
				},
			},
		)
	}()
	free := slices.Clone(available)
	slices.SortFunc(
		free,
		func(a, b FreeRange) int { return cmp.Compare(a.Start, b.Start) },
	)
	end := 0
	for _, region := range free {
		if !region.valid(size) || region.Start < end ||
			strings.TrimSpace(region.Source) == "" {
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
