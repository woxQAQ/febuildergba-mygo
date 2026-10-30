package deployment

import (
	"bytes"
	"encoding/binary"
	"image"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

const (
	Resource           = "deployments"
	deploymentPosition = "deploy_position"
)

// DeploymentField describes one value without exposing packed bits to the UI.
type Field struct {
	ID, Group            string
	Min, Max             int64
	offset, width, shift int
	movement             bool
}

type deploymentRecord struct {
	data, movement []byte
	movementEdited bool
}

func deploymentFields(stride int) []Field {
	fields := []Field{
		{
			ID:     "deploy_character",
			Group:  records.GroupGeneral,
			Min:    1,
			Max:    255,
			offset: 0,
			width:  1,
		},
		{
			ID:     "deploy_class",
			Group:  records.GroupGeneral,
			Max:    255,
			offset: 1,
			width:  1,
		},
		{
			ID:     "deploy_leader",
			Group:  records.GroupGeneral,
			Max:    255,
			offset: 2,
			width:  1,
		},
		{
			ID:     "deploy_level",
			Group:  records.GroupGeneral,
			Max:    31,
			offset: 3,
			width:  1,
			shift:  3,
		},
		{
			ID:     "deploy_faction",
			Group:  records.GroupGeneral,
			Max:    3,
			offset: 3,
			width:  1,
			shift:  1,
		},
		{
			ID:     "deploy_autolevel",
			Group:  records.GroupGeneral,
			Max:    1,
			offset: 3,
			width:  1,
		},
	}
	itemOffset := 8
	if stride == 20 {
		itemOffset = 12
		fields = append(
			fields,
			Field{
				ID:     "deploy_start_x",
				Group:  deploymentPosition,
				Max:    63,
				offset: 4,
				width:  2,
			},
			Field{
				ID:     "deploy_start_y",
				Group:  deploymentPosition,
				Max:    63,
				offset: 4,
				width:  2,
				shift:  6,
			},
			Field{
				ID:       "deploy_end_x",
				Group:    deploymentPosition,
				Max:      63,
				width:    2,
				movement: true,
			},
			Field{
				ID:       "deploy_end_y",
				Group:    deploymentPosition,
				Max:      63,
				width:    2,
				shift:    6,
				movement: true,
			},
			Field{
				ID:     "deploy_position_flags",
				Group:  records.GroupAdvanced,
				Max:    7,
				offset: 4,
				width:  2,
				shift:  12,
			},
		)
	} else {
		for i, id := range []string{"deploy_start_x", "deploy_start_y", "deploy_end_x", "deploy_end_y"} {
			fields = append(
				fields,
				Field{
					ID:     id,
					Group:  deploymentPosition,
					Max:    255,
					offset: 4 + i,
					width:  1,
				},
			)
		}
	}
	for i, id := range []string{"deploy_item_1", "deploy_item_2", "deploy_item_3", "deploy_item_4"} {
		fields = append(
			fields,
			Field{
				ID:     id,
				Group:  "items",
				Max:    255,
				offset: itemOffset + i,
				width:  1,
			},
		)
	}
	for i, id := range []string{"deploy_ai_1", "deploy_ai_2", "deploy_ai_3", "deploy_ai_4"} {
		fields = append(
			fields,
			Field{
				ID:     id,
				Group:  "deploy_ai",
				Max:    255,
				offset: itemOffset + 4 + i,
				width:  1,
			},
		)
	}
	return fields
}

func (r deploymentRecord) value(field Field) int64 {
	data, at := r.data, field.offset
	if field.movement {
		if len(r.movement) == 0 {
			at = 4
		} else {
			data, at = r.movement, len(r.movement)-8
		}
	}
	var word int64
	for i := range field.width {
		word |= int64(data[at+i]) << (i * 8)
	}
	return (word >> field.shift) & field.Max
}

func (r deploymentRecord) unit() Unit {
	unit := Unit{
		Character: int(r.data[0]),
		Class:     int(r.data[1]),
		Level:     int(r.data[3] >> 3),
		Faction:   int((r.data[3] >> 1) & 3),
	}
	if len(r.data) == 20 {
		word := gba.Uint16(r.data, 4)
		unit.Start = image.Pt(int(word&63), int((word>>6)&63))
		unit.End = unit.Start
		unit.MovementCount = len(r.movement) / 8
		if len(r.movement) != 0 {
			at := len(r.movement) - 8
			word = gba.Uint16(r.movement, at)
			unit.End = image.Pt(int(word&63), int((word>>6)&63))
		}
	} else {
		unit.Start = image.Pt(int(r.data[4]), int(r.data[5]))
		unit.End = image.Pt(int(r.data[6]), int(r.data[7]))
	}
	return unit
}

// DeploymentDraft preserves complete native records and FE8 paths. Changes stay
// local until an immutable append-and-reference plan is explicitly applied.
type Draft struct {
	binding    edit.Stamp
	chapter    int
	group      Group
	records    []deploymentRecord
	generation uint64
}

func (s *Editor) NewDraft(
	chapter, site int,
) (draft *Draft, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.edit_deployment",
			fault.Context{
				Table:    string(records.Chapters),
				RecordID: new(chapter),
				Range: &fault.Range{
					Offset: site,
					Length: 4,
					Size:   len(s.image.Data),
				},
			},
		)
	}()
	s.Refresh()
	groups, err := s.source.Candidates(s.image.Data, chapter)
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		if group.Pointer != site {
			continue
		}
		if group.Err != nil {
			return nil, group.Err
		}
		if len(group.Units) > 0 {
			group.faction = group.Units[0].Faction
		}
		return s.deploymentDraft(chapter, group), nil
	}
	return nil, edit.ErrField
}

// Generation changes with local edits, independently of the ROM revision.
func (d *Draft) Generation() uint64 { return d.generation }

func (d *Draft) Revision() uint64 { return d.binding.Revision() }

func (d *Draft) Pointer() int { return d.group.Pointer }

func (d *Draft) Address() int { return d.group.Address }

func (d *Draft) Count() int { return len(d.records) }

func (d *Draft) Fields() []Field { return deploymentFields(d.group.Stride) }

func (d *Draft) Units() []Unit {
	result := make([]Unit, len(d.records))
	for i, record := range d.records {
		result[i] = record.unit()
	}
	return result
}

func (d *Draft) Modified() bool {
	if len(d.records) != len(d.group.records) {
		return true
	}
	for i, record := range d.records {
		original := d.group.records[i]
		if !bytes.Equal(record.data, original.data) ||
			!bytes.Equal(record.movement, original.movement) {
			return true
		}
	}
	return false
}

func (d *Draft) Current(s *Editor) bool {
	return d.binding.Current(s.context)
}

func (d *Draft) Value(index int, fieldID string) (int64, error) {
	if index < 0 || index >= len(d.records) {
		return 0, edit.ErrField
	}
	for _, field := range d.Fields() {
		if field.ID == fieldID {
			return d.records[index].value(field), nil
		}
	}
	return 0, edit.ErrField
}

func (d *Draft) Set(
	index int,
	fieldID string,
	value int64,
) (err error) {
	ctx := fault.Context{
		Resource: Resource,
		RecordID: new(index),
		Field:    fieldID,
		Value:    new(value),
	}
	defer func() { err = fault.Wrap(err, "game.change_deployment_draft", ctx) }()
	if index < 0 || index >= len(d.records) {
		return edit.ErrField
	}
	for _, field := range d.Fields() {
		if field.ID != fieldID {
			continue
		}
		if value < field.Min || value > field.Max {
			ctx.Bounds = &fault.Bounds{Min: field.Min, Max: field.Max}
			return edit.ErrValue
		}
		record := &d.records[index]
		if record.value(field) == value {
			return nil
		}
		d.generation++
		data, at := record.data, field.offset
		if field.movement {
			if len(record.movement) == 0 {
				record.movement = movementStep(record.unit().Start)
				record.data[7] = 1
			}
			data, at = record.movement, len(record.movement)-8
			record.movementEdited = true
		}
		writeDeploymentValue(data, at, field, value)
		return nil
	}
	return edit.ErrField
}

func writeDeploymentValue(
	data []byte,
	at int,
	field Field,
	value int64,
) {
	var word int64
	for i := range field.width {
		word |= int64(data[at+i]) << (8 * i)
	}
	word = (word & ^(field.Max << field.shift)) | value<<field.shift
	for i := range field.width {
		data[at+i] = byte((word >> (8 * i)) & 0xFF)
	}
}

func (d *Draft) Add() error {
	if len(d.records) >= maxDeploymentUnits {
		return edit.ErrValue
	}
	data := make([]byte, d.group.Stride)
	data[0], data[3] = 1, 8|byte((d.group.faction&3)<<1)
	d.generation++
	d.records = append(d.records, deploymentRecord{data: data})
	return nil
}

func (d *Draft) Duplicate(index int) error {
	if index < 0 || index >= len(d.records) ||
		len(d.records) >= maxDeploymentUnits {
		return edit.ErrValue
	}
	d.generation++
	d.records = append(
		d.records,
		cloneDeploymentRecords(d.records[index : index+1])[0],
	)
	return nil
}

func (d *Draft) Remove(index int) error {
	if index < 0 || index >= len(d.records) {
		return edit.ErrField
	}
	d.generation++
	d.records = slices.Delete(d.records, index, index+1)
	return nil
}

func (s *Editor) deploymentDraft(
	chapter int,
	group Group,
) *Draft {
	if len(group.Units) > 0 {
		group.faction = group.Units[0].Faction
	}
	return &Draft{
		binding: s.context.Bind(s.image.Revision),
		chapter: chapter,
		group:   group,
		records: cloneDeploymentRecords(group.records),
	}
}

// payload keeps unmodified native paths shared and copies only changed paths.
// address is the aligned destination of this complete deployment payload.
func (d *Draft) Payload(address int) []byte {
	var content []byte
	for _, record := range d.records {
		content = append(content, record.data...)
	}
	content = append(content, d.group.terminator...)
	for i, record := range d.records {
		if !record.movementEdited || len(record.movement) == 0 {
			continue
		}
		for len(content)%4 != 0 {
			content = append(content, 0)
		}
		binary.LittleEndian.PutUint32(
			content[i*d.group.Stride+8:],
			uint32(0x08000000+address+len(content)),
		) // #nosec G115 -- PlanAppend validates the complete size before application.
		content = append(content, record.movement...)
	}
	return content
}

// newDeploymentGroup creates an unattached unit payload for a coordinated event.
func (s *Editor) NewGroup(
	chapter int,
	format Format,
	label string,
	faction int,
) *Draft {
	s.Refresh()
	return s.deploymentDraft(
		chapter,
		Group{
			Pointer:    -1,
			Label:      label,
			Stride:     format.Stride,
			faction:    faction,
			terminator: make([]byte, 4),
		},
	)
}
