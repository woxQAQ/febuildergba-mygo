package event

import (
	"bytes"
	"encoding/binary"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

const (
	TurnStart     = "turn_start"
	TurnEnd       = "turn_end"
	TurnPhase     = "turn_phase"
	EventFlag     = "event_flag"
	EventTrigger  = "event_trigger_flag"
	AreaX1        = "area_x1"
	AreaY1        = "area_y1"
	AreaX2        = "area_x2"
	AreaY2        = "area_y2"
	TurnRoute     = "turn_route"
	TalkInitiator = "talk_initiator"
	TalkTarget    = "talk_target"
	TalkMode      = "talk_mode"
	TalkFlag      = "talk_flag"
)

type eventRecord struct {
	event      Condition
	raw        []byte
	deployment *deployment.Draft
	message    *uint16
}

// EventDraft edits a complete condition list. Native scripts remain untouched;
// new rows own either a deployment or a text ID and their generated script.
type ConditionDraft struct {
	binding  edit.Stamp
	chapter  int
	original Conditions
	records  []eventRecord
	scripts  []EventScriptRef
}

func (s *Editor) NewConditions(
	chapter int,
	table Table,
) (*ConditionDraft, error) {
	resource, err := s.Conditions(chapter, table)
	if err != nil {
		return nil, err
	}
	d := &ConditionDraft{
		binding:  s.context.Bind(s.image.Revision),
		chapter:  chapter,
		original: resource,
	}
	for _, event := range resource.Events {
		d.records = append(
			d.records,
			eventRecord{event: event, raw: bytes.Clone(event.Raw)},
		)
	}
	if table == AlwaysEvents || table == LocationEvents {
		objectives := resource
		if table == LocationEvents {
			objectives, _ = s.Conditions(chapter, AlwaysEvents)
		}
		d.scripts = s.objectiveScripts(objectives, chapter)
	}
	return d, nil
}

func (d *ConditionDraft) Current(s *Editor) bool {
	return d.binding.Current(s.context)
}

func (d *ConditionDraft) Modified() bool {
	if len(d.records) != len(d.original.Events) {
		return true
	}
	for i, record := range d.records {
		if record.deployment != nil || record.message != nil ||
			!bytes.Equal(record.raw, d.original.Events[i].Raw) {
			return true
		}
	}
	return false
}

func (d *ConditionDraft) Table() Table { return d.original.Table }

func (d *ConditionDraft) Events() []Condition {
	events := make([]Condition, len(d.records))
	for i, record := range d.records {
		event := record.event
		event.decode(record.raw, d.original.format, d.original.Table)
		event.Texts = slices.Clone(record.event.Texts)
		if record.message != nil {
			event.Texts = nil
			if *record.message > 0 {
				event.Texts = []uint16{*record.message}
			}
		}
		events[i] = event
	}
	return events
}

func (d *ConditionDraft) Fields(index int) []records.Field {
	if index < 0 || index >= len(d.records) {
		return nil
	}
	if d.original.Table == LocationEvents {
		return d.locationFields(index)
	}
	if fields := d.objectiveFields(index); len(fields) > 0 {
		return fields
	}
	if int(
		gba.Uint16(d.records[index].raw, 0),
	) != d.original.format.condition(
		d.original.Table,
	) {
		return nil
	}
	if d.original.Table == TalkEvents {
		fields := []records.Field{
			{ID: TalkInitiator, Offset: 8, Width: 1, Hex: true},
			{ID: TalkTarget, Offset: 9, Width: 1, Hex: true},
			{ID: EventFlag, Offset: 2, Width: 2, Hex: true},
		}
		if d.original.format.kind != eventFE6 {
			width := 1
			if d.original.format.kind == eventFE8 {
				width = 2
			}
			fields = append(
				fields,
				records.Field{ID: TalkMode, Offset: 12, Width: width},
				records.Field{ID: TalkFlag, Offset: 14, Width: 2, Hex: true},
			)
		}
		return fields
	}
	if d.original.Table == AlwaysEvents {
		return []records.Field{
			{ID: AreaX1, Offset: 8, Width: 1, Signed: true},
			{ID: AreaY1, Offset: 9, Width: 1, Signed: true},
			{ID: AreaX2, Offset: 10, Width: 1, Signed: true},
			{ID: AreaY2, Offset: 11, Width: 1, Signed: true},
			{ID: EventFlag, Offset: 2, Width: 2, Hex: true},
		}
	}
	fields := []records.Field{
		{ID: TurnStart, Offset: 8, Width: 1},
		{ID: TurnEnd, Offset: 9, Width: 1},
		{ID: TurnPhase, Offset: 10, Width: 2, Hex: true},
		{ID: EventFlag, Offset: 2, Width: 2, Hex: true},
	}
	if len(d.records[index].raw) == 16 {
		fields = append(
			fields,
			records.Field{ID: TurnRoute, Offset: 12, Width: 4},
		)
	}
	return fields
}

func (d *ConditionDraft) Value(index int, id string) (int64, error) {
	for _, field := range d.Fields(index) {
		if field.ID == id {
			return field.Value(d.records[index].raw, 0), nil
		}
	}
	return 0, edit.ErrField
}

func (d *ConditionDraft) Set(index int, id string, value int64) (err error) {
	ctx := fault.Context{
		Resource: eventConditionsResource,
		RecordID: new(index),
		Field:    id,
		Value:    new(value),
	}
	defer func() { err = fault.Wrap(err, "game.change_event_draft", ctx) }()
	for _, field := range d.Fields(index) {
		if field.ID != id {
			continue
		}
		low, high := field.Bounds()
		if d.original.Table == LocationEvents {
			location, _ := d.original.format.location(d.records[index].raw)
			if id == ChestItem {
				low, high = 1, int64(location.ItemMaximum)
			}
			if id == ChestMoney && location.Item == location.GoldItem {
				low = int64(location.GoldMinimum)
			}
		}
		if value < low || value > high {
			ctx.Bounds = &fault.Bounds{Min: low, Max: high}
			return edit.ErrValue
		}
		for i := range field.Width {
			d.records[index].raw[field.Offset+i] = byte(
				(value >> (i * 8)) & 0xFF,
			)
		}
		return nil
	}
	return edit.ErrField
}

func (d *ConditionDraft) Add(s *Editor) error {
	if !d.Current(s) {
		return rom.ErrStale
	}
	if len(d.records) >= maxEventConditions {
		return edit.ErrValue
	}
	if d.original.Table == LocationEvents {
		return edit.ErrField
	}
	kind := d.original.format.condition(d.original.Table)
	raw := make([]byte, d.original.format.eventStride(d.original.Table, kind))
	raw[0] = byte(
		kind,
	) // #nosec G115 -- Confirmed native condition kinds fit in a byte.
	if d.original.Table == TalkEvents {
		raw[8], raw[9] = 1, 2
		d.records = append(
			d.records,
			eventRecord{
				event:   Condition{Kind: kind},
				raw:     raw,
				message: new(uint16),
			},
		)
		return nil
	}
	if d.original.Table == TurnEvents {
		raw[8], raw[10] = 3, 0x80
	}
	units := s.deployments.NewGroup(
		d.chapter,
		d.original.format.units,
		d.original.Table.source(),
		2,
	)
	if err := units.Add(); err != nil {
		return err
	}
	d.records = append(
		d.records,
		eventRecord{
			event: Condition{
				Kind:         kind,
				reversedArea: d.original.format.kind == eventFE8,
			},
			raw:        raw,
			deployment: units,
		},
	)
	return nil
}

func (d *ConditionDraft) Remove(index int) error {
	if index < 0 || index >= len(d.records) {
		return edit.ErrField
	}
	d.records = slices.Delete(d.records, index, index+1)
	return nil
}

func (d *ConditionDraft) Deployment(index int) *deployment.Draft {
	if index < 0 || index >= len(d.records) {
		return nil
	}
	return d.records[index].deployment
}

type ConditionEdit struct {
	binding                         edit.Stamp
	chapter, pointer, before, after int
	plan                            space.Plan
	uses                            []int
}

func (e *ConditionEdit) Destination() int { return e.plan.Destination.Start }

func (e *ConditionEdit) Length() int { return e.plan.Destination.End - e.plan.Destination.Start }

func (e *ConditionEdit) Before() int { return e.before }

func (e *ConditionEdit) After() int { return e.after }

func (e *ConditionEdit) Uses() []int { return slices.Clone(e.uses) }

func (s *Editor) PlanConditions(
	d *ConditionDraft,
) (result *ConditionEdit, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.plan_event_conditions",
			fault.Context{Resource: eventConditionsResource},
		)
	}()
	s.Refresh()
	if d == nil || !d.Current(s) {
		return nil, rom.ErrStale
	}
	if !d.Modified() {
		return nil, edit.ErrValue
	}
	site := d.original.Pointer
	if err = s.resourcePointer(site); err != nil {
		return nil, err
	}
	base := (len(s.image.Data) + 3) &^ 3
	var content []byte
	offsets := make([]int, len(d.records))
	for i, record := range d.records {
		if err = d.validateLocation(record); err != nil {
			return nil, err
		}
		if record.message != nil {
			if _, readErr := s.DialogueText(*record.message); readErr != nil {
				return nil, readErr
			}
			if d.original.Table == TalkEvents && (record.raw[9] == 0 ||
				(d.original.format.kind == eventFE6 && record.raw[8] == 0)) {
				return nil, edit.ErrValue
			}
		}
		offsets[i] = len(content)
		content = append(content, record.raw...)
	}
	content = append(content, 0, 0, 0, 0)
	for i, record := range d.records {
		if record.deployment == nil && record.message == nil {
			continue
		}
		for len(content)%4 != 0 {
			content = append(content, 0)
		}
		scriptOffset := len(content)
		if record.message != nil {
			content = append(
				content,
				d.original.format.dialogueScript(*record.message)...)
		} else {
			if !record.deployment.Current(s.deployments) {
				return nil, rom.ErrStale
			}
			if err = record.deployment.Validate(); err != nil {
				return nil, err
			}
			content = append(
				content,
				d.original.format.reinforcementScript()...)
			for len(content)%4 != 0 {
				content = append(content, 0)
			}
			unitOffset := len(content)
			content = append(
				content,
				record.deployment.Payload(base+unitOffset)...)
			binary.LittleEndian.PutUint32(
				content[scriptOffset+4:],
				uint32(0x08000000+base+unitOffset),
			) // #nosec G115 -- PlanAppend bounds the complete resource before application.
		}
		binary.LittleEndian.PutUint32(
			content[offsets[i]+4:],
			uint32(0x08000000+base+scriptOffset),
		) // #nosec G115 -- PlanAppend bounds the complete resource before application.
	}
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
	) // #nosec G115 -- PlanAppend bounds ROM addresses.
	result = &ConditionEdit{
		binding: s.context.Bind(s.image.Revision),
		chapter: d.chapter,
		pointer: site,
		before:  len(d.original.Events),
		after:   len(d.records),
		plan:    plan,
	}
	if chapters := s.records.Table(
		records.Chapters,
	); chapters != nil &&
		chapters.Err == nil {
		for _, row := range chapters.Rows {
			turns, readErr := s.Conditions(row.ID, d.original.Table)
			if readErr == nil && turns.Pointer == site {
				result.uses = append(result.uses, row.ID)
			}
		}
	}
	return result, nil
}

func (s *Editor) ApplyConditions(edit *ConditionEdit) (err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.apply_event_conditions",
			fault.Context{Resource: eventConditionsResource},
		)
	}()
	if edit == nil {
		return rom.ErrStale
	}
	if err := edit.binding.Check(s.context); err != nil {
		return err
	}
	if err := s.context.Document().ApplyEdit(
		edit.binding.Revision(),
		eventConditionsResource,
		edit.plan.Edit,
	); err != nil {
		return err
	}
	s.Refresh()
	return nil
}

func (f Format) reinforcementScript() []byte {
	if f.kind == eventFE8 {
		return []byte{
			0x40,
			0x2C,
			1,
			0,
			0,
			0,
			0,
			0,
			0x20,
			0x30,
			0,
			0,
			0x20,
			1,
			0,
			0,
		}
	}
	load, wait, end := uint32(0x30), uint32(0x39), uint32(0x0A)
	if f.kind == eventFE7U {
		load, wait = 0x32, 0x3B
	}
	if f.kind == eventFE6 {
		load, wait, end = 0x12, 0x14, 0x06
	}
	var result []byte
	for _, word := range []uint32{load, 0, wait, end, 0} {
		result = binary.LittleEndian.AppendUint32(result, word)
	}
	return result
}

// Resource pointers are four-byte writes, distinct from the immutable appended
// resources. A corrupted graph cannot authorize overwriting record tables.
func (s *Editor) resourcePointer(site int) error {
	return s.source.ProtectPointer(s.image.Data, site)
}
