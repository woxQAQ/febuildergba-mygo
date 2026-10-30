package text

import (
	"bytes"
	"encoding/binary"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func (g Format) plan(
	data []byte,
	id uint16,
	content string,
	role Role,
	entry int,
	aliases map[byte]string,
) (space.Plan, error) {
	encoded, expected, err := g.encodeText(data, entry, id, content, role)
	if err != nil {
		return space.Plan{}, err
	}
	plan, err := space.PlanAppend(len(data), encoded)
	if err != nil {
		return space.Plan{}, err
	}
	value := uint32(
		0x08000000 + plan.Destination.Start,
	) // #nosec G115 -- PlanAppend bounds addresses by MaxSize.
	plan.Edit.Changes = append(
		plan.Edit.Changes,
		rom.Change{
			Offset: entry,
			Data:   binary.LittleEndian.AppendUint32(nil, value),
		},
	)
	prospective := bytes.Repeat([]byte{0xFF}, plan.Edit.Size)
	copy(prospective, data)
	for _, change := range plan.Edit.Changes {
		copy(prospective[change.Offset:], change.Data)
	}
	if role == Description {
		decoded, decodeErr := g.Read(prospective, id)
		if decodeErr != nil || !decoded.Editable ||
			decoded.Content != expected {
			return space.Plan{}, ErrTextEncoding
		}
	} else {
		if decoded, decodeErr := g.ReadName(
			prospective,
			id,
			aliases,
		); decodeErr != nil ||
			decoded != expected {
			return space.Plan{}, ErrNameEncoding
		}
	}
	return plan, nil
}

// Edit is an immutable replacement preview bound to the document and parsing
// settings used during planning.
type Edit struct {
	commit      *edit.Plan
	textID      uint16
	fieldID     string
	content     string
	uses        []Use
	destination space.Range
}

func (e *Edit) TextID() uint16 { return e.textID }

func (e *Edit) Content() string { return e.content }

func (e *Edit) Revision() uint64 { return e.commit.Revision() }

func (e *Edit) Destination() int { return e.destination.Start }

func (e *Edit) Length() int { return e.destination.End - e.destination.Start }

func (e *Edit) Size() int { return e.commit.Size() }

func (e *Edit) Uses() []Use { return slices.Clone(e.uses) }

func (s *Editor) Plan(
	id uint16,
	fieldID, content string,
	role Role,
) (result *Edit, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.plan_text",
			fault.Context{
				Resource:   Resource,
				ResourceID: new(int(id)),
				Field:      fieldID,
			},
		)
	}()
	if err = s.refresh(); err != nil {
		return nil, err
	}
	references := s.references(s.image.Data, id)
	entry, err := s.format.entry(s.image.Data, id, references)
	if err != nil {
		return nil, err
	}
	plan, err := s.format.plan(
		s.image.Data,
		id,
		content,
		role,
		entry,
		references.Aliases,
	)
	if err != nil {
		return nil, err
	}
	for _, region := range references.Manual {
		for _, change := range plan.Edit.Changes {
			if change.Offset < region.End &&
				region.Start < change.Offset+len(change.Data) {
				return nil, edit.ErrLayout
			}
		}
	}
	return &Edit{
		commit: edit.NewPlan(s.stamp, "text."+fieldID, plan.Edit),
		textID: id, fieldID: fieldID, content: content,
		uses: slices.Clone(references.Uses), destination: plan.Destination,
	}, nil
}

func (s *Editor) Apply(plan *Edit) (err error) {
	ctx := fault.Context{Resource: Resource}
	if plan != nil {
		ctx.ResourceID = new(int(plan.textID))
		ctx.Field = plan.fieldID
	}
	defer func() { err = fault.Wrap(err, "game.apply_text", ctx) }()
	if plan == nil {
		return rom.ErrStale
	}
	return plan.commit.Apply(s.context)
}

// Writes require a valid primary table pointer. Recovery is only a read
// fallback and does not prove that the game will consume a replacement.
func (f Format) entry(
	data []byte,
	id uint16,
	references References,
) (int, error) {
	table, ok := gba.Pointer(data, f.Pointer)
	entry := table + int(id)*4
	if id == 0 || id >= 0x7FFF || !ok || table%4 != 0 || table < 0xC0 ||
		!gba.Within(data, entry, 4) {
		return 0, ErrText
	}
	for _, region := range references.Ranges {
		if entry < region.End && region.Start < entry+4 {
			return 0, edit.ErrLayout
		}
	}
	protected := []int{f.Pointer, f.TreePointer, f.RootPointer}
	protected = append(protected, references.Sites...)
	for _, at := range protected {
		if entry < at+4 && at < entry+4 {
			return 0, ErrText
		}
	}
	return entry, nil
}

func (s *Editor) PlanID(
	id uint16,
	content string,
) (result *Edit, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.plan_text_id",
			fault.Context{Resource: Resource, ResourceID: new(int(id))},
		)
	}()
	value, err := s.Read(id)
	if err != nil {
		return nil, err
	}
	if !value.Editable {
		return nil, value.ReadOnlyReason
	}
	return s.Plan(id, "content", content, Description)
}
