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
	event      EventCondition
	raw        []byte
	deployment *DeploymentDraft
	message    *uint16
}

// EventDraft edits a complete condition list. Native scripts remain untouched;
// new rows own either a deployment or a text ID and their generated script.
type EventDraft struct {
	document *rom.Document
	revision uint64
	layouts  map[Kind]Layout
	chapter  int
	original ChapterEvents
	records  []eventRecord
	scripts  []EventScriptRef
}

func (s *Session) NewEventDraft(
	chapter int,
	table EventTable,
) (*EventDraft, error) {
	resource, err := s.ChapterEvents(chapter, table)
	if err != nil {
		return nil, err
	}
	d := &EventDraft{
		document: s.Document,
		revision: s.image.Revision,
		layouts:  maps.Clone(s.layouts),
		chapter:  chapter,
		original: resource,
	}
	for _, event := range resource.Events {
		d.records = append(
			d.records,
			eventRecord{event: event, raw: bytes.Clone(event.Raw)},
		)
	}
	if table == AlwaysEvents {
		d.scripts = s.objectiveScripts(resource, chapter)
	}
	return d, nil
}

func (d *EventDraft) Current(s *Session) bool {
	return d.document == s.Document &&
		d.revision == s.Document.State().Revision &&
		maps.Equal(d.layouts, s.layouts)
}

func (d *EventDraft) Modified() bool {
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

func (d *EventDraft) Table() EventTable { return d.original.Table }

func (d *EventDraft) Events() []EventCondition {
	events := make([]EventCondition, len(d.records))
	for i, record := range d.records {
		event := record.event
		event.decode(record.raw, d.original.format)
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

func (d *EventDraft) Fields(index int) []Field {
	if index < 0 || index >= len(d.records) {
		return nil
	}
	if fields := d.objectiveFields(index); len(fields) > 0 {
		return fields
	}
	if int(
		u16(d.records[index].raw, 0),
	) != d.original.format.condition(
		d.original.Table,
	) {
		return nil
	}
	if d.original.Table == TalkEvents {
		fields := []Field{
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
				Field{ID: TalkMode, Offset: 12, Width: width},
				Field{ID: TalkFlag, Offset: 14, Width: 2, Hex: true},
			)
		}
		return fields
	}
	if d.original.Table == AlwaysEvents {
		return []Field{
			{ID: AreaX1, Offset: 8, Width: 1, Signed: true},
			{ID: AreaY1, Offset: 9, Width: 1, Signed: true},
			{ID: AreaX2, Offset: 10, Width: 1, Signed: true},
			{ID: AreaY2, Offset: 11, Width: 1, Signed: true},
			{ID: EventFlag, Offset: 2, Width: 2, Hex: true},
		}
	}
	fields := []Field{
		{ID: TurnStart, Offset: 8, Width: 1},
		{ID: TurnEnd, Offset: 9, Width: 1},
		{ID: TurnPhase, Offset: 10, Width: 2, Hex: true},
		{ID: EventFlag, Offset: 2, Width: 2, Hex: true},
	}
	if len(d.records[index].raw) == 16 {
		fields = append(fields, Field{ID: TurnRoute, Offset: 12, Width: 4})
	}
	return fields
}

func (d *EventDraft) Value(index int, id string) (int64, error) {
	for _, field := range d.Fields(index) {
		if field.ID == id {
			return field.Value(d.records[index].raw, 0), nil
		}
	}
	return 0, ErrField
}

func (d *EventDraft) Set(index int, id string, value int64) (err error) {
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
		if value < low || value > high {
			ctx.Bounds = &fault.Bounds{Min: low, Max: high}
			return ErrValue
		}
		for i := range field.Width {
			d.records[index].raw[field.Offset+i] = byte(
				(value >> (i * 8)) & 0xFF,
			)
		}
		return nil
	}
	return ErrField
}

func (d *EventDraft) Add(s *Session) error {
	if !d.Current(s) {
		return rom.ErrStale
	}
	if len(d.records) >= maxEventConditions {
		return ErrValue
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
				event:   EventCondition{Kind: kind},
				raw:     raw,
				message: new(uint16),
			},
		)
		return nil
	}
	if d.original.Table == TurnEvents {
		raw[8], raw[10] = 3, 0x80
	}
	group := DeploymentGroup{
		Pointer:    -1,
		Label:      d.original.Table.source(),
		Stride:     d.original.format.stride,
		faction:    2,
		terminator: make([]byte, 4),
	}
	units := s.deploymentDraft(d.chapter, group)
	if err := units.Add(); err != nil {
		return err
	}
	d.records = append(
		d.records,
		eventRecord{
			event: EventCondition{
				Kind:         kind,
				reversedArea: d.original.format.kind == eventFE8,
			},
			raw:        raw,
			deployment: units,
		},
	)
	return nil
}

func (d *EventDraft) Remove(index int) error {
	if index < 0 || index >= len(d.records) {
		return ErrField
	}
	d.records = slices.Delete(d.records, index, index+1)
	return nil
}

func (d *EventDraft) Deployment(index int) *DeploymentDraft {
	if index < 0 || index >= len(d.records) {
		return nil
	}
	return d.records[index].deployment
}

type EventEdit struct {
	document                        *rom.Document
	revision                        uint64
	layouts                         map[Kind]Layout
	chapter, pointer, before, after int
	plan                            space.Plan
	uses                            []int
}

func (e *EventEdit) Destination() int { return e.plan.Destination.Start }

func (e *EventEdit) Length() int { return e.plan.Destination.End - e.plan.Destination.Start }
func (e *EventEdit) Before() int { return e.before }
func (e *EventEdit) After() int  { return e.after }
func (e *EventEdit) Uses() []int { return slices.Clone(e.uses) }

func (s *Session) PlanEventEdit(d *EventDraft) (result *EventEdit, err error) {
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
		return nil, ErrValue
	}
	site := d.original.Pointer
	if err = s.resourcePointer(site); err != nil {
		return nil, err
	}
	base := (len(s.image.Data) + 3) &^ 3
	var content []byte
	offsets := make([]int, len(d.records))
	for i, record := range d.records {
		if record.message != nil {
			if _, readErr := s.DialogueText(*record.message); readErr != nil {
				return nil, readErr
			}
			if record.raw[9] == 0 ||
				(d.original.format.kind == eventFE6 && record.raw[8] == 0) {
				return nil, ErrValue
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
			if !record.deployment.Current(s) {
				return nil, rom.ErrStale
			}
			if err = record.deployment.validatePaths(); err != nil {
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
				record.deployment.payload(base+unitOffset)...)
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
	result = &EventEdit{
		document: s.Document,
		revision: s.image.Revision,
		layouts:  maps.Clone(s.layouts),
		chapter:  d.chapter,
		pointer:  site,
		before:   len(d.original.Events),
		after:    len(d.records),
		plan:     plan,
	}
	if chapters := s.Table(Chapters); chapters != nil && chapters.Err == nil {
		for _, row := range chapters.Rows {
			turns, readErr := s.Adapter.ChapterEvents(
				s.image.Data,
				row.ID,
				row.Address,
				d.original.Table,
			)
			if readErr == nil && turns.Pointer == site {
				result.uses = append(result.uses, row.ID)
			}
		}
	}
	return result, nil
}

func (s *Session) ApplyEventEdit(edit *EventEdit) (err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.apply_event_conditions",
			fault.Context{Resource: eventConditionsResource},
		)
	}()
	if edit == nil || edit.document != s.Document ||
		!maps.Equal(edit.layouts, s.layouts) {
		return rom.ErrStale
	}
	if err := s.Document.ApplyEdit(
		edit.revision,
		eventConditionsResource,
		edit.plan.Edit,
	); err != nil {
		return err
	}
	s.Refresh()
	return nil
}

func (f deploymentFormat) reinforcementScript() []byte {
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
func (s *Session) resourcePointer(site int) error {
	if site < 0xC0 || site%4 != 0 || !within(s.image.Data, site, 4) {
		return ErrEventConditions
	}
	for _, table := range s.Tables {
		if table.Pointer >= 0 && site < table.Pointer+4 &&
			site+4 > table.Pointer {
			return ErrEventConditions
		}
		if table.Err == nil {
			for _, row := range table.Rows {
				if site < row.Address+table.Stride && site+4 > row.Address {
					return ErrEventConditions
				}
			}
		}
	}
	return nil
}
