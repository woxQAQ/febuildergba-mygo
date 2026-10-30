package event

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

var ErrDeploymentScript = fault.Code{
	ID:   "game.incomplete_deployment_script",
	Kind: fault.Unsupported,
}

const HeaderField = "chapter_events_id"

type Deployments struct {
	binding  edit.Stamp
	Groups   []deployment.Group
	Warnings []error
	ranges   []space.Range
}

type deploymentSlot struct {
	label           string
	offset, faction int
}

type Format struct {
	root, opening        int
	units                deployment.Format
	ending, defeat, rout int
	kind                 deploymentEventKind
	loadHelper           int
	slots                []deploymentSlot
}

func FE8(root, helper, defeat int) Format {
	return Format{
		root:       root,
		units:      deployment.Format{Stride: 20},
		opening:    72,
		ending:     76,
		defeat:     defeat,
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

func FE7(root int, english bool) Format {
	kind := eventFE7J
	if english {
		kind = eventFE7U
	}
	defeat, rout := 0xD6F71C, 0x0807AA5D
	if kind == eventFE7U {
		defeat, rout = 0xCA749C, 0x08079FE9
	}
	return Format{
		root:    root,
		units:   deployment.Format{Stride: 16},
		opening: 56,
		ending:  60,
		defeat:  defeat,
		rout:    rout,
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

func FE6(root int) Format {
	return Format{
		root:    root,
		units:   deployment.Format{Stride: 16},
		opening: -1,
		ending:  24,
		defeat:  0x66AC6C,
		rout:    0x0806C2F9,
		kind:    eventFE6,
		slots: []deploymentSlot{
			{"deploy_enemy", 16, 2}, {"deploy_player", 20, 0},
		},
	}
}

func (d Deployments) Current(
	editor *Editor,
) bool {
	return d.binding.Current(editor.context)
}

func (s *Editor) Deployments(
	id int,
) (result Deployments, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.chapter_deployments",
			fault.Context{Table: string(records.Chapters), RecordID: new(id)},
		)
	}()
	s.Refresh()
	defer func() { result.binding = s.context.Bind(s.image.Revision) }()
	if s.context.Validate(s.image.Data) != nil {
		return result, edit.ErrVersion
	}
	_, row, field, err := s.records.Resolve(
		records.Chapters,
		id,
		HeaderField,
	)
	if err != nil {
		return result, err
	}
	return s.format.ReadDeployments(
		s.image.Data,
		id,
		s.image.Data[row.Address+field.Offset],
	)
}

func (f Format) ReadDeployments(
	data []byte,
	chapter int,
	eventID byte,
) (result Deployments, err error) {
	ctx := fault.Context{
		Resource:   HeaderField,
		ResourceID: new(int(eventID)),
	}
	defer func() { err = fault.Wrap(err, "game.read_deployment_entries", ctx) }()
	base, err := f.eventHeader(data, eventID)
	if err != nil {
		return result, err
	}
	parsed := deployment.NewReader(data, f.units)
	for _, slot := range f.slots {
		result.Groups = append(
			result.Groups,
			parsed.Group(base+slot.offset, slot.label, slot.faction),
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

func (f Format) Root() int { return f.root }

func (f Format) HeaderSize() int {
	last := f.slots[len(f.slots)-1].offset + 4
	if f.opening >= 0 {
		last = f.opening + 8
	}
	return max(last, f.ending+4)
}

func (d Deployments) Regions() []space.Range { return slices.Clone(d.ranges) }
