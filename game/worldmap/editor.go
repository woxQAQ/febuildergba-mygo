package worldmap

import (
	"encoding/binary"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// Field identifies a native node value. Inventory pointers and the padding byte
// have no setters; shop edits retain their own workflow and history operation.
type Field uint8

const (
	Name Field = iota
	X
	Y
	EirikaChapter
	EphraimChapter
	BranchFlag
	EirikaNext
	EphraimNext
	EirikaAlternate
	EphraimAlternate
	ClearedIcon
	UnclearedIcon
	Placement
	Encounters
	ShipTravel
)

type nodeField struct {
	offset, width, minimum, maximum int
	mask                            byte
	signed                          bool
}

var nodeFields = [...]nodeField{
	Name:             {28, 2, 0, 65535, 0, false},
	X:                {24, 2, -32768, 32767, 0, true},
	Y:                {26, 2, -32768, 32767, 0, true},
	EirikaChapter:    {4, 1, 0, 255, 0, false},
	EphraimChapter:   {5, 1, 0, 255, 0, false},
	BranchFlag:       {6, 2, -32768, 32767, 0, true},
	EirikaNext:       {8, 1, -1, NodeCount - 1, 0, true},
	EphraimNext:      {9, 1, -1, NodeCount - 1, 0, true},
	EirikaAlternate:  {10, 1, -1, NodeCount - 1, 0, true},
	EphraimAlternate: {11, 1, -1, NodeCount - 1, 0, true},
	// Native drawing selects +2 when the cleared-state bit is set, +3 otherwise.
	ClearedIcon:   {2, 1, 0, 16, 0, false},
	UnclearedIcon: {3, 1, 0, 16, 0, false},
	Placement:     {0, 1, 0, 3, 0, false},
	Encounters:    {1, 1, 1, 3, 0, true},
	ShipTravel:    {30, 1, 0, 1, 1, false},
}

func (f Field) Bounds() (int, int) {
	if int(f) >= len(nodeFields) {
		return 0, 0
	}
	return nodeFields[f].minimum, nodeFields[f].maximum
}

func (n Node) Value(field Field) int {
	if int(field) >= len(nodeFields) {
		return 0
	}
	f := nodeFields[field]
	if f.mask != 0 {
		return int(n.raw[f.offset] & f.mask)
	}
	if f.width == 1 {
		if f.signed {
			return int(
				int8(n.raw[f.offset]),
			) // #nosec G115 -- Native signed-byte interpretation.
		}
		return int(n.raw[f.offset])
	}
	value := binary.LittleEndian.Uint16(n.raw[f.offset:])
	if f.signed {
		return int(
			int16(value),
		) // #nosec G115 -- Native signed-halfword interpretation.
	}
	return int(value)
}

var ErrReference = fault.Code{
	ID:   "game.worldmap_reference_conflict",
	Kind: fault.Conflict,
}

// Source validates references in the current layout and authorizes in-place
// writes against the entire session's confirmed resource claims.
type Source struct {
	ValidateChapter func([]byte, int) error
	ValidateText    func([]byte, uint16) error
	ProtectRange    func([]byte, space.Owner, space.Range) error
	ProtectPointer  func([]byte, space.Owner, int) error
}

type Editor struct {
	context    *edit.Context
	format     Format
	source     Source
	stamp      edit.Stamp
	nodes      []Node
	err        error
	routeStamp edit.Stamp
	routes     []Route
	routeErr   error
	scenery    sceneCache
}

func New(context *edit.Context, format Format, source Source) *Editor {
	return &Editor{context: context, format: format, source: source}
}

// Available reports a declared native node layout, not whether the current
// game flow uses overworld travel. Readability is checked when loading records.
func (e *Editor) Available() bool { return e.format.Pointer != 0 }

func (e *Editor) Nodes() ([]Node, error) {
	if !e.stamp.Current(e.context) {
		image := e.context.Capture()
		e.stamp = image.Stamp()
		e.nodes, e.err = nil, e.context.Validate(image.Data)
		if e.err == nil {
			e.nodes, e.err = e.format.Nodes(image.Data)
		}
	}
	return slices.Clone(e.nodes), e.err
}

// Draft owns one fixed node; separate nodes and shop inventories keep independent
// drafts. Values outside the native ranges are retained until explicitly replaced.
type Draft struct {
	stamp          edit.Stamp
	original, node Node
}

func (e *Editor) NewDraft(id int) (*Draft, error) {
	nodes, err := e.Nodes()
	if err != nil {
		return nil, err
	}
	if id < 0 || id >= len(nodes) {
		return nil, edit.ErrValue
	}
	return &Draft{stamp: e.stamp, original: nodes[id], node: nodes[id]}, nil
}

func (d *Draft) Current(
	e *Editor,
) bool {
	return d != nil && d.stamp.Current(e.context)
}
func (d *Draft) Modified() bool { return d.node.raw != d.original.raw }
func (d *Draft) Node() Node     { return d.node }

func (d *Draft) Set(field Field, value int) error {
	if int(field) >= len(nodeFields) {
		return edit.ErrValue
	}
	f := nodeFields[field]
	if value != d.original.Value(field) &&
		(value < f.minimum || value > f.maximum) {
		return edit.ErrValue
	}
	if f.mask != 0 {
		d.node.raw[f.offset] = d.node.raw[f.offset]&^f.mask | byte(
			value&0xFF,
		)&f.mask
	} else if f.width == 1 {
		d.node.raw[f.offset] = byte(value & 0xFF)
	} else {
		binary.LittleEndian.PutUint16(
			d.node.raw[f.offset:],
			uint16(value&0xFFFF),
		)
	}
	// Packing retains the native two's-complement bits after range validation.
	d.node.Name = binary.LittleEndian.Uint16(d.node.raw[28:])
	return nil
}

type Change struct {
	Field         Field
	Before, After int
}

type Plan struct {
	commit  *edit.Commit
	node    Node
	changes []Change
}

func (p *Plan) Current(
	e *Editor,
) bool {
	return p != nil && p.commit.Current(e.context)
}
func (p *Plan) Node() Node        { return p.node }
func (p *Plan) Changes() []Change { return slices.Clone(p.changes) }

func (e *Editor) Plan(d *Draft) (plan *Plan, err error) {
	defer func() { err = fault.Wrap(err, "worldmap.plan_node", fault.Context{Resource: Resource}) }()
	if d == nil {
		return nil, rom.ErrStale
	}
	if err = d.stamp.Check(e.context); err != nil {
		return nil, err
	}
	if !d.Modified() || e.source.ProtectRange == nil {
		return nil, edit.ErrValue
	}
	image := e.context.Capture()
	if err = e.context.Validate(image.Data); err != nil {
		return nil, err
	}
	node := d.node
	if err = e.source.ProtectRange(image.Data, node.Owner(), space.Range{
		Start: node.Address, End: node.Address + NodeSize,
	}); err != nil {
		return nil, fault.New(
			ErrReference,
			"worldmap.protect_node",
			fault.Context{Resource: Resource, RecordID: new(node.ID)},
			err,
		)
	}
	plan = &Plan{node: node}
	change := rom.Edit{Size: len(image.Data)}
	for index, f := range nodeFields {
		field := Field(index)
		before, after := d.original.Value(field), node.Value(field)
		if before == after {
			continue
		}
		switch field {
		case Name:
			if e.source.ValidateText == nil {
				return nil, edit.ErrLayout
			}
			err = e.source.ValidateText(image.Data, node.Name)
		case EirikaChapter, EphraimChapter:
			if e.source.ValidateChapter == nil {
				return nil, edit.ErrLayout
			}
			err = e.source.ValidateChapter(image.Data, after)
		}
		if err != nil {
			return nil, fault.Wrap(
				err,
				"worldmap.validate_reference",
				fault.Context{
					Resource: Resource,
					RecordID: new(node.ID),
					Range: &fault.Range{
						Offset: node.Address + f.offset,
						Length: f.width,
					},
				},
			)
		}
		plan.changes = append(
			plan.changes,
			Change{Field: field, Before: before, After: after},
		)
		change.Changes = append(change.Changes, rom.Change{
			Offset: node.Address + f.offset,
			Data:   node.raw[f.offset : f.offset+f.width],
		})
	}
	plan.commit = edit.NewCommit(d.stamp, Resource, change)
	return plan, nil
}

func (e *Editor) Apply(plan *Plan) error {
	if plan == nil {
		return rom.ErrStale
	}
	return fault.Wrap(
		plan.commit.Apply(e.context),
		"worldmap.apply_node",
		fault.Context{Resource: Resource},
	)
}
