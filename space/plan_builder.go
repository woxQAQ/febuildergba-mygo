package space

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

// Protection checks existing writes against the resource declarations and
// layout rules of the caller. A missing check rejects that kind of write.
// Checks must not modify the snapshot or the builder.
type Protection struct {
	Pointer func([]byte, Owner, int) error
	Range   func([]byte, Owner, Range) error
}

// PointerSite identifies a selected reference and its expected encoded value.
// Address is a four-byte-aligned file offset. ExpectedWord is the complete
// little-endian word, including a possible null value.
type PointerSite struct {
	Owner        Owner
	Address      int
	ExpectedWord uint32
}

// Builder composes replacements, relocations, appended payloads and in-place
// patches against one snapshot. Every operation validates all its writes before
// adding any of them. The input snapshot must remain unchanged while building.
// Document identity, revision and interpretation are bound by the commit layer.
type Builder struct {
	data       []byte
	protection Protection
	plan       WritePlan
	sources    []Range
	encoding   bool
}

func NewBuilder(data []byte, protection Protection) *Builder {
	return &Builder{
		data:       data,
		protection: protection,
		plan:       WritePlan{Edit: rom.Edit{Size: len(data)}},
	}
}

// AppendEncoded resolves the destination and encodes the payload as one
// operation. The encoder must leave the source and protection rules unchanged.
// Calls that try to modify this builder from the encoder are rejected.
func (b *Builder) AppendEncoded(encode func(int) ([]byte, error)) (int, error) {
	content, err := b.encode(encode)
	if err != nil {
		return 0, err
	}
	return b.Append(content)
}

// ReplaceEncoded encodes at the resolved destination before scheduling the
// payload and selected pointer updates together. Failure consumes no space.
func (b *Builder) ReplaceEncoded(
	encode func(int) ([]byte, error),
	pointers ...PointerSite,
) (int, error) {
	content, err := b.encode(encode)
	if err != nil {
		return 0, err
	}
	return b.Replace(content, pointers...)
}

func (b *Builder) encode(encode func(int) ([]byte, error)) ([]byte, error) {
	if b.encoding || encode == nil {
		return nil, ErrRange
	}
	b.encoding = true
	defer func() { b.encoding = false }()
	return encode((b.plan.Edit.Size + 3) &^ 3)
}

// Append writes an independent payload without changing existing references.
func (b *Builder) Append(content []byte) (int, error) {
	allocation, err := planAppend(b.plan.Edit.Size, content)
	if err != nil {
		return 0, err
	}
	if err = b.add(allocation.Edit); err != nil {
		return 0, err
	}
	b.allocated(allocation.Destination)
	return allocation.Destination.Start, nil
}

// Replace appends a payload and redirects only the selected references. The
// old payload and independent references remain intact. ExpectedWord values and
// ownership are checked before allocating any space in the accumulated plan.
func (b *Builder) Replace(
	content []byte,
	pointers ...PointerSite,
) (int, error) {
	if len(pointers) == 0 {
		return 0, ErrReference
	}
	allocation, err := planAppend(b.plan.Edit.Size, content)
	if err != nil {
		return 0, err
	}
	value := uint32(
		0x08000000 + allocation.Destination.Start,
	) // #nosec G115 -- planAppend bounds the address by ROM MaxSize.
	for _, pointer := range pointers {
		if err = b.checkPointer(pointer); err != nil {
			return 0, err
		}
		allocation.Edit.Changes = append(allocation.Edit.Changes, rom.Change{
			Offset: pointer.Address,
			Data:   binary.LittleEndian.AppendUint32(nil, value),
		})
	}
	if err = b.add(allocation.Edit); err != nil {
		return 0, err
	}
	b.allocated(allocation.Destination)
	return allocation.Destination.Start, nil
}

// Repoint updates a selected reference to a file offset in the source or the
// accumulated expansion. Offset zero means the ROM start, not a null pointer.
func (b *Builder) Repoint(pointer PointerSite, target int) error {
	if target < 0 || target >= b.plan.Edit.Size || target >= rom.MaxSize {
		return ErrReference
	}
	return b.repointWord(
		pointer,
		uint32(0x08000000+target),
	)
}

// ClearPointer writes a null word to a selected reference. Its domain meaning
// is established by the caller before planning.
func (b *Builder) ClearPointer(pointer PointerSite) error {
	return b.repointWord(pointer, 0)
}

func (b *Builder) repointWord(pointer PointerSite, value uint32) error {
	if err := b.checkPointer(pointer); err != nil {
		return err
	}
	return b.add(rom.Edit{Size: b.plan.Edit.Size, Changes: []rom.Change{{
		Offset: pointer.Address,
		Data:   binary.LittleEndian.AppendUint32(nil, value),
	}}})
}

// Patch replaces existing bytes without shifting any subsequent content.
// Changes to an appended payload must be encoded before Append or Replace.
func (b *Builder) Patch(owner Owner, offset int, content []byte) error {
	if offset < 0 || offset > len(b.data) || len(content) == 0 ||
		len(content) > len(b.data)-offset {
		return ErrRange
	}
	if b.protection.Range == nil {
		return ErrClaim
	}
	if err := b.protection.Range(
		b.data,
		owner,
		Range{offset, offset + len(content)},
	); err != nil {
		return err
	}
	return b.add(
		rom.Edit{
			Size:    b.plan.Edit.Size,
			Changes: []rom.Change{{Offset: offset, Data: bytes.Clone(content)}},
		},
	)
}

// Plan returns an independent copy of the accumulated edit. Later operations
// and mutations of a returned plan cannot change any other preview.
func (b *Builder) Plan() WritePlan {
	result := b.plan
	result.Edit.Changes = cloneChanges(b.plan.Edit.Changes)
	return result
}

func (b *Builder) checkPointer(pointer PointerSite) error {
	site := pointer.Address
	if site < 0 || site%4 != 0 || site > len(b.data)-4 {
		return ErrRange
	}
	if binary.LittleEndian.Uint32(b.data[site:]) != pointer.ExpectedWord {
		return fault.Wrap(ErrReference, "space.verify_reference", fault.Context{
			Range: &fault.Range{Offset: site, Length: 4, Size: len(b.data)},
		})
	}
	if b.protection.Pointer == nil {
		return ErrClaim
	}
	return b.protection.Pointer(b.data, pointer.Owner, site)
}

// add takes ownership of the supplied writes only after the complete
// composition has passed bounds and overlap checks.
func (b *Builder) add(edit rom.Edit) error {
	if b.encoding || edit.Size < b.plan.Edit.Size || edit.Size > rom.MaxSize ||
		edit.Size < 1 {
		return ErrRange
	}
	ordered := append(slices.Clone(b.plan.Edit.Changes), edit.Changes...)
	slices.SortFunc(ordered, func(a, b rom.Change) int {
		return cmp.Compare(a.Offset, b.Offset)
	})
	end := 0
	for _, change := range ordered {
		if change.Offset < end || change.Offset < 0 ||
			change.Offset > edit.Size ||
			len(change.Data) == 0 ||
			len(change.Data) > edit.Size-change.Offset {
			return fault.Wrap(ErrRange, "space.compose", fault.Context{
				Range: &fault.Range{
					Offset: change.Offset,
					Length: len(change.Data),
					Size:   edit.Size,
				},
			})
		}
		end = change.Offset + len(change.Data)
		for _, source := range b.sources {
			if source.overlaps(Range{change.Offset, end}) {
				return fault.Wrap(
					ErrRange,
					"space.preserve_source",
					fault.Context{
						Range: &fault.Range{
							Offset: change.Offset,
							Length: len(change.Data),
						},
					},
				)
			}
		}
	}
	b.plan.Edit = rom.Edit{Size: edit.Size, Changes: ordered}
	return nil
}

func (b *Builder) allocated(destination Range) {
	if b.plan.Destination == (Range{}) {
		b.plan.Destination = destination
		return
	}
	b.plan.Destination.Start = min(b.plan.Destination.Start, destination.Start)
	b.plan.Destination.End = max(b.plan.Destination.End, destination.End)
}

func cloneChanges(changes []rom.Change) []rom.Change {
	result := make([]rom.Change, len(changes))
	for i, change := range changes {
		result[i] = rom.Change{
			Offset: change.Offset,
			Data:   bytes.Clone(change.Data),
		}
	}
	return result
}
