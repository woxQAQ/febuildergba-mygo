package support

import (
	"bytes"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

// BonusKind follows the native six unsigned-byte bonus fields. Values are the
// raw contributions per support level; the engine halves the accumulated sum.
type BonusKind uint8

const (
	AttackBonus BonusKind = iota
	DefenseBonus
	HitBonus
	AvoidBonus
	CriticalBonus
	DodgeBonus
)

const (
	bonusStride   = 8
	maxBonusRows  = 256
	bonusResource = "support_bonuses"
)

// AffinityBonuses retains native row identity, including repeated affinity IDs.
// Only the first matching row contributes to the engine's lookup.
type AffinityBonuses struct {
	Index, Address int
	Affinity       uint8
	Values         [6]uint8
	Effective      bool
}

type bonusTable struct {
	address int
	raw     []byte
	err     error
}

func (f Format) bonuses(data []byte) (table bonusTable) {
	at, valid := gba.Pointer(data, f.BonusPointer)
	if !valid || at < 0xC0 || at%4 != 0 {
		table.err = ErrData
		return
	}
	table.address = at
	for index := 0; index <= maxBonusRows; index++ {
		row := at + index*bonusStride
		if !gba.Within(data, row, bonusStride) {
			break
		}
		table.raw = append(table.raw, data[row:row+bonusStride]...)
		if data[row] == 0 {
			return
		}
	}
	table.err = ErrData
	return
}

func (f Format) bonusClaims(data []byte) []romwrite.Claim {
	if f.BonusPointer < 0xC0 || !gba.Within(data, f.BonusPointer, 4) {
		return nil
	}
	owner := romwrite.Owner{Resource: bonusResource}
	result := []romwrite.Claim{{
		Start: f.BonusPointer, End: f.BonusPointer + 4,
		Owner: owner, Site: true,
	}}
	if table := f.bonuses(data); len(table.raw) > 0 {
		result = append(result, romwrite.Claim{
			Start: table.address,
			End:   table.address + len(table.raw),
			Owner: owner,
		})
	}
	return result
}

// BonusDraft owns the entire affinity table. Relationship and talk tables are
// independent inputs and are not required to read or edit its six values.
type BonusDraft struct {
	binding       edit.Stamp
	address       int
	original, raw []byte
}

func (e *Editor) NewBonusDraft() (*BonusDraft, error) {
	image := e.context.Capture()
	if err := e.context.Validate(image.Data); err != nil {
		return nil, err
	}
	table := e.format.bonuses(image.Data)
	if table.err != nil {
		return nil, fault.Wrap(table.err, "support.read_bonuses", fault.Context{
			Resource: bonusResource,
			Range: &fault.Range{
				Offset: e.format.BonusPointer,
				Length: 4,
				Size:   len(image.Data),
			},
		})
	}
	return &BonusDraft{
		binding: image.Stamp(), address: table.address,
		original: table.raw, raw: bytes.Clone(table.raw),
	}, nil
}

func (d *BonusDraft) Current(
	e *Editor,
) bool {
	return d != nil && d.binding.Current(e.context)
}

func (d *BonusDraft) Modified() bool { return !bytes.Equal(d.original, d.raw) }
func (d *BonusDraft) Address() int   { return d.address }
func (d *BonusDraft) Raw() []byte    { return bytes.Clone(d.raw) }

func (d *BonusDraft) Rows() []AffinityBonuses {
	rows := make([]AffinityBonuses, len(d.raw)/bonusStride-1)
	seen := [256]bool{}
	for index := range rows {
		at := index * bonusStride
		id := d.raw[at]
		rows[index] = AffinityBonuses{
			Index:     index,
			Address:   d.address + at,
			Affinity:  id,
			Effective: !seen[id],
			Values:    [6]uint8(d.raw[at+1 : at+7]),
		}
		seen[id] = true
	}
	return rows
}

func (d *BonusDraft) Set(index int, kind BonusKind, value int) error {
	if index < 0 || index >= len(d.raw)/bonusStride-1 || kind > DodgeBonus ||
		value < 0 ||
		value > 255 {
		return edit.ErrValue
	}
	d.raw[index*bonusStride+1+int(kind)] = byte(value)
	return nil
}

type BonusChange struct {
	Index         int
	Affinity      uint8
	Effective     bool
	Before, After [6]uint8
	Units         []Unit
}

type BonusPlan struct {
	commit                       *edit.Commit
	pointer, before, destination int
	changes                      []BonusChange
	usesErr                      error
}

func (p *BonusPlan) Current(
	e *Editor,
) bool {
	return p != nil && p.commit.Current(e.context)
}
func (p *BonusPlan) Pointer() int     { return p.pointer }
func (p *BonusPlan) Before() int      { return p.before }
func (p *BonusPlan) Destination() int { return p.destination }
func (p *BonusPlan) Size() int        { return p.commit.Size() }
func (p *BonusPlan) UsesError() error { return p.usesErr }
func (p *BonusPlan) Changes() []BonusChange {
	result := slices.Clone(p.changes)
	for index := range result {
		result[index].Units = slices.Clone(result[index].Units)
	}
	return result
}

func (e *Editor) PlanBonuses(d *BonusDraft) (result *BonusPlan, err error) {
	defer func() { err = fault.Wrap(err, "support.plan_bonuses", fault.Context{Resource: bonusResource}) }()
	if d == nil {
		return nil, rom.ErrStale
	}
	if err = d.binding.Check(e.context); err != nil {
		return nil, err
	}
	if !d.Modified() || e.source.ProtectPointer == nil {
		return nil, edit.ErrValue
	}
	image := e.context.Capture()
	if err = e.context.Validate(image.Data); err != nil {
		return nil, err
	}
	site := e.format.BonusPointer
	builder := romwrite.NewBuilder(
		image.Data,
		romwrite.Protection{Pointer: e.source.ProtectPointer},
	)
	destination, err := builder.Replace(d.raw, romwrite.PointerSite{
		Owner:   romwrite.Owner{Resource: bonusResource},
		Address: site,
		ExpectedWord: uint32( // #nosec G115 -- The bonus address was read from a valid GBA ROM pointer.
			0x08000000 + d.address,
		),
	})
	if err != nil {
		return nil, err
	}
	result = &BonusPlan{
		commit:  edit.NewCommit(d.binding, bonusResource, builder.Plan().Edit),
		pointer: site, before: d.address, destination: destination,
	}
	units, usesErr := e.source.Units(image.Data)
	result.usesErr = usesErr
	for _, row := range d.Rows() {
		before := [6]uint8(
			d.original[row.Index*bonusStride+1 : row.Index*bonusStride+7],
		)
		if before == row.Values {
			continue
		}
		change := BonusChange{
			Index:     row.Index,
			Affinity:  row.Affinity,
			Effective: row.Effective,
			Before:    before,
			After:     row.Values,
		}
		if row.Effective {
			for _, unit := range units {
				if unit.Affinity == row.Affinity {
					change.Units = append(change.Units, unit)
				}
			}
		}
		result.changes = append(result.changes, change)
	}
	return result, nil
}

func (e *Editor) ApplyBonuses(plan *BonusPlan) error {
	if plan == nil {
		return rom.ErrStale
	}
	return fault.Wrap(
		plan.commit.Apply(e.context),
		"support.apply_bonuses",
		fault.Context{Resource: bonusResource},
	)
}
