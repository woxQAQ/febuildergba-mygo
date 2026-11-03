package text

import (
	"bytes"
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
	references References,
) (space.WritePlan, error) {
	encoded, expected, err := g.encodeText(data, entry, id, content, role)
	if err != nil {
		return space.WritePlan{}, err
	}
	builder := space.NewBuilder(data, space.Protection{
		Pointer: func(data []byte, _ space.Owner, site int) error {
			expected, err := g.entry(data, id, references)
			if err != nil {
				return err
			}
			if site != expected {
				return ErrText
			}
			return nil
		},
	})
	if _, err := builder.Replace(encoded, space.PointerSite{
		Owner:   space.Owner{Resource: Resource, ID: int(id)},
		Address: entry, ExpectedWord: gba.Uint32(data, entry),
	}); err != nil {
		return space.WritePlan{}, err
	}
	plan := builder.Plan()
	prospective := bytes.Repeat([]byte{0xFF}, plan.Edit.Size)
	copy(prospective, data)
	for _, change := range plan.Edit.Changes {
		copy(prospective[change.Offset:], change.Data)
	}
	if role == Description {
		decoded, decodeErr := g.Read(prospective, id)
		if decodeErr != nil || !decoded.Editable ||
			decoded.Content != expected {
			return space.WritePlan{}, ErrTextEncoding
		}
	} else {
		if decoded, decodeErr := g.ReadName(
			prospective,
			id,
			references.Aliases,
		); decodeErr != nil ||
			decoded != expected {
			return space.WritePlan{}, ErrNameEncoding
		}
	}
	return plan, nil
}

// Plan is an immutable replacement preview bound to the document and parsing
// settings used during planning.
type Plan struct {
	commit      *edit.Commit
	textID      uint16
	fieldID     string
	content     string
	uses        []Use
	destination space.Range
}

func (e *Plan) TextID() uint16 { return e.textID }

func (e *Plan) Content() string { return e.content }

func (e *Plan) Revision() uint64 { return e.commit.Revision() }

func (e *Plan) Destination() int { return e.destination.Start }

func (e *Plan) Length() int { return e.destination.End - e.destination.Start }

func (e *Plan) Size() int { return e.commit.Size() }

func (e *Plan) Uses() []Use { return slices.Clone(e.uses) }

func (s *Editor) Plan(
	id uint16,
	fieldID, content string,
	role Role,
) (result *Plan, err error) {
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
		references,
	)
	if err != nil {
		return nil, err
	}
	return &Plan{
		commit: edit.NewCommit(s.stamp, "text."+fieldID, plan.Edit),
		textID: id, fieldID: fieldID, content: content,
		uses: slices.Clone(references.Uses), destination: plan.Destination,
	}, nil
}

func (s *Editor) Apply(plan *Plan) (err error) {
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
	for _, region := range references.Manual {
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
) (result *Plan, err error) {
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
