package game

import (
	"bytes"
	"encoding/binary"
	"maps"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

const textResource = "text"

// TextUse identifies a known field sharing a text ID. Events and other text
// users also observe edits to that same ID.
type TextUse struct {
	Kind  Kind
	ID    int
	Field string
}

// TextEdit is an immutable preview bound to a document revision and table
// layouts.
type TextEdit struct {
	document *rom.Document
	revision uint64
	layouts  map[Kind]Layout
	textID   uint16
	fieldID  string
	content  string
	uses     []TextUse
	plan     space.Plan
}

func (e *TextEdit) TextID() uint16   { return e.textID }
func (e *TextEdit) Content() string  { return e.content }
func (e *TextEdit) Revision() uint64 { return e.revision }
func (e *TextEdit) Destination() int { return e.plan.Destination.Start }

func (e *TextEdit) Length() int     { return e.plan.Destination.End - e.plan.Destination.Start }
func (e *TextEdit) Size() int       { return e.plan.Edit.Size }
func (e *TextEdit) Uses() []TextUse { return slices.Clone(e.uses) }

func (s *Session) Text(
	kind Kind,
	id int,
	fieldID string,
) (result TextValue, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.read_text_field",
			fault.Context{
				Table:    string(kind),
				RecordID: new(id),
				Field:    fieldID,
			},
		)
	}()
	s.Refresh()
	_, row, field, err := s.locate(kind, id, fieldID)
	if err != nil {
		return TextValue{}, err
	}
	if field.Text != DescriptionText || field.Width != 2 {
		return TextValue{}, ErrField
	}
	return s.Adapter.ReadText(
		s.image.Data,
		u16(s.image.Data, row.Address+field.Offset),
	)
}

func (s *Session) PlanTextEdit(
	kind Kind,
	id int,
	fieldID, content string,
) (result *TextEdit, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.plan_text_field",
			fault.Context{
				Table:    string(kind),
				RecordID: new(id),
				Field:    fieldID,
			},
		)
	}()
	s.Refresh()
	_, row, field, err := s.locate(kind, id, fieldID)
	if err != nil {
		return nil, err
	}
	if field.Text == NoText || field.Width != 2 {
		return nil, ErrField
	}
	return s.planTextEdit(
		u16(s.image.Data, row.Address+field.Offset),
		fieldID,
		content,
		field.Text,
	)
}

func (s *Session) planTextEdit(
	textID uint16,
	fieldID, content string,
	role TextRole,
) (result *TextEdit, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.plan_text",
			fault.Context{
				Resource:   textResource,
				ResourceID: new(int(textID)),
				Field:      fieldID,
			},
		)
	}()
	plan, err := s.Adapter.PlanTextEdit(s.image.Data, textID, content, role)
	if err != nil {
		return nil, err
	}
	edit := &TextEdit{
		document: s.Document,
		revision: s.image.Revision,
		layouts:  maps.Clone(s.layouts),
		textID:   textID,
		fieldID:  fieldID,
		content:  content,
		plan:     plan,
	}
	for _, table := range s.Tables {
		if table.Err != nil {
			continue
		}
		for _, change := range plan.Edit.Changes {
			if table.Manual &&
				change.Offset < table.Address+table.Stride*table.Count &&
				table.Address < change.Offset+len(change.Data) {
				return nil, ErrLayout
			}
		}
	}
	edit.uses = s.TextUses(textID)
	return edit, nil
}

func (s *Session) ApplyTextEdit(edit *TextEdit) (err error) {
	ctx := fault.Context{Resource: textResource}
	if edit != nil {
		ctx.ResourceID = new(int(edit.textID))
		ctx.Field = edit.fieldID
	}
	defer func() { err = fault.Wrap(err, "game.apply_text", ctx) }()
	if edit == nil || edit.document != s.Document ||
		!maps.Equal(edit.layouts, s.layouts) {
		return rom.ErrStale
	}
	if err := s.Document.ApplyEdit(
		edit.revision,
		"text."+edit.fieldID,
		edit.plan.Edit,
	); err != nil {
		return err
	}
	s.Refresh()
	return nil
}

func (g gameFormat) ReadText(
	data []byte,
	id uint16,
) (result TextValue, err error) {
	defer func() {
		ctx := fault.Context{Resource: textResource, ResourceID: new(int(id))}
		err = fault.Wrap(err, "game.read_text", ctx)
		result.ReadOnlyReason = fault.Wrap(
			result.ReadOnlyReason,
			"game.edit_text",
			ctx,
		)
	}()
	if !g.Match(data) {
		return TextValue{}, ErrVersion
	}
	stream, err := g.text.readStream(data, id)
	if err != nil {
		return TextValue{}, err
	}
	value := g.text.descriptionValue(stream)
	value.ID = id
	if id == 0 {
		value.Editable, value.ReadOnlyReason = false, ErrText
	}
	return value, nil
}

func (g gameFormat) encodeText(
	data []byte,
	entry int,
	id uint16,
	content string,
	role TextRole,
) ([]byte, string, error) {
	switch role {
	case NameText:
		if _, err := g.text.readName(data, id, nil); err != nil {
			return nil, "", err
		}
		encoded, err := g.text.encodeName(data, entry, content)
		return encoded, content, err
	case DescriptionText:
		original, err := g.ReadText(data, id)
		if err != nil {
			return nil, "", err
		}
		if !original.Editable {
			return nil, "", fault.Wrap(
				original.ReadOnlyReason,
				"game.encode_text",
				fault.Context{Resource: textResource, ResourceID: new(int(id))},
			)
		}
		value, err := g.text.parseDescription(content)
		if err != nil {
			return nil, "", fault.Wrap(
				err,
				"game.parse_text",
				fault.Context{Resource: textResource, ResourceID: new(int(id))},
			)
		}
		if original.Content == value.Content {
			return nil, "", ErrValue
		}
		encoded, err := g.text.encodeTextUnits(data, entry, value.units, false)
		return encoded, value.Content, err
	default:
		return nil, "", ErrField
	}
}

func (g gameFormat) PlanTextEdit(
	data []byte,
	id uint16,
	content string,
	role TextRole,
) (space.Plan, error) {
	if !g.Match(data) {
		return space.Plan{}, ErrVersion
	}
	entry, err := g.textEntry(data, id)
	if err != nil {
		return space.Plan{}, err
	}
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
	if role == DescriptionText {
		decoded, decodeErr := g.ReadText(prospective, id)
		if decodeErr != nil || !decoded.Editable ||
			decoded.Content != expected {
			return space.Plan{}, ErrTextEncoding
		}
	} else {
		var aliases map[byte]string
		if g.features != nil {
			aliases, _ = g.features(prospective)
		}
		if decoded, decodeErr := g.text.readName(
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

// Writes require a valid primary text-table pointer. The recovery address is a
// read fallback, not evidence that the game will consume edits at that address.
func (g gameFormat) textEntry(data []byte, id uint16) (int, error) {
	table, ok := pointer(data, g.text.pointer)
	entry := table + int(id)*4
	if id == 0 || id >= 0x7FFF || !ok || table%4 != 0 || table < 0xC0 ||
		!within(data, entry, 4) {
		return 0, ErrText
	}
	for _, region := range g.deploymentRanges(data) {
		if entry < region.End && region.Start < entry+4 {
			return 0, ErrLayout
		}
	}
	protected := []int{g.text.pointer, g.text.treePointer, g.text.rootPointer}
	for _, rule := range g.tableRules(data) {
		protected = append(protected, rule.pointers...)
		other := rule.read(
			data,
			Layout{},
			false,
			func(uint16) (string, error) { return "", nil },
		)
		if region, extentErr := rule.extent(
			data,
			other,
		); extentErr == nil && entry < region.End &&
			region.Start < entry+4 {
			return 0, ErrLayout
		}
	}
	for _, address := range protected {
		if entry < address+4 && address < entry+4 {
			return 0, ErrText
		}
	}
	return entry, nil
}
