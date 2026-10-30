package deployment

import (
	"bytes"
	"image"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

var ErrDeployment = fault.Code{
	ID:   "game.invalid_deployment",
	Kind: fault.InvalidData,
}

const maxDeploymentUnits = 256

// DeploymentGroup identifies a pointer site, rather than a resource address.
// Two modes may share their current bytes while remaining separate edit targets.
type Group struct {
	Label                    string
	Pointer, Address, Stride int
	Units                    []Unit
	Err                      error
	records                  []deploymentRecord
	terminator               []byte
	faction                  int
}

// DeploymentUnit contains the values used by chapter navigation and map marks.
// End is the last encoded movement coordinate, not an event simulation result.
type Unit struct {
	Character, Class, Level, Faction int
	Start, End                       image.Point
	MovementCount                    int
}

func (f Format) group(
	data []byte,
	site int,
	label string,
	faction int,
	parsed map[int]Group,
) (group Group) {
	group = Group{
		Label:      label,
		Pointer:    site,
		Stride:     f.Stride,
		faction:    faction,
		terminator: make([]byte, 4),
	}
	ctx := fault.Context{
		Resource: label,
		Range:    &fault.Range{Offset: site, Length: 4, Size: len(data)},
	}
	fail := func() Group {
		group.Err = fault.New(
			ErrDeployment,
			"game.read_deployment_group",
			ctx,
			nil,
		)
		return group
	}
	if !gba.Within(data, site, 4) || site < 0xC0 || site%4 != 0 {
		return fail()
	}
	if gba.Uint32(data, site) == 0 {
		return group
	}
	at, valid := gba.Pointer(data, site)
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
		if !gba.Within(data, at, 1) {
			return fail()
		}
		if data[at] == 0 {
			group.terminator = bytes.Clone(data[at:min(at+4, len(data))])
			return group
		}
		if index == maxDeploymentUnits || !gba.Within(data, at, f.Stride) {
			return fail()
		}
		record := deploymentRecord{data: bytes.Clone(data[at : at+f.Stride])}
		if f.Stride == 20 && record.data[7] != 0 {
			count := int(record.data[7])
			path, valid := gba.Pointer(data, at+8)
			if !valid || path < 0xC0 || path%4 != 0 ||
				!gba.Within(data, path, count*8) {
				ctx.Resource = movementResource
				ctx.RecordID = new(index)
				return fail()
			}
			record.movement = bytes.Clone(data[path : path+count*8])
		}
		group.records = append(group.records, record)
		group.Units = append(group.Units, record.unit())
		at += f.Stride
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

// unitFormat describes the native unit record independently of event headers.
type Format struct{ Stride int }

// deploymentReader shares parsed aliases only within one immutable input.
type Reader struct {
	format Format
	data   []byte
	parsed map[int]Group
}

func NewReader(data []byte, format Format) *Reader {
	return &Reader{format: format, data: data, parsed: make(map[int]Group)}
}

func (r *Reader) Group(site int, label string, faction int) Group {
	return r.format.group(r.data, site, label, faction, r.parsed)
}

// Regions reports confirmed unit and movement extents without exposing their
// native record buffers to other resource scanners.
func (g Group) Regions() []space.Range {
	result := []space.Range{
		{
			Start: g.Address,
			End:   g.Address + len(g.records)*g.Stride + len(g.terminator),
		},
	}
	for _, record := range g.records {
		if len(record.data) == 20 && len(record.movement) > 0 {
			start := int(gba.Uint32(record.data, 8) & 0x01FFFFFF)
			result = append(
				result,
				space.Range{Start: start, End: start + len(record.movement)},
			)
		}
	}
	return result
}

// Movement returns the encoded path origin and detached steps for one unit.
func (g Group) Movement(unit int) (int, []MovementStep) {
	if g.Stride != 20 || unit < 0 || unit >= len(g.records) {
		return 0, nil
	}
	record := g.records[unit]
	return int(
			gba.Uint32(record.data, 8) & 0x01FFFFFF,
		), movementSteps(
			record.movement,
		)
}
