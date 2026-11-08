package arena

import (
	"bytes"
	"encoding/binary"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

// Class supplies the native promotion tier from the effective record layout.
type (
	Class struct {
		ID       int
		Promoted bool
	}
	Source struct {
		Classes        func([]byte) ([]Class, error)
		Items          func([]byte) ([]int, error)
		Units          func([]byte) ([]int, error)
		ProtectPointer func([]byte, romwrite.Owner, int) error
	}
)

type Editor struct {
	context *edit.Context
	format  Format
	source  Source
}

func New(context *edit.Context, format Format, source Source) *Editor {
	return &Editor{context, format, source}
}
func (e *Editor) Groups() []Kind { return e.format.Groups() }

// Draft keeps base weapons and their upgrade lookup together. Other groups can
// be edited independently when an unrelated arena resource is unreadable.
type Draft struct {
	binding edit.Stamp
	tables  []*table
}

func (e *Editor) NewDraft(group Kind) (draft *Draft, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"arena.read",
			fault.Context{Resource: "arena", ResourceID: new(int(group))},
		)
	}()
	if !slices.Contains(e.Groups(), group) {
		return nil, edit.ErrValue
	}
	image := e.context.Capture()
	if err = e.context.Validate(image.Data); err != nil {
		return nil, err
	}
	kinds := []Kind{group}
	if group == BaseWeapons {
		kinds = append(kinds, Upgrades)
	}
	draft = &Draft{binding: image.Stamp()}
	for _, kind := range kinds {
		t, readErr := e.format.read(image.Data, kind)
		if readErr != nil {
			return nil, readErr
		}
		draft.tables = append(draft.tables, t)
	}
	return draft, nil
}

func (d *Draft) Current(
	e *Editor,
) bool {
	return d != nil && d.binding.Current(e.context)
}

func (d *Draft) Modified() bool {
	for _, t := range d.tables {
		if !bytes.Equal(t.raw, t.original) {
			return true
		}
	}
	return false
}

func (d *Draft) Kinds() []Kind {
	var kinds []Kind
	for _, t := range d.tables {
		kinds = append(kinds, t.kind)
	}
	return kinds
}

func (d *Draft) table(kind Kind) *table {
	for _, t := range d.tables {
		if t.kind == kind {
			return t
		}
	}
	return nil
}

func (d *Draft) Raw(kind Kind) []byte {
	if t := d.table(kind); t != nil {
		return bytes.Clone(t.raw)
	}
	return nil
}

func (d *Draft) Address(kind Kind) int {
	if t := d.table(kind); t != nil {
		return t.address
	}
	return 0
}

// Row.Key survives class insertion and reordering. Opaque weapon separators,
// the FF marker and the unused staff slot remain read-only.
type Row struct {
	Key, Index int
	Value      uint16
	Editable   bool
	// WeaponType is the native type index for base weapon slots, or -1.
	WeaponType int
}

func (d *Draft) Rows(kind Kind) []Row {
	t := d.table(kind)
	if t == nil {
		return nil
	}
	rows := make([]Row, t.count())
	for i := range rows {
		value := uint16(t.raw[i])
		if kind == LinkUnits {
			value = gba.Uint16(t.raw, i*2)
		}
		editable := true
		if kind == BaseWeapons {
			editable = i != 4
		}
		if kind == Upgrades {
			editable = t.original[i] != 0 && t.original[i] != 0xFF
		}
		weaponType := -1
		if kind == BaseWeapons {
			weaponType = i
		}
		rows[i] = Row{
			WeaponType: weaponType,
			Key:        t.keys[i],
			Index:      i,
			Value:      value,
			Editable:   editable,
		}
	}
	return rows
}

func (d *Draft) Set(kind Kind, index, value int) error {
	t := d.table(kind)
	if t == nil || index < 0 || index >= t.count() || value < 0 || value > 255 {
		return edit.ErrValue
	}
	row := d.Rows(kind)[index]
	if !row.Editable || (kind != LinkUnits && value == 0) ||
		(kind == Upgrades && value == 255) {
		return edit.ErrValue
	}
	if kind == LinkUnits {
		binary.LittleEndian.PutUint16(t.raw[index*2:], uint16(value))
	} else {
		t.raw[index] = byte(value)
	}
	return nil
}

func (d *Draft) Insert(kind Kind, index, value int) error {
	t := d.table(kind)
	if !kind.ClassList() || t == nil || index < 0 || index > t.count() ||
		t.count() >= MaxClasses ||
		value < 1 ||
		value > 255 {
		return edit.ErrValue
	}
	t.raw = slices.Insert(t.raw, index, byte(value))
	t.keys = slices.Insert(t.keys, index, t.nextKey)
	t.nextKey++
	return nil
}

func (d *Draft) Delete(kind Kind, index int) error {
	t := d.table(kind)
	if !kind.ClassList() || t == nil || index < 0 || index >= t.count() {
		return edit.ErrValue
	}
	t.raw = slices.Delete(t.raw, index, index+1)
	t.keys = slices.Delete(t.keys, index, index+1)
	return nil
}

func (d *Draft) Move(kind Kind, from, to int) error {
	t := d.table(kind)
	if !kind.ClassList() || t == nil || from < 0 || to < 0 ||
		from >= t.count() ||
		to >= t.count() {
		return edit.ErrValue
	}
	value, key := t.raw[from], t.keys[from]
	t.raw = slices.Insert(slices.Delete(t.raw, from, from+1), to, value)
	t.keys = slices.Insert(slices.Delete(t.keys, from, from+1), to, key)
	return nil
}

type Change struct {
	Kind                      Kind
	Before, Destination       int
	Pointers                  []int
	Shared                    []Kind
	BeforeBytes, AfterBytes   []byte
	BeforeValues, AfterValues []uint16
}
type Plan struct {
	commit  *edit.Commit
	changes []Change
}

func (p *Plan) Current(
	e *Editor,
) bool {
	return p != nil && p.commit.Current(e.context)
}

func (p *Plan) Changes() []Change {
	result := slices.Clone(p.changes)
	for i := range result {
		result[i].Pointers = slices.Clone(result[i].Pointers)
		result[i].Shared = slices.Clone(result[i].Shared)
		result[i].BeforeBytes = bytes.Clone(result[i].BeforeBytes)
		result[i].AfterBytes = bytes.Clone(result[i].AfterBytes)
		result[i].BeforeValues = slices.Clone(result[i].BeforeValues)
		result[i].AfterValues = slices.Clone(result[i].AfterValues)
	}
	return result
}
func (p *Plan) Size() int { return p.commit.Size() }

func (e *Editor) Plan(d *Draft) (plan *Plan, err error) {
	defer func() { err = fault.Wrap(err, "arena.plan", fault.Context{Resource: "arena"}) }()
	if d == nil {
		return nil, rom.ErrStale
	}
	if err = d.binding.Check(e.context); err != nil {
		return nil, err
	}
	if !d.Modified() {
		return nil, edit.ErrValue
	}
	if e.source.ProtectPointer == nil {
		return nil, edit.ErrLayout
	}
	image := e.context.Capture()
	if err = e.context.Validate(image.Data); err != nil {
		return nil, err
	}
	if err = e.validate(image.Data, d); err != nil {
		return nil, err
	}
	builder := romwrite.NewBuilder(
		image.Data,
		romwrite.Protection{Pointer: e.source.ProtectPointer},
	)
	plan = &Plan{}
	for _, t := range d.tables {
		if bytes.Equal(t.original, t.raw) {
			continue
		}
		pointers := make([]romwrite.PointerSite, len(t.pointers))
		for i, site := range t.pointers {
			pointers[i] = romwrite.PointerSite{
				Owner:   t.kind.Owner(),
				Address: site,
				ExpectedWord: uint32( // #nosec G115 -- The table address was read from a valid GBA ROM pointer.
					0x08000000 + t.address,
				),
			}
		}
		destination, appendErr := builder.Replace(t.raw, pointers...)
		if appendErr != nil {
			return nil, appendErr
		}
		change := Change{
			Kind:        t.kind,
			Before:      t.address,
			Destination: destination,
			Pointers:    slices.Clone(t.pointers),
			BeforeBytes: bytes.Clone(t.original),
			AfterBytes:  bytes.Clone(t.raw),
			BeforeValues: tableValues(
				t.kind,
				t.original,
			),
			AfterValues: tableValues(t.kind, t.raw),
		}
		for kind := Melee; kind <= LinkUnits; kind++ {
			if kind == t.kind {
				continue
			}
			for _, site := range e.format.pointers(kind) {
				at, valid := gba.Pointer(image.Data, site)
				if !valid {
					continue
				}
				raw, _ := readPayload(image.Data, kind, at)
				if at < t.address+len(t.original) && at+len(raw) > t.address {
					change.Shared = append(change.Shared, kind)
					break
				}
			}
		}
		plan.changes = append(plan.changes, change)
	}
	plan.commit = edit.NewCommit(d.binding, "arena", builder.Plan().Edit)
	return plan, nil
}

func (e *Editor) Apply(plan *Plan) error {
	if plan == nil {
		return rom.ErrStale
	}
	return fault.Wrap(
		plan.commit.Apply(e.context),
		"arena.apply",
		fault.Context{Resource: "arena"},
	)
}

func tableValues(kind Kind, raw []byte) []uint16 {
	t := table{kind: kind, raw: raw}
	values := make([]uint16, t.count())
	for i := range values {
		if kind == LinkUnits {
			values[i] = gba.Uint16(raw, i*2)
		} else {
			values[i] = uint16(raw[i])
		}
	}
	return values
}

func (e *Editor) validate(data []byte, d *Draft) error {
	first := d.tables[0]
	if first.kind.ClassList() {
		classes, err := e.source.Classes(data)
		if err != nil {
			return err
		}
		tiers := [2]bool{}
		for _, id := range first.raw[:len(first.raw)-1] {
			index := slices.IndexFunc(
				classes,
				func(c Class) bool { return c.ID == int(id) },
			)
			if index < 0 {
				return ErrClasses
			}
			tier := 0
			if classes[index].Promoted {
				tier = 1
			}
			tiers[tier] = true
		}
		if !tiers[0] || !tiers[1] {
			return ErrClasses
		}
		return nil
	}
	if first.kind == LinkUnits {
		units, err := e.source.Units(data)
		if err != nil {
			return err
		}
		for i := 0; i < linkSize; i += 2 {
			value := gba.Uint16(first.raw, i)
			if value != gba.Uint16(first.original, i) && value != 0 &&
				!slices.Contains(units, int(value)) {
				return edit.ErrValue
			}
		}
		return nil
	}
	items, err := e.source.Items(data)
	if err != nil {
		return err
	}
	base, upgrades := d.table(BaseWeapons), d.table(Upgrades)
	end := bytes.IndexByte(upgrades.raw, 0xFF)
	// A match immediately before FF would return FF as an item. Every base
	// weapon must be found because the native no-match return is undefined.
	if end < 1 || upgrades.raw[end-1] != 0 {
		return ErrWeapons
	}
	for _, t := range d.tables {
		for i, value := range t.raw {
			if value != t.original[i] && !slices.Contains(items, int(value)) {
				return edit.ErrValue
			}
		}
	}
	for i, id := range base.raw {
		if i == 4 {
			continue
		}
		if id == 0 || id == 255 || bytes.IndexByte(upgrades.raw[:end], id) < 0 {
			return ErrWeapons
		}
	}
	return nil
}
