package game

import (
	"bytes"
	"image"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

var ErrEventConditions = fault.Code{
	ID:   "game.invalid_event_conditions",
	Kind: fault.InvalidData,
}

const (
	eventConditionsResource = "event_conditions"
	maxEventConditions      = 256
)

// EventTable selects a native condition list in the chapter event header.
// AlwaysEvents includes area, flag and ASM conditions; unrelated rows are retained.
type EventTable uint8

const (
	TurnEvents EventTable = iota
	AlwaysEvents
	TalkEvents
)

var eventTables = [...]EventTable{TurnEvents, TalkEvents, AlwaysEvents}

func (table EventTable) offset() int {
	switch table {
	case AlwaysEvents:
		return 12
	case TalkEvents:
		return 4
	default:
		return 0
	}
}

func (f deploymentFormat) condition(table EventTable) int {
	switch table {
	case AlwaysEvents:
		return 0x0B
	case TalkEvents:
		if f.kind == eventFE6 {
			return 4
		}
		return 3
	default:
		return 2
	}
}

func (table EventTable) source() string {
	switch table {
	case AlwaysEvents:
		return "deploy_area_event"
	case TalkEvents:
		return "deploy_talk_event"
	default:
		return "deploy_turn_event"
	}
}

func (event EventCondition) source(table EventTable) string {
	if table == AlwaysEvents && event.Kind != 11 {
		return "deploy_objective_event"
	}
	return table.source()
}

// EventCondition retains its native bytes independently of its script. Typed
// parameters are decoded views, and only confirmed fields are offered for edits.
type EventCondition struct {
	Address, Kind, Flag, Script           int
	Start, End, Phase                     int
	Route                                 uint32
	X1, Y1, X2, Y2                        int
	Initiator, Target, TalkMode, TalkFlag int
	TriggerFlag                           uint32
	Rout                                  bool
	Texts                                 []uint16
	Raw                                   []byte
	Groups                                []DeploymentGroup
	Warnings                              []error
	reversedArea                          bool
}

// Area returns map cell bounds with an exclusive maximum. The native endpoints
// are inclusive. FE8 accepts reversed endpoints; FE6/7 require ordered endpoints.
func (event EventCondition) Area() image.Rectangle {
	if event.Kind != 0x0B {
		return image.Rectangle{}
	}
	x1, y1, x2, y2 := event.X1, event.Y1, event.X2, event.Y2
	if event.reversedArea {
		x1, x2 = min(x1, x2), max(x1, x2)
		y1, y2 = min(y1, y2), max(y1, y2)
	}
	if x1 > x2 || y1 > y2 {
		return image.Rectangle{}
	}
	return image.Rect(x1, y1, x2+1, y2+1)
}

func (event *EventCondition) decode(raw []byte, f deploymentFormat) {
	event.Raw = bytes.Clone(raw)
	event.reversedArea = f.kind == eventFE8
	event.TriggerFlag = u32(raw, 8)
	if f.kind != eventFE8 {
		event.TriggerFlag &= 0xFFFF
	}
	event.Rout = f.rout != 0 && int(u32(raw, 8)) == f.rout &&
		((f.kind == eventFE6 && u16(raw, 0) == 13) ||
			(f.kind != eventFE6 && u16(raw, 0) == 14))
	event.Kind, event.Flag = int(u16(raw, 0)), int(u16(raw, 2))
	event.Start, event.End, event.Phase = int(
		raw[8],
	), int(
		raw[9],
	), int(
		u16(raw, 10),
	)
	event.Initiator, event.Target = int(raw[8]), int(raw[9])
	event.TalkMode, event.TalkFlag = 0, 0
	if len(raw) == 16 {
		event.Route = u32(raw, 12)
		event.TalkMode, event.TalkFlag = int(u16(raw, 12)), int(u16(raw, 14))
		if f.kind == eventFE7J || f.kind == eventFE7U {
			event.TalkMode &= 0xFF
		}
	}
	// Native area coordinates use signed bytes, including off-map bounds.
	event.X1, event.Y1, event.X2, event.Y2 = int(
		int8(raw[8]),
	), int(
		int8(raw[9]),
	), int(
		int8(raw[10]),
	), int(
		int8(raw[11]),
	) // #nosec G115 -- Explicit native signed-byte interpretation.
}

type ChapterEvents struct {
	Pointer, Address int
	Table            EventTable
	Events           []EventCondition
	ranges           []space.Range
	format           deploymentFormat
	ending           *EventCondition
}

func (f deploymentFormat) eventStride(table EventTable, kind int) int {
	if table == TalkEvents && f.kind != eventFE6 {
		return 16
	}
	if table == TurnEvents && (f.kind == eventFE7J || f.kind == eventFE7U) &&
		kind != 1 {
		return 16
	}
	return 12
}

func (f deploymentFormat) eventHeader(data []byte, eventID byte) (int, error) {
	list, ok := pointer(data, f.root)
	if !ok || list < 0xC0 || list%4 != 0 {
		return 0, ErrDeployment
	}
	header, ok := pointer(data, list+int(eventID)*4)
	if !ok || header < 0xC0 || header%4 != 0 {
		return 0, ErrDeployment
	}
	return header, nil
}

func (f deploymentFormat) readEvents(
	data []byte,
	site int,
	table EventTable,
) (result ChapterEvents, err error) {
	result.Pointer, result.Table, result.format = site, table, f
	ctx := fault.Context{
		Resource: eventConditionsResource,
		Range:    &fault.Range{Offset: site, Length: 4, Size: len(data)},
	}
	defer func() { err = fault.Wrap(err, "game.read_event_conditions", ctx) }()
	if table > TalkEvents || site < 0xC0 || site%4 != 0 ||
		!within(data, site, 4) {
		return result, ErrEventConditions
	}
	if u32(data, site) == 0 {
		return result, nil
	}
	at, valid := pointer(data, site)
	if !valid || at < 0xC0 || at%4 != 0 {
		return result, ErrEventConditions
	}
	result.Address = at
	for i := 0; i <= maxEventConditions; i++ {
		if !within(data, at, 4) {
			return result, ErrEventConditions
		}
		if u32(data, at) == 0 {
			result.ranges = append(
				result.ranges,
				space.Range{Start: result.Address, End: at + 4},
			)
			return result, nil
		}
		kind := int(u16(data, at))
		length := f.eventStride(table, kind)
		if kind == 0 || i == maxEventConditions || !within(data, at, length) {
			return result, ErrEventConditions
		}
		event := EventCondition{Address: at, reversedArea: f.kind == eventFE8}
		event.decode(data[at:at+length], f)
		event.Script, _ = pointer(data, at+4)
		if u32(data, at+4) == 1 {
			event.Script = 1 // Native marker: set the completion flag without a script.
		}
		result.Events = append(result.Events, event)
		at += length
	}
	return result, ErrEventConditions
}

func (s *Session) ChapterEvents(
	chapter int,
	table EventTable,
) (result ChapterEvents, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.chapter_events",
			fault.Context{Table: string(Chapters), RecordID: new(chapter)},
		)
	}()
	s.Refresh()
	if !s.Adapter.Match(s.image.Data) {
		return result, ErrVersion
	}
	_, row, _, err := s.locate(Chapters, chapter, chapterEventsField)
	if err != nil {
		return result, err
	}
	return s.Adapter.ChapterEvents(s.image.Data, chapter, row.Address, table)
}

func (g gameFormat) ChapterEvents(
	data []byte,
	chapter, address int,
	table EventTable,
) (ChapterEvents, error) {
	if table > TalkEvents {
		return ChapterEvents{}, ErrEventConditions
	}
	for _, field := range g.chapters.fields {
		if field.ID != chapterEventsField ||
			!within(data, address+field.Offset, 1) {
			continue
		}
		header, err := g.deployments.eventHeader(
			data,
			data[address+field.Offset],
		)
		if err != nil {
			return ChapterEvents{}, err
		}
		result, err := g.deployments.readEvents(
			data,
			header+table.offset(),
			table,
		)
		if err != nil {
			return result, err
		}
		parsed := make(map[int]DeploymentGroup)
		for i := range result.Events {
			result.ranges = append(
				result.ranges,
				g.deployments.eventResources(
					data,
					chapter,
					table,
					&result.Events[i],
					parsed,
				)...)
		}
		if table == AlwaysEvents {
			site := header + g.deployments.ending
			if within(data, site, 4) {
				result.ranges = append(
					result.ranges,
					space.Range{Start: site, End: site + 4},
				)
				if script, valid := pointer(
					data,
					site,
				); valid && script >= 0xC0 &&
					script%4 == 0 {
					result.ending = &EventCondition{
						Address: site,
						Script:  script,
					}
					result.ranges = append(
						result.ranges,
						g.deployments.eventResources(
							data,
							chapter,
							table,
							result.ending,
							parsed,
						)...)
				}
			}
		}
		return result, nil
	}
	return ChapterEvents{}, ErrLayout
}
