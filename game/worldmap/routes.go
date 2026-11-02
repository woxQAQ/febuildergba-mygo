package worldmap

import (
	"encoding/binary"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

const (
	RouteResource = "worldmap.routes"
	RouteCount    = 32
	RouteSize     = 12
	// The native mover reserves sixteen coordinate words, including both nodes.
	MaxMovementPoints = 6
	MovementEnd       = 4096
)

var (
	ErrRoutes = fault.Code{
		ID:   "game.invalid_worldmap_routes",
		Kind: fault.InvalidData,
	}
	ErrMovement = fault.Code{
		ID:   "game.invalid_worldmap_movement",
		Kind: fault.InvalidData,
	}
	ErrConnections = fault.Code{
		ID:   "game.worldmap_connections",
		Kind: fault.InvalidInput,
	}
)

// MovementPoint is an intermediate pixel position at a fraction of total travel
// time. Progress 0 and 4096 belong to the route's nodes, not stored points.
type MovementPoint struct{ Progress, X, Y int }

type Route struct {
	ID, Address    int
	Nodes          [2]int
	Road, Movement int
	MovementErr    error
	points         []MovementPoint
	terminator     [8]byte
}

func (r Route) Owner() space.Owner      { return space.Owner{Resource: RouteResource, ID: r.ID} }
func (r Route) Points() []MovementPoint { return slices.Clone(r.points) }

func (f Format) Routes(data []byte) ([]Route, error) {
	if f.RoutePointer == 0 {
		return nil, nil
	}
	at, valid := gba.Pointer(data, f.RoutePointer)
	if !valid || at < 0xC0 || at%4 != 0 ||
		!gba.Within(data, at, RouteCount*RouteSize) {
		return nil, fault.Wrap(
			ErrRoutes,
			"worldmap.read_routes",
			fault.Context{
				Range: &fault.Range{
					Offset: f.RoutePointer,
					Length: 4,
					Size:   len(data),
				},
			},
		)
	}
	routes := make([]Route, RouteCount)
	for i := range routes {
		address := at + i*RouteSize
		r := Route{
			ID:      i,
			Address: address,
			Nodes: [2]int{
				int(int8(data[address+4])),
				int(int8(data[address+5])),
			},
		}
		r.Road, _ = gba.Pointer(data, address)
		if gba.Uint32(data, address+8) != 0 {
			var valid bool
			r.Movement, valid = gba.Pointer(data, address+8)
			if !valid || r.Movement < 0xC0 || r.Movement%4 != 0 {
				r.MovementErr = ErrMovement
			} else {
				r.points, r.terminator, r.MovementErr = readMovement(
					data,
					r.Movement,
				)
			}
		}
		routes[i] = r
	}
	return routes, nil
}

func readMovement(data []byte, at int) ([]MovementPoint, [8]byte, error) {
	var points []MovementPoint
	for i := 0; i <= MaxMovementPoints; i++ {
		pos := at + i*8
		if !gba.Within(data, pos, 8) {
			break
		}
		progress := int(int32(gba.Uint32(data, pos)))
		if progress < 0 {
			return points, [8]byte(data[pos : pos+8]), nil
		}
		points = append(
			points,
			MovementPoint{
				progress,
				int(int16(gba.Uint16(data, pos+4))),
				int(int16(gba.Uint16(data, pos+6))),
			},
		)
	}
	return nil, [8]byte{}, ErrMovement
}

func (f Format) routeClaims(data []byte) []space.Claim {
	if f.RoutePointer < 0xC0 || !gba.Within(data, f.RoutePointer, 4) {
		return nil
	}
	claims := []space.Claim{
		{
			Start: f.RoutePointer,
			End:   f.RoutePointer + 4,
			Owner: space.Owner{Resource: RouteResource, ID: -1},
			Site:  true,
		},
	}
	routes, _ := f.Routes(data)
	for _, r := range routes {
		claims = append(
			claims,
			space.Claim{
				Start: r.Address,
				End:   r.Address + RouteSize,
				Owner: r.Owner(),
			},
		)
		for _, payload := range []struct {
			site, at, length int
			resource         string
		}{
			{r.Address, r.Road, 0, "worldmap.roads"},
			{r.Address + 8, r.Movement, 0, "worldmap.movement"},
		} {
			owner := space.Owner{Resource: payload.resource, ID: r.ID}
			claims = append(
				claims,
				space.Claim{
					Start: payload.site,
					End:   payload.site + 4,
					Owner: owner,
					Host:  r.Owner(),
					Site:  true,
				},
			)
			if payload.site == r.Address {
				_, payload.length, _ = readRoad(data, payload.at)
			} else if r.MovementErr == nil && r.Movement != 0 {
				payload.length = (len(r.points) + 1) * 8
			}
			// Even a malformed resource has a confirmed first header. It must
			// not become writable merely because the rest cannot be decoded.
			aligned := payload.at%4 == 0 ||
				payload.site == r.Address && payload.at%2 == 0
			if payload.length == 0 && payload.at >= 0xC0 && aligned &&
				gba.Within(data, payload.at, 4) {
				payload.length = 4
			}
			if payload.at >= 0xC0 && payload.length > 0 &&
				gba.Within(data, payload.at, payload.length) {
				claims = append(
					claims,
					space.Claim{
						Start: payload.at,
						End:   payload.at + payload.length,
						Owner: owner,
					},
				)
			}
		}
	}
	return claims
}

func (e *Editor) RoutesAvailable() bool { return e.format.RoutePointer != 0 }
func (e *Editor) Routes() ([]Route, error) {
	if !e.routeStamp.Current(e.context) {
		image := e.context.Document().Snapshot()
		e.routeStamp = e.context.Bind(image.Revision)
		e.routes, e.routeErr = nil, e.context.Validate(image.Data)
		if e.routeErr == nil {
			e.routes, e.routeErr = e.format.Routes(image.Data)
		}
	}
	return slices.Clone(e.routes), e.routeErr
}

// RouteDraft owns the endpoints, movement and road layout of one native route.
type RouteDraft struct {
	stamp    edit.Stamp
	original Route
	nodes    [2]int
	points   []MovementPoint
	road     *RoadDraft
	roadErr  error
}

func (e *Editor) NewRouteDraft(id int) (*RouteDraft, error) {
	routes, err := e.Routes()
	if err != nil {
		return nil, err
	}
	if id < 0 || id >= len(routes) {
		return nil, edit.ErrValue
	}
	r := routes[id]
	road, roadErr := newRoadDraft(
		e.context.Document().Snapshot().Data,
		e.format.Scene,
		r.Road,
	)
	return &RouteDraft{
		stamp:    e.routeStamp,
		original: r,
		nodes:    r.Nodes,
		points:   r.Points(),
		road:     road, roadErr: roadErr,
	}, nil
}

func (d *RouteDraft) Current(
	e *Editor,
) bool {
	return d != nil && d.stamp.Current(e.context)
}

func (d *RouteDraft) Road() (*RoadDraft, error) { return d.road, d.roadErr }

func (d *RouteDraft) Modified() bool {
	return d.nodes != d.original.Nodes ||
		!slices.Equal(
			d.points,
			d.original.points,
		) || d.road != nil && d.road.Modified()
}

func (d *RouteDraft) Route() Route {
	r := d.original
	r.Nodes = d.nodes
	r.points = slices.Clone(d.points)
	return r
}

func (d *RouteDraft) SetNode(endpoint, id int) error {
	if endpoint < 0 || endpoint > 1 || id < 0 || id >= NodeCount {
		return edit.ErrValue
	}
	d.nodes[endpoint] = id
	return nil
}

// SetPoints retains structurally valid input; order and progress are validated
// at planning so a point can be reordered before its time is corrected.
func (d *RouteDraft) SetPoints(points []MovementPoint) error {
	if d.original.MovementErr != nil {
		return d.original.MovementErr
	}
	if len(points) > MaxMovementPoints {
		return edit.ErrValue
	}
	for _, p := range points {
		if p.Progress < 1 || p.Progress >= MovementEnd || p.X < -32768 ||
			p.X > 32767 ||
			p.Y < -32768 ||
			p.Y > 32767 {
			return edit.ErrValue
		}
	}
	d.points = slices.Clone(points)
	return nil
}

type RoutePlan struct {
	commit        *edit.Plan
	before, after Route
	appended      space.Range
	shared        []int
	roadShared    []int
	roadChanges   []RoadChange
}

func (p *RoutePlan) Current(
	e *Editor,
) bool {
	return p != nil && p.commit.Current(e.context)
}
func (p *RoutePlan) Before() Route         { return p.before }
func (p *RoutePlan) After() Route          { return p.after }
func (p *RoutePlan) Appended() space.Range { return p.appended }

// Shared lists known route IDs still using the old movement pointer.
func (p *RoutePlan) Shared() []int { return slices.Clone(p.shared) }

func (p *RoutePlan) RoadShared() []int { return slices.Clone(p.roadShared) }

func (p *RoutePlan) RoadChanges() []RoadChange { return slices.Clone(p.roadChanges) }

func (e *Editor) PlanRoute(d *RouteDraft) (plan *RoutePlan, err error) {
	defer func() { err = fault.Wrap(err, "worldmap.plan_route", fault.Context{Resource: RouteResource}) }()
	if d == nil {
		return nil, rom.ErrStale
	}
	if err = d.stamp.Check(e.context); err != nil {
		return nil, err
	}
	if !d.Modified() || e.source.ProtectRange == nil {
		return nil, edit.ErrValue
	}
	image := e.context.Document().Snapshot()
	if err = e.context.Validate(image.Data); err != nil {
		return nil, err
	}
	r := d.original
	if err = e.source.ProtectRange(
		image.Data,
		r.Owner(),
		space.Range{Start: r.Address, End: r.Address + RouteSize},
	); err != nil {
		return nil, fault.New(
			ErrReference,
			"worldmap.protect_route",
			fault.Context{Resource: RouteResource, RecordID: new(r.ID)},
			err,
		)
	}
	routes, err := e.format.Routes(image.Data)
	if err != nil {
		return nil, err
	}
	if d.nodes != r.Nodes {
		if _, err = e.format.Nodes(image.Data); err != nil {
			return nil, err
		}
		if err = validateConnections(routes, r.ID, d.nodes); err != nil {
			return nil, err
		}
	}
	plan = &RoutePlan{before: r, after: d.Route()}
	appender := space.NewAppender(len(image.Data))
	var direct []rom.Change
	if d.road != nil && d.road.Modified() {
		plan.roadChanges = d.road.changes()
		for _, other := range routes {
			if other.ID != r.ID && other.Road == r.Road {
				plan.roadShared = append(plan.roadShared, other.ID)
			}
		}
		plan.after.Road, err = appender.Add(d.road.encode(), r.Address)
		if err != nil {
			return nil, err
		}
	}
	if !slices.Equal(d.points, r.points) {
		previous := 0
		for _, point := range d.points {
			if point.Progress <= previous {
				return nil, ErrMovement
			}
			previous = point.Progress
		}
		for _, other := range routes {
			if r.Movement != 0 && other.ID != r.ID &&
				other.Movement == r.Movement {
				plan.shared = append(plan.shared, other.ID)
			}
		}
		if len(d.points) == 0 {
			direct = append(
				direct,
				rom.Change{Offset: r.Address + 8, Data: make([]byte, 4)},
			)
			plan.after.Movement = 0
		} else {
			var payload []byte
			for _, p := range d.points {
				payload = binary.LittleEndian.AppendUint32(
					payload,
					uint32(p.Progress),
				)
				payload = binary.LittleEndian.AppendUint16(payload, uint16(p.X))
				payload = binary.LittleEndian.AppendUint16(payload, uint16(p.Y))
			}
			end := r.terminator
			if r.Movement == 0 {
				end = [8]byte{0xFF, 0xFF, 0xFF, 0xFF, 0, 0, 0, 0}
			}
			payload = append(payload, end[:]...)
			plan.after.Movement, err = appender.Add(payload, r.Address+8)
			if err != nil {
				return nil, err
			}
		}
	}
	if d.nodes != r.Nodes {
		direct = append(
			direct,
			rom.Change{
				Offset: r.Address + 4,
				Data:   []byte{byte(d.nodes[0]), byte(d.nodes[1])},
			},
		)
	}
	allocation := appender.Plan()
	allocation.Edit.Changes = append(allocation.Edit.Changes, direct...)
	plan.appended = allocation.Destination
	plan.commit = edit.NewPlan(d.stamp, RouteResource, allocation.Edit)
	return plan, nil
}

func validateConnections(routes []Route, id int, nodes [2]int) error {
	if nodes[0] < 0 || nodes[0] >= NodeCount || nodes[1] < 0 ||
		nodes[1] >= NodeCount ||
		nodes[0] == nodes[1] {
		return ErrConnections
	}
	var counts [NodeCount]int
	for _, r := range routes {
		if r.ID == id {
			r.Nodes = nodes
		} else if r.Nodes == nodes || r.Nodes == [2]int{nodes[1], nodes[0]} {
			return ErrConnections
		}
		for _, n := range r.Nodes {
			if n >= 0 && n < NodeCount {
				counts[n]++
			}
		}
	}
	for _, n := range nodes {
		// Node zero's native consumer writes one additional connection slot.
		limit := 7
		if n == 0 {
			limit = 6
		}
		if counts[n] > limit {
			return ErrConnections
		}
	}
	return nil
}

func (e *Editor) ApplyRoute(plan *RoutePlan) error {
	if plan == nil {
		return rom.ErrStale
	}
	return fault.Wrap(
		plan.commit.Apply(e.context),
		"worldmap.apply_route",
		fault.Context{Resource: RouteResource},
	)
}
