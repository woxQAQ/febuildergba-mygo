package game

import (
	"bytes"
	"encoding/binary"
	"image"
	"maps"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

const (
	deploymentResource = "deployments"
	deploymentPosition = "deploy_position"
)

// DeploymentField describes one value without exposing packed bits to the UI.
type DeploymentField struct {
	ID, Group            string
	Min, Max             int64
	offset, width, shift int
	movement             bool
}

type deploymentRecord struct {
	data, movement []byte
	movementEdited bool
}

func deploymentFields(stride int) []DeploymentField {
	fields := []DeploymentField{
		{
			ID:     "deploy_character",
			Group:  GroupGeneral,
			Min:    1,
			Max:    255,
			offset: 0,
			width:  1,
		},
		{
			ID:     "deploy_class",
			Group:  GroupGeneral,
			Max:    255,
			offset: 1,
			width:  1,
		},
		{
			ID:     "deploy_leader",
			Group:  GroupGeneral,
			Max:    255,
			offset: 2,
			width:  1,
		},
		{
			ID:     "deploy_level",
			Group:  GroupGeneral,
			Max:    31,
			offset: 3,
			width:  1,
			shift:  3,
		},
		{
			ID:     "deploy_faction",
			Group:  GroupGeneral,
			Max:    3,
			offset: 3,
			width:  1,
			shift:  1,
		},
		{
			ID:     "deploy_autolevel",
			Group:  GroupGeneral,
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
			DeploymentField{
				ID:     "deploy_start_x",
				Group:  deploymentPosition,
				Max:    63,
				offset: 4,
				width:  2,
			},
			DeploymentField{
				ID:     "deploy_start_y",
				Group:  deploymentPosition,
				Max:    63,
				offset: 4,
				width:  2,
				shift:  6,
			},
			DeploymentField{
				ID:       "deploy_end_x",
				Group:    deploymentPosition,
				Max:      63,
				width:    2,
				movement: true,
			},
			DeploymentField{
				ID:       "deploy_end_y",
				Group:    deploymentPosition,
				Max:      63,
				width:    2,
				shift:    6,
				movement: true,
			},
			DeploymentField{
				ID:     "deploy_position_flags",
				Group:  GroupAdvanced,
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
				DeploymentField{
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
			DeploymentField{
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
			DeploymentField{
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

func (r deploymentRecord) value(field DeploymentField) int64 {
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

func (r deploymentRecord) unit() DeploymentUnit {
	unit := DeploymentUnit{
		Character: int(r.data[0]),
		Class:     int(r.data[1]),
		Level:     int(r.data[3] >> 3),
		Faction:   int((r.data[3] >> 1) & 3),
	}
	if len(r.data) == 20 {
		word := u16(r.data, 4)
		unit.Start = image.Pt(int(word&63), int((word>>6)&63))
		unit.End = unit.Start
		unit.MovementCount = len(r.movement) / 8
		if len(r.movement) != 0 {
			at := len(r.movement) - 8
			word = u16(r.movement, at)
			unit.End = image.Pt(int(word&63), int((word>>6)&63))
			unit.RelativeEnd = r.movement[at+3] != 0
		}
	} else {
		unit.Start = image.Pt(int(r.data[4]), int(r.data[5]))
		unit.End = image.Pt(int(r.data[6]), int(r.data[7]))
	}
	return unit
}

// DeploymentDraft preserves complete native records and FE8 paths. Changes stay
// local until an immutable append-and-reference plan is explicitly applied.
type DeploymentDraft struct {
	document   *rom.Document
	revision   uint64
	layouts    map[Kind]Layout
	chapter    int
	group      DeploymentGroup
	records    []deploymentRecord
	generation uint64
}

func (s *Session) NewDeploymentDraft(
	chapter, site int,
) (draft *DeploymentDraft, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.edit_deployment",
			fault.Context{
				Table:    string(Chapters),
				RecordID: new(chapter),
				Range: &fault.Range{
					Offset: site,
					Length: 4,
					Size:   len(s.image.Data),
				},
			},
		)
	}()
	result, err := s.ChapterDeployments(chapter)
	if err != nil {
		return nil, err
	}
	for _, group := range result.Groups {
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
	for _, kind := range []EventTable{TurnEvents, AlwaysEvents} {
		conditions, readErr := s.ChapterEvents(chapter, kind)
		if readErr != nil {
			continue
		}
		for _, event := range conditions.Events {
			for _, group := range event.Groups {
				if group.Pointer == site {
					if group.Err != nil {
						return nil, group.Err
					}
					return s.deploymentDraft(chapter, group), nil
				}
			}
		}
	}
	return nil, ErrField
}

// Generation changes with local edits, independently of the ROM revision.
func (d *DeploymentDraft) Generation() uint64 { return d.generation }

func (d *DeploymentDraft) Revision() uint64 { return d.revision }
func (d *DeploymentDraft) Pointer() int     { return d.group.Pointer }
func (d *DeploymentDraft) Address() int     { return d.group.Address }
func (d *DeploymentDraft) Count() int       { return len(d.records) }

func (d *DeploymentDraft) Fields() []DeploymentField { return deploymentFields(d.group.Stride) }

func (d *DeploymentDraft) Units() []DeploymentUnit {
	result := make([]DeploymentUnit, len(d.records))
	for i, record := range d.records {
		result[i] = record.unit()
	}
	return result
}

func (d *DeploymentDraft) Modified() bool {
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

func (d *DeploymentDraft) Current(s *Session) bool {
	return d.document == s.Document &&
		d.revision == s.Document.State().Revision &&
		maps.Equal(d.layouts, s.layouts)
}

func (d *DeploymentDraft) Value(index int, fieldID string) (int64, error) {
	if index < 0 || index >= len(d.records) {
		return 0, ErrField
	}
	for _, field := range d.Fields() {
		if field.ID == fieldID {
			return d.records[index].value(field), nil
		}
	}
	return 0, ErrField
}

func (d *DeploymentDraft) Set(
	index int,
	fieldID string,
	value int64,
) (err error) {
	ctx := fault.Context{
		Resource: deploymentResource,
		RecordID: new(index),
		Field:    fieldID,
		Value:    new(value),
	}
	defer func() { err = fault.Wrap(err, "game.change_deployment_draft", ctx) }()
	if index < 0 || index >= len(d.records) {
		return ErrField
	}
	for _, field := range d.Fields() {
		if field.ID != fieldID {
			continue
		}
		if value < field.Min || value > field.Max {
			ctx.Bounds = &fault.Bounds{Min: field.Min, Max: field.Max}
			return ErrValue
		}
		record := &d.records[index]
		if record.value(field) == value {
			return nil
		}
		d.generation++
		data, at := record.data, field.offset
		if field.movement {
			if len(record.movement) == 0 {
				record.movement = []byte{
					record.data[4],
					record.data[5] & 0x0F,
					0,
					0,
					0xFF,
					0xFF,
					0,
					0,
				}
				record.data[7] = 1
			}
			data, at = record.movement, len(record.movement)-8
			record.movementEdited = true
		}
		var word int64
		for i := range field.width {
			word |= int64(data[at+i]) << (8 * i)
		}
		word = (word & ^(field.Max << field.shift)) | value<<field.shift
		for i := range field.width {
			data[at+i] = byte((word >> (8 * i)) & 0xFF)
		}
		return nil
	}
	return ErrField
}

func (d *DeploymentDraft) Add() error {
	if len(d.records) >= maxDeploymentUnits {
		return ErrValue
	}
	data := make([]byte, d.group.Stride)
	data[0], data[3] = 1, 8|byte((d.group.faction&3)<<1)
	d.generation++
	d.records = append(d.records, deploymentRecord{data: data})
	return nil
}

func (d *DeploymentDraft) Duplicate(index int) error {
	if index < 0 || index >= len(d.records) ||
		len(d.records) >= maxDeploymentUnits {
		return ErrValue
	}
	d.generation++
	d.records = append(
		d.records,
		cloneDeploymentRecords(d.records[index : index+1])[0],
	)
	return nil
}

func (d *DeploymentDraft) Remove(index int) error {
	if index < 0 || index >= len(d.records) {
		return ErrField
	}
	d.generation++
	d.records = slices.Delete(d.records, index, index+1)
	return nil
}

type DeploymentUse struct {
	Chapter int
	Label   string
}

type DeploymentEdit struct {
	document                        *rom.Document
	revision                        uint64
	layouts                         map[Kind]Layout
	chapter, pointer, before, after int
	plan                            space.Plan
	uses                            []DeploymentUse
}

func (e *DeploymentEdit) Destination() int { return e.plan.Destination.Start }

func (e *DeploymentEdit) Length() int           { return e.plan.Destination.End - e.plan.Destination.Start }
func (e *DeploymentEdit) Before() int           { return e.before }
func (e *DeploymentEdit) After() int            { return e.after }
func (e *DeploymentEdit) Uses() []DeploymentUse { return slices.Clone(e.uses) }

func (s *Session) PlanDeploymentEdit(
	draft *DeploymentDraft,
) (result *DeploymentEdit, err error) {
	defer func() { err = fault.Wrap(err, "game.plan_deployment", fault.Context{Resource: deploymentResource}) }()
	s.Refresh()
	if draft == nil || !draft.Current(s) {
		return nil, rom.ErrStale
	}
	if !draft.Modified() {
		return nil, ErrValue
	}
	// The selected site is the only existing range changed. Reject a damaged
	// event graph that aliases the header, known record tables, or their roots.
	site := draft.group.Pointer
	for _, table := range s.Tables {
		if table.Pointer >= 0 && site < table.Pointer+4 &&
			site+4 > table.Pointer {
			return nil, ErrDeployment
		}
		if table.Err == nil {
			for _, row := range table.Rows {
				if site < row.Address+table.Stride && site+4 > row.Address {
					return nil, ErrDeployment
				}
			}
		}
	}
	content := draft.payload((len(s.image.Data) + 3) &^ 3)
	plan, err := space.PlanAppend(len(s.image.Data), content)
	if err != nil {
		return nil, err
	}
	plan.Edit.Changes = append(
		plan.Edit.Changes,
		rom.Change{
			Offset: site,
			Data: binary.LittleEndian.AppendUint32(
				nil,
				uint32(0x08000000+plan.Destination.Start),
			),
		},
	) // #nosec G115 -- PlanAppend bounds addresses by MaxSize.
	result = &DeploymentEdit{
		document: s.Document,
		revision: s.image.Revision,
		layouts:  maps.Clone(s.layouts),
		chapter:  draft.chapter,
		pointer:  site,
		before:   len(draft.group.records),
		after:    len(draft.records),
		plan:     plan,
	}
	if chapters := s.Table(Chapters); chapters != nil && chapters.Err == nil {
		for _, row := range chapters.Rows {
			groups, readErr := s.Adapter.ChapterDeployments(
				s.image.Data,
				row.ID,
				row.Address,
			)
			if readErr != nil {
				continue
			}
			for _, group := range groups.Groups {
				if group.Pointer == site {
					result.uses = append(
						result.uses,
						DeploymentUse{Chapter: row.ID, Label: group.Label},
					)
				}
			}
		}
	}
	return result, nil
}

func (s *Session) ApplyDeploymentEdit(edit *DeploymentEdit) (err error) {
	ctx := fault.Context{Resource: deploymentResource}
	if edit != nil {
		ctx.Table, ctx.RecordID = string(Chapters), new(edit.chapter)
		ctx.Range = &fault.Range{
			Offset: edit.pointer,
			Length: 4,
			Size:   len(s.image.Data),
		}
	}
	defer func() { err = fault.Wrap(err, "game.apply_deployment", ctx) }()
	if edit == nil || edit.document != s.Document {
		return rom.ErrStale
	}
	if !maps.Equal(edit.layouts, s.layouts) {
		return rom.ErrStale
	}
	if err := s.Document.ApplyEdit(
		edit.revision,
		deploymentResource,
		edit.plan.Edit,
	); err != nil {
		return err
	}
	s.Refresh()
	return nil
}
