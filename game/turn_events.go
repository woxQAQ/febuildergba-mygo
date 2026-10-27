package game

import (
	"bytes"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

var ErrTurnEvents = fault.Code{
	ID:   "game.invalid_turn_events",
	Kind: fault.InvalidData,
}

const (
	turnEventsResource = "turn_events"
	maxTurnEvents      = 256
)

// TurnEvent retains its native condition record independently of its script.
// Unknown condition kinds and scripts can be carried through table edits.
type TurnEvent struct {
	Address, Kind, Flag, Start, End, Phase int
	Route                                  uint32
	Script                                 int
	Raw                                    []byte
	Groups                                 []DeploymentGroup
	Warnings                               []error
}

type ChapterTurns struct {
	Pointer, Address int
	Events           []TurnEvent
	ranges           []space.Range
	format           deploymentFormat
}

func (f deploymentFormat) turnStride(kind int) int {
	if (f.kind == eventFE7J || f.kind == eventFE7U) && kind != 1 {
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

func (f deploymentFormat) readTurns(
	data []byte,
	site int,
) (result ChapterTurns, err error) {
	result.Pointer, result.format = site, f
	ctx := fault.Context{
		Resource: turnEventsResource,
		Range:    &fault.Range{Offset: site, Length: 4, Size: len(data)},
	}
	defer func() { err = fault.Wrap(err, "game.read_turn_events", ctx) }()
	if site < 0xC0 || site%4 != 0 || !within(data, site, 4) {
		return result, ErrTurnEvents
	}
	if u32(data, site) == 0 {
		return result, nil
	}
	at, valid := pointer(data, site)
	if !valid || at < 0xC0 || at%4 != 0 {
		return result, ErrTurnEvents
	}
	result.Address = at
	for i := 0; i <= maxTurnEvents; i++ {
		if !within(data, at, 4) {
			return result, ErrTurnEvents
		}
		if u32(data, at) == 0 {
			result.ranges = append(
				result.ranges,
				space.Range{Start: result.Address, End: at + 4},
			)
			return result, nil
		}
		kind := int(u16(data, at))
		length := f.turnStride(kind)
		if kind == 0 || i == maxTurnEvents || !within(data, at, length) {
			return result, ErrTurnEvents
		}
		event := TurnEvent{
			Address: at,
			Kind:    kind,
			Flag:    int(u16(data, at+2)),
			Start: int(
				data[at+8],
			),
			End:   int(data[at+9]),
			Phase: int(u16(data, at+10)),
			Raw:   bytes.Clone(data[at : at+length]),
		}
		event.Script, _ = pointer(data, at+4)
		if length == 16 {
			event.Route = u32(data, at+12)
		}
		result.Events = append(result.Events, event)
		at += length
	}
	return result, ErrTurnEvents
}

func (s *Session) ChapterTurns(chapter int) (result ChapterTurns, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.chapter_turns",
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
	return s.Adapter.ChapterTurns(s.image.Data, chapter, row.Address)
}

func (g gameFormat) ChapterTurns(
	data []byte,
	chapter, address int,
) (ChapterTurns, error) {
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
			return ChapterTurns{}, err
		}
		result, err := g.deployments.readTurns(data, header)
		if err != nil {
			return result, err
		}
		parsed := make(map[int]DeploymentGroup)
		for i := range result.Events {
			event := &result.Events[i]
			resources := ChapterDeployments{}
			scanner := deploymentScanner{
				format:    g.deployments,
				data:      data,
				result:    &resources,
				seen:      make(map[int]bool),
				active:    make(map[int]bool),
				parsed:    parsed,
				remaining: 8192,
				chapter:   chapter,
			}
			if event.Script < 0xC0 || event.Script%4 != 0 {
				scanner.warn(event.Address + 4)
			} else {
				scanner.walk(
					event.Script,
					[16]int{},
					0,
					chapter,
					"deploy_turn_event",
				)
			}
			event.Groups, event.Warnings = resources.Groups, resources.Warnings
			result.ranges = append(result.ranges, resources.ranges...)
		}
		return result, nil
	}
	return ChapterTurns{}, ErrLayout
}
