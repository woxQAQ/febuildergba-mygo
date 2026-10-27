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
	TurnStart = "turn_start"
	TurnEnd   = "turn_end"
	TurnPhase = "turn_phase"
	TurnFlag  = "turn_flag"
	TurnRoute = "turn_route"
)

type turnRecord struct {
	event      TurnEvent
	raw        []byte
	deployment *DeploymentDraft
}

// TurnDraft edits a complete condition list. Native scripts remain untouched;
// new reinforcement rows own a deployment and a generated minimal load script.
type TurnDraft struct {
	document *rom.Document
	revision uint64
	layouts  map[Kind]Layout
	chapter  int
	original ChapterTurns
	records  []turnRecord
}

func (s *Session) NewTurnDraft(chapter int) (*TurnDraft, error) {
	resource, err := s.ChapterTurns(chapter)
	if err != nil {
		return nil, err
	}
	d := &TurnDraft{
		document: s.Document,
		revision: s.image.Revision,
		layouts:  maps.Clone(s.layouts),
		chapter:  chapter,
		original: resource,
	}
	for _, event := range resource.Events {
		d.records = append(
			d.records,
			turnRecord{event: event, raw: bytes.Clone(event.Raw)},
		)
	}
	return d, nil
}

func (d *TurnDraft) Current(s *Session) bool {
	return d.document == s.Document &&
		d.revision == s.Document.State().Revision &&
		maps.Equal(d.layouts, s.layouts)
}

func (d *TurnDraft) Modified() bool {
	if len(d.records) != len(d.original.Events) {
		return true
	}
	for i, record := range d.records {
		if record.deployment != nil ||
			!bytes.Equal(record.raw, d.original.Events[i].Raw) {
			return true
		}
	}
	return false
}

func (d *TurnDraft) Events() []TurnEvent {
	events := make([]TurnEvent, len(d.records))
	for i, record := range d.records {
		event := record.event
		event.Raw = bytes.Clone(record.raw)
		event.Flag = int(u16(record.raw, 2))
		event.Start, event.End, event.Phase = int(
			record.raw[8],
		), int(
			record.raw[9],
		), int(
			u16(record.raw, 10),
		)
		if len(record.raw) == 16 {
			event.Route = u32(record.raw, 12)
		}
		events[i] = event
	}
	return events
}

func (d *TurnDraft) Fields(index int) []Field {
	if index < 0 || index >= len(d.records) ||
		u16(d.records[index].raw, 0) != 2 {
		return nil
	}
	fields := []Field{
		{ID: TurnStart, Offset: 8, Width: 1},
		{ID: TurnEnd, Offset: 9, Width: 1},
		{ID: TurnPhase, Offset: 10, Width: 2, Hex: true},
		{ID: TurnFlag, Offset: 2, Width: 2, Hex: true},
	}
	if len(d.records[index].raw) == 16 {
		fields = append(fields, Field{ID: TurnRoute, Offset: 12, Width: 4})
	}
	return fields
}

func (d *TurnDraft) Value(index int, id string) (int64, error) {
	for _, field := range d.Fields(index) {
		if field.ID == id {
			return field.Value(d.records[index].raw, 0), nil
		}
	}
	return 0, ErrField
}

func (d *TurnDraft) Set(index int, id string, value int64) (err error) {
	ctx := fault.Context{
		Resource: turnEventsResource,
		RecordID: new(index),
		Field:    id,
		Value:    new(value),
	}
	defer func() { err = fault.Wrap(err, "game.change_turn_draft", ctx) }()
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

func (d *TurnDraft) Add(s *Session) error {
	if !d.Current(s) {
		return rom.ErrStale
	}
	if len(d.records) >= maxTurnEvents {
		return ErrValue
	}
	raw := make([]byte, d.original.format.turnStride(2))
	raw[0], raw[8], raw[10] = 2, 3, 0x80
	group := DeploymentGroup{
		Pointer:    -1,
		Label:      "deploy_turn_event",
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
		turnRecord{event: TurnEvent{Kind: 2}, raw: raw, deployment: units},
	)
	return nil
}

func (d *TurnDraft) Remove(index int) error {
	if index < 0 || index >= len(d.records) {
		return ErrField
	}
	d.records = slices.Delete(d.records, index, index+1)
	return nil
}

func (d *TurnDraft) Deployment(index int) *DeploymentDraft {
	if index < 0 || index >= len(d.records) {
		return nil
	}
	return d.records[index].deployment
}

type TurnEdit struct {
	document                        *rom.Document
	revision                        uint64
	layouts                         map[Kind]Layout
	chapter, pointer, before, after int
	plan                            space.Plan
	uses                            []int
}

func (e *TurnEdit) Destination() int { return e.plan.Destination.Start }

func (e *TurnEdit) Length() int { return e.plan.Destination.End - e.plan.Destination.Start }
func (e *TurnEdit) Before() int { return e.before }
func (e *TurnEdit) After() int  { return e.after }
func (e *TurnEdit) Uses() []int { return slices.Clone(e.uses) }

func (s *Session) PlanTurnEdit(d *TurnDraft) (result *TurnEdit, err error) {
	defer func() { err = fault.Wrap(err, "game.plan_turn_events", fault.Context{Resource: turnEventsResource}) }()
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
		offsets[i] = len(content)
		content = append(content, record.raw...)
	}
	content = append(content, 0, 0, 0, 0)
	for i, record := range d.records {
		if record.deployment == nil {
			continue
		}
		if !record.deployment.Current(s) {
			return nil, rom.ErrStale
		}
		for len(content)%4 != 0 {
			content = append(content, 0)
		}
		scriptOffset := len(content)
		script := d.original.format.reinforcementScript()
		content = append(content, script...)
		for len(content)%4 != 0 {
			content = append(content, 0)
		}
		unitOffset := len(content)
		content = append(content, record.deployment.payload(base+unitOffset)...)
		binary.LittleEndian.PutUint32(
			content[offsets[i]+4:],
			uint32(0x08000000+base+scriptOffset),
		) // #nosec G115 -- PlanAppend bounds the complete resource before application.
		binary.LittleEndian.PutUint32(
			content[scriptOffset+4:],
			uint32(0x08000000+base+unitOffset),
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
	result = &TurnEdit{
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
			turns, readErr := s.Adapter.ChapterTurns(
				s.image.Data,
				row.ID,
				row.Address,
			)
			if readErr == nil && turns.Pointer == site {
				result.uses = append(result.uses, row.ID)
			}
		}
	}
	return result, nil
}

func (s *Session) ApplyTurnEdit(edit *TurnEdit) (err error) {
	defer func() { err = fault.Wrap(err, "game.apply_turn_events", fault.Context{Resource: turnEventsResource}) }()
	if edit == nil || edit.document != s.Document ||
		!maps.Equal(edit.layouts, s.layouts) {
		return rom.ErrStale
	}
	if err := s.Document.ApplyEdit(
		edit.revision,
		turnEventsResource,
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
		return ErrTurnEvents
	}
	for _, table := range s.Tables {
		if table.Pointer >= 0 && site < table.Pointer+4 &&
			site+4 > table.Pointer {
			return ErrTurnEvents
		}
		if table.Err == nil {
			for _, row := range table.Rows {
				if site < row.Address+table.Stride && site+4 > row.Address {
					return ErrTurnEvents
				}
			}
		}
	}
	return nil
}
