package game

import (
	"bytes"
	"image"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

var (
	ErrDeployment = fault.Code{
		ID:   "game.invalid_deployment",
		Kind: fault.InvalidData,
	}
	ErrDeploymentScript = fault.Code{
		ID:   "game.incomplete_deployment_script",
		Kind: fault.Unsupported,
	}
)

const (
	maxDeploymentUnits = 256
	chapterEventsField = "chapter_events_id"
)

// DeploymentGroup identifies a pointer site, rather than a resource address.
// Two modes may share their current bytes while remaining separate edit targets.
type DeploymentGroup struct {
	Label                    string
	Pointer, Address, Stride int
	Units                    []DeploymentUnit
	Err                      error
	records                  []deploymentRecord
	terminator               []byte
	faction                  int
}

// DeploymentUnit contains the values used by chapter navigation and map marks.
// End is the last encoded movement coordinate, not an event simulation result.
type DeploymentUnit struct {
	Character, Class, Level, Faction int
	Start, End                       image.Point
	MovementCount                    int
	RelativeEnd                      bool
}

type ChapterDeployments struct {
	Groups   []DeploymentGroup
	Warnings []error
	ranges   []space.Range
}

type deploymentSlot struct {
	label           string
	offset, faction int
}

type deploymentFormat struct {
	root, stride, opening int
	kind                  deploymentEventKind
	loadHelper            int
	slots                 []deploymentSlot
}

func fe8Deployments(root, helper int) deploymentFormat {
	return deploymentFormat{
		root:       root,
		stride:     20,
		opening:    72,
		kind:       eventFE8,
		loadHelper: helper,
		slots: []deploymentSlot{
			{"deploy_player_normal", 40, 0},
			{"deploy_player_hard", 44, 0},
			{"deploy_skirmish_player_1", 48, 0},
			{"deploy_skirmish_player_2", 52, 0},
			{"deploy_skirmish_player_3", 56, 0},
			{"deploy_skirmish_enemy_1", 60, 2},
			{"deploy_skirmish_enemy_2", 64, 2},
			{"deploy_skirmish_enemy_3", 68, 2},
		},
	}
}

func fe7Deployments(root int, kind deploymentEventKind) deploymentFormat {
	return deploymentFormat{
		root:    root,
		stride:  16,
		opening: 56,
		kind:    kind,
		slots: []deploymentSlot{
			{"deploy_eliwood_enemy_normal", 24, 2},
			{"deploy_eliwood_enemy_hard", 28, 2},
			{"deploy_hector_enemy_normal", 32, 2},
			{"deploy_hector_enemy_hard", 36, 2},
			{"deploy_eliwood_player_normal", 40, 0},
			{"deploy_eliwood_player_hard", 44, 0},
			{"deploy_hector_player_normal", 48, 0},
			{"deploy_hector_player_hard", 52, 0},
		},
	}
}

func fe6Deployments(root int) deploymentFormat {
	return deploymentFormat{
		root:    root,
		stride:  16,
		opening: -1,
		kind:    eventFE6,
		slots: []deploymentSlot{
			{"deploy_enemy", 16, 2}, {"deploy_player", 20, 0},
		},
	}
}

func (s *Session) ChapterDeployments(
	id int,
) (result ChapterDeployments, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.chapter_deployments",
			fault.Context{Table: string(Chapters), RecordID: new(id)},
		)
	}()
	s.Refresh()
	if !s.Adapter.Match(s.image.Data) {
		return result, ErrVersion
	}
	_, row, _, err := s.locate(Chapters, id, chapterEventsField)
	if err != nil {
		return result, err
	}
	return s.Adapter.ChapterDeployments(s.image.Data, id, row.Address)
}

func (g gameFormat) ChapterDeployments(
	data []byte,
	chapter, address int,
) (ChapterDeployments, error) {
	for _, field := range g.chapters.fields {
		if field.ID == chapterEventsField &&
			within(data, address+field.Offset, 1) {
			return g.deployments.read(data, chapter, data[address+field.Offset])
		}
	}
	return ChapterDeployments{}, ErrLayout
}

func (f deploymentFormat) read(
	data []byte,
	chapter int,
	eventID byte,
) (result ChapterDeployments, err error) {
	ctx := fault.Context{
		Resource:   chapterEventsField,
		ResourceID: new(int(eventID)),
	}
	defer func() { err = fault.Wrap(err, "game.read_deployment_entries", ctx) }()
	base, err := f.eventHeader(data, eventID)
	if err != nil {
		return result, err
	}
	parsed := make(map[int]DeploymentGroup)
	for _, slot := range f.slots {
		result.Groups = append(
			result.Groups,
			f.group(data, base+slot.offset, slot.label, slot.faction, parsed),
		)
	}
	scanner := deploymentScanner{
		format:    f,
		data:      data,
		result:    &result,
		seen:      make(map[int]bool),
		active:    make(map[int]bool),
		parsed:    parsed,
		remaining: 8192,
		chapter:   chapter,
	}
	scanner.entries(base)
	return result, nil
}

func (f deploymentFormat) group(
	data []byte,
	site int,
	label string,
	faction int,
	parsed map[int]DeploymentGroup,
) (group DeploymentGroup) {
	group = DeploymentGroup{
		Label:      label,
		Pointer:    site,
		Stride:     f.stride,
		faction:    faction,
		terminator: make([]byte, 4),
	}
	ctx := fault.Context{
		Resource: label,
		Range:    &fault.Range{Offset: site, Length: 4, Size: len(data)},
	}
	fail := func() DeploymentGroup {
		group.Err = fault.New(
			ErrDeployment,
			"game.read_deployment_group",
			ctx,
			nil,
		)
		return group
	}
	if !within(data, site, 4) || site < 0xC0 || site%4 != 0 {
		return fail()
	}
	if u32(data, site) == 0 {
		return group
	}
	at, valid := pointer(data, site)
	if !valid || at < 0xC0 || at%4 != 0 {
		return fail()
	}
	group.Address = at
	if cached, found := parsed[at]; found {
		group.records, group.terminator = cached.records, cached.terminator
		group.Units = slices.Clone(cached.Units)
		group.Err = fault.Wrap(cached.Err, "game.deployment_entry", ctx)
		return group
	}
	defer func() { parsed[group.Address] = group }()
	for index := 0; index <= maxDeploymentUnits; index++ {
		if !within(data, at, 1) {
			return fail()
		}
		if data[at] == 0 {
			group.terminator = bytes.Clone(data[at:min(at+4, len(data))])
			return group
		}
		if index == maxDeploymentUnits || !within(data, at, f.stride) {
			return fail()
		}
		record := deploymentRecord{data: bytes.Clone(data[at : at+f.stride])}
		if f.stride == 20 && record.data[7] != 0 {
			count := int(record.data[7])
			path, valid := pointer(data, at+8)
			if !valid || path < 0xC0 || path%4 != 0 ||
				!within(data, path, count*8) {
				ctx.Resource = "deploy_movement"
				ctx.RecordID = new(index)
				return fail()
			}
			record.movement = bytes.Clone(data[path : path+count*8])
		}
		group.records = append(group.records, record)
		group.Units = append(group.Units, record.unit())
		at += f.stride
	}
	return fail()
}

func cloneDeploymentRecords(records []deploymentRecord) []deploymentRecord {
	result := slices.Clone(records)
	for i := range result {
		result[i].data = bytes.Clone(result[i].data)
		result[i].movement = bytes.Clone(result[i].movement)
	}
	return result
}
