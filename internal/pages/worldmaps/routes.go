package worldmaps

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type (
	pointInput struct{ values [3]valueInput }
	routeState struct {
		draft      *worldmap.RouteDraft
		plan       *worldmap.RoutePlan
		err        error
		nodes      [2]valueInput
		points     []*pointInput
		selected   int
		placing    bool
		road       roadPainter
		eventsOpen bool
	}
)

func (s *routeState) invalid() bool {
	for _, v := range s.nodes {
		if v.invalid {
			return true
		}
	}
	for _, p := range s.points {
		for _, v := range p.values {
			if v.invalid {
				return true
			}
		}
	}
	return false
}

func newPointInput(point worldmap.MovementPoint) *pointInput {
	return &pointInput{
		values: [3]valueInput{
			{text: strconv.Itoa(point.Progress)},
			{text: strconv.Itoa(point.X)},
			{text: strconv.Itoa(point.Y)},
		},
	}
}

func (s *routeState) syncPoints() {
	s.plan, s.err = nil, nil
	points := make([]worldmap.MovementPoint, len(s.points))
	valid := true
	for i, p := range s.points {
		var values [3]int
		for j := range p.values {
			value, err := input.ParseInteger(p.values[j].text)
			low, high := int64(-32768), int64(32767)
			if j == 0 {
				low, high = 1, worldmap.MovementEnd-1
			}
			p.values[j].invalid = err != nil || value < low || value > high
			valid = valid && !p.values[j].invalid
			values[j] = int(value)
		}
		points[i] = worldmap.MovementPoint{
			Progress: values[0],
			X:        values[1],
			Y:        values[2],
		}
	}
	if valid {
		s.err = s.draft.SetPoints(points)
	}
}

type routePage struct {
	query                string
	selected, nodeFilter int
	rows                 []worldmap.Route
	list                 ui.ListState
	drafts               map[int]*routeState
	canvas               routeCanvas
	active               *routeState
}

func (p *Page) OpenRoute(id int) {
	p.tab = 1
	routes, err := p.state.Session.WorldMap.Routes()
	if err != nil || id < 0 || id >= len(routes) {
		return
	}
	p.routes.query = ""
	p.routes.nodeFilter = -1
	p.routes.rows = routes
	p.routes.selected = id
	p.routes.list.ScrollTo(id, ui.Center)
}

func (p *Page) routeCaption(route worldmap.Route) string {
	return fmt.Sprintf(
		"%02X · %s ↔ %s",
		route.ID,
		p.nodeCaption(route.Nodes[0]),
		p.nodeCaption(route.Nodes[1]),
	)
}

func (p *Page) nodeCaption(id int) string {
	nodes, _ := p.state.Session.WorldMap.Nodes()
	if id >= 0 && id < len(nodes) {
		return p.caption(nodes[id])
	}
	return fmt.Sprintf("%02X", id)
}

func (p *Page) connectedRoutes(c *ui.Context, id int) {
	ui.Button(c.Key("connected-routes"), locales.WorldmapConnectedRoutes()).
		Disabled(p.state.Busy || !p.state.Session.WorldMap.RoutesAvailable()).
		OnClick(func() {
			p.tab = 1
			p.routes.nodeFilter = id
			p.routes.query = ""
			p.routes.selected = -1
			p.routes.list = ui.ListState{}
		})
}

func (p *Page) routeDraft(id int) *routeState {
	s := p.routes.drafts[id]
	if s != nil && s.draft != nil &&
		!s.draft.Current(p.state.Session.WorldMap) &&
		!s.draft.Modified() &&
		!s.invalid() {
		s = nil
	}
	if s == nil {
		s = &routeState{selected: -1}
		s.draft, s.err = p.state.Session.WorldMap.NewRouteDraft(id)
		if s.draft != nil {
			r := s.draft.Route()
			for i, n := range r.Nodes {
				s.nodes[i].text = strconv.Itoa(n)
			}
			for _, point := range r.Points() {
				s.points = append(s.points, newPointInput(point))
			}
		}
		p.routes.drafts[id] = s
	}
	return s
}

func (p *Page) refreshRoutes() error {
	r := &p.routes
	selected := -1
	if r.selected >= 0 && r.selected < len(r.rows) {
		selected = r.rows[r.selected].ID
	}
	routes, err := p.state.Session.WorldMap.Routes()
	r.rows = nil
	query := strings.ToLower(strings.TrimSpace(r.query))
	id, idErr := strconv.ParseUint(strings.TrimPrefix(query, "0x"), 16, 8)
	for _, route := range routes {
		if r.nodeFilter >= 0 && !slices.Contains(route.Nodes[:], r.nodeFilter) {
			continue
		}
		if query == "" ||
			strings.Contains(strings.ToLower(p.routeCaption(route)), query) ||
			idErr == nil && int(id) == route.ID {
			r.rows = append(r.rows, route)
		}
	}
	r.selected = -1
	for i, route := range r.rows {
		if route.ID == selected {
			r.selected = i
		}
	}
	if r.selected < 0 && len(r.rows) > 0 {
		r.selected = 0
	}
	return err
}

func (p *Page) viewRoutes(c *ui.Context) {
	err := p.refreshRoutes()
	r := &p.routes
	p.sidebar.Render(c, 260, func() {
		ui.Column(c).Fill().Padding(12).Gap(8).Children(func() {
			components.TextDraft(
				p.state,
				ui.TextInput(c.Key("search-routes"), &r.query).
					Label(locales.WorldmapSearch()).
					Placeholder(locales.WorldmapSearch()).
					Disabled(p.state.Busy),
			)
			if r.nodeFilter >= 0 {
				ui.Text(c, p.nodeCaption(r.nodeFilter)).FontSize(13)
				ui.Button(c, locales.WorldmapAllRoutes()).
					Disabled(p.state.Busy).
					OnClick(func() { r.nodeFilter = -1; r.list = ui.ListState{} })
			}
			r.list.Selected = &r.selected
			r.list.Key = func(index int) any { return r.rows[index].ID }
			ui.Table(c.Key("routes"), &r.list, []ui.TableColumn{{ID: "route", Title: locales.WorldmapRoutes()}}, len(r.rows), func(row, _ int) {
				route := r.rows[row]
				caption := p.routeCaption(route)
				if entry := r.drafts[route.ID]; entry != nil &&
					entry.draft != nil &&
					(entry.draft.Modified() || entry.invalid()) {
					caption += " *"
				}
				ui.Text(c, caption).SingleLine().FontSize(13).Tooltip(caption)
			}).
				Grow(1).
				Label(locales.WorldmapRoutes()).
				Disabled(p.state.Busy)
			if err != nil {
				ui.Text(c, components.ErrorText(err)).
					TextColor(c.Theme().Danger)
			}
		})
	}, func() { p.routeDetail(c) })
}

func (p *Page) routeDetail(c *ui.Context) {
	r := &p.routes
	if r.selected < 0 || r.selected >= len(r.rows) {
		p.activateRoute(nil)
		return
	}
	current := r.rows[r.selected]
	entry := p.routeDraft(current.ID)
	p.activateRoute(entry)
	reload := func() { delete(r.drafts, current.ID) }
	ui.Scroll(c.Key(entry)).
		Fill().
		MinWidth(0).
		Padding(16).
		Gap(12).
		Children(func() {
			ui.Text(c, p.routeCaption(current)).FontSize(20).Bold()
			if entry.draft == nil {
				ui.Text(c, components.ErrorText(entry.err)).
					TextColor(c.Theme().Danger)
				ui.Button(c, locales.DeployReload()).
					Disabled(p.state.Busy).
					OnClick(reload)
				return
			}
			e := p.state.Session.WorldMap
			stale := !entry.draft.Current(e)
			disabled := p.state.Busy || stale
			if stale {
				entry.plan = nil
				ui.Text(c, locales.WorldmapStale()).TextColor(c.Theme().Danger)
			}
			ui.Text(c, locales.WorldmapRouteScope()).
				FontSize(12).
				TextColor(c.Theme().TextMuted)
			ui.Row(c.Key("route-actions")).Wrap().Gap(8).Children(func() {
				ui.Button(c.Key("preview-route"), locales.WorldmapPreviewRoute()).
					Disabled(disabled || entry.invalid() || !entry.draft.Modified()).
					OnClick(func() { entry.plan, entry.err = e.PlanRoute(entry.draft) })
				ui.Button(c, locales.DeployReload()).
					Disabled(p.state.Busy).
					OnClick(reload)
				ui.Checkbox(c, &p.advanced, locales.Advanced())
			})
			if entry.err != nil {
				ui.Text(c, components.ErrorText(entry.err)).
					TextColor(c.Theme().Danger)
			}
			if entry.invalid() {
				ui.Text(c, locales.SupportInvalidInput()).
					TextColor(c.Theme().Danger)
			}
			if entry.plan != nil {
				p.routePreview(c, entry, reload)
			}
			p.routeEndpoints(c, entry, disabled)
			p.relatedEvents(
				c,
				event.ScriptWorldMapRouteTarget,
				current.ID,
				&entry.eventsOpen,
			)
			p.backgroundControls(c)
			ui.Column(c.Key("road-workspace")).
				Bind(&entry.road.history).
				Gap(8).
				Children(func() {
					p.roadHistory(c, entry, disabled)
					if !stale {
						p.routeMap(c, entry, disabled)
					}
				})
			p.movementPoints(c, entry, disabled)
			if p.advanced {
				ui.Row(c.Key("route-address")).Wrap().Gap(8).Children(func() {
					ui.Textf(c, "0x%08X", current.Address).
						Font("monospace").
						FontSize(12)
					ui.Button(c, locales.EditBytes()).
						Disabled(disabled || p.openHex == nil).
						OnClick(func() { p.openHex(current.Address, worldmap.RouteSize) })
				})
			}
		})
}

func (p *Page) routeEndpoints(c *ui.Context, s *routeState, disabled bool) {
	ui.Row(c.Key("endpoints")).Wrap().Gap(12).Children(func() {
		for i, label := range []string{locales.WorldmapRouteStart(), locales.WorldmapRouteEnd()} {
			ui.Column(c.Key(i)).
				Width(230).
				MaxWidthPercent(100).
				Gap(4).
				Children(func() {
					ui.Text(c, label).FontSize(13)
					options := p.options(worldmap.EirikaNext)[1:]
					control := ui.Autocomplete(c.Key("node"), &s.nodes[i].text, options).
						Label(label).
						Disabled(disabled).
						OnChange(func() {
							s.plan, s.err = nil, nil
							text, _, _ := strings.Cut(s.nodes[i].text, " · ")
							id, err := input.ParseInteger(text)
							s.nodes[i].invalid = err != nil || id < 0 ||
								id >= worldmap.NodeCount
							if !s.nodes[i].invalid {
								s.err = s.draft.SetNode(i, int(id))
							}
						})
					components.TextDraft(p.state, control)
					if s.nodes[i].invalid {
						control.Error(fmt.Sprintf("0…%d", worldmap.NodeCount-1))
					}
					id := s.draft.Route().Nodes[i]
					ui.Button(c, locales.WorldmapOpenNode()).
						Disabled(p.state.Busy || s.nodes[i].invalid || id < 0 || id >= worldmap.NodeCount).
						OnClick(func() { p.Open(id) })
				})
		}
	})
}

func (p *Page) movementPoints(c *ui.Context, s *routeState, disabled bool) {
	ui.Column(c.Key("movement")).Gap(8).Children(func() {
		ui.Text(c, locales.WorldmapMovement()).Bold()
		if err := s.draft.Route().MovementErr; err != nil {
			ui.Text(c, components.ErrorText(err)).TextColor(c.Theme().Danger)
			return
		}
		ui.Text(c, locales.WorldmapProgressHelp()).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
		ui.Row(c.Key("point-tools")).Wrap().Gap(8).Children(func() {
			ui.Button(c, locales.WorldmapAddPoint()).
				Disabled(disabled || s.invalid() || len(s.points) >= worldmap.MaxMovementPoints).
				OnClick(func() {
					points := s.draft.Route().Points()
					index := len(points)
					if s.selected >= 0 && s.selected < len(points) {
						index = s.selected + 1
					}
					before, after := worldmap.MovementPoint{}, worldmap.MovementPoint{
						Progress: worldmap.MovementEnd,
					}
					nodes, _ := p.state.Session.WorldMap.Nodes()
					route := s.draft.Route()
					if id := route.Nodes[0]; id >= 0 && id < len(nodes) {
						before.X, before.Y = nodes[id].Value(
							worldmap.X,
						), nodes[id].Value(
							worldmap.Y,
						)
					}
					if id := route.Nodes[1]; id >= 0 && id < len(nodes) {
						after.X, after.Y = nodes[id].Value(
							worldmap.X,
						), nodes[id].Value(
							worldmap.Y,
						)
					}
					if index > 0 {
						before = points[index-1]
					}
					if index < len(points) {
						after = points[index]
					}
					if after.Progress-before.Progress < 2 {
						s.err = worldmap.ErrMovement
						return
					}
					point := worldmap.MovementPoint{
						Progress: (before.Progress + after.Progress) / 2,
						X:        (before.X + after.X) / 2,
						Y:        (before.Y + after.Y) / 2,
					}
					s.points = slices.Insert(
						s.points,
						index,
						newPointInput(point),
					)
					s.selected = index
					s.syncPoints()
				})
			ui.Checkbox(c, &s.placing, locales.WorldmapPlacePoint()).
				Disabled(disabled || s.selected < 0 || s.invalid()).
				OnChange(func() {
					if s.placing {
						s.road.tool = int(roadInspect)
						s.clearRoadGesture()
					}
				})
		})
		if len(s.points) == 0 {
			ui.Text(c, locales.WorldmapNoPoints()).
				TextColor(c.Theme().TextMuted)
		}
		for index, point := range s.points {
			ui.Row(c.Key(point)).
				Wrap().
				Gap(6).
				AlignItems(ui.Center).
				Children(func() {
					ui.Button(c.Key("select-point"), fmt.Sprintf("%d", index+1)).
						Label(fmt.Sprintf("%s %d", locales.WorldmapPoint(), index+1)).
						Selected(s.selected == index).
						Disabled(p.state.Busy).
						OnClick(func() { s.selected = index })
					for field, label := range []string{locales.WorldmapProgress(), locales.WorldmapX(), locales.WorldmapY()} {
						ui.Column(c.Key(field)).
							Width(110).
							Gap(3).
							Children(func() {
								ui.Text(c, label).FontSize(12)
								control := ui.TextInput(c.Key("value"), &point.values[field].text).
									Label(fmt.Sprintf("%s %d", label, index+1)).
									TextAlign(ui.End).
									Disabled(disabled).
									OnChange(s.syncPoints)
								components.TextDraft(p.state, control)
								if point.values[field].invalid {
									control.Error(locales.SupportInvalidInput())
								}
							})
					}
					ui.Button(c.Key("up"), "↑").
						Label(fmt.Sprintf("%s %d", locales.WorldmapMoveUp(), index+1)).
						Disabled(disabled || index == 0).
						OnClick(func() {
							s.points[index-1], s.points[index] = s.points[index], s.points[index-1]
							s.selected = index - 1
							s.syncPoints()
						})
					ui.Button(c.Key("down"), "↓").
						Label(fmt.Sprintf("%s %d", locales.WorldmapMoveDown(), index+1)).
						Disabled(disabled || index+1 == len(s.points)).
						OnClick(func() {
							s.points[index+1], s.points[index] = s.points[index], s.points[index+1]
							s.selected = index + 1
							s.syncPoints()
						})
					ui.Button(c.Key("remove"), locales.WorldmapRemovePoint()).
						Label(fmt.Sprintf("%s %d", locales.WorldmapRemovePoint(), index+1)).
						Disabled(disabled).
						OnClick(func() {
							s.points = slices.Delete(s.points, index, index+1)
							s.selected = min(index, len(s.points)-1)
							s.syncPoints()
						})
				})
		}
	})
}

func (p *Page) routePreview(c *ui.Context, s *routeState, reload func()) {
	plan := s.plan
	before, after := plan.Before(), plan.After()
	ui.Column(c.Key("route-preview")).
		Gap(6).
		Padding(8, 0).
		BorderWidth(1, 0).
		BorderColor(c.Theme().Border).
		Children(func() {
			ui.Text(c, locales.WorldmapApplyRouteHelp()).FontSize(12)
			ui.Textf(c, "%s: %02X ↔ %02X → %02X ↔ %02X", locales.WorldmapRoutes(), before.Nodes[0], before.Nodes[1], after.Nodes[0], after.Nodes[1]).
				FontSize(13)
			for i, point := range before.Points() {
				ui.Textf(c, "− %d: %d · (%d, %d)", i+1, point.Progress, point.X, point.Y).
					FontSize(12)
			}
			for i, point := range after.Points() {
				ui.Textf(c, "+ %d: %d · (%d, %d)", i+1, point.Progress, point.X, point.Y).
					FontSize(12)
			}
			if appended := plan.Appended(); appended.End > appended.Start {
				ui.Textf(c, "%s: 0x%08X · %d B", locales.WorldmapNewRouteData(), appended.Start, appended.End-appended.Start).
					FontSize(12)
			}
			if shared := plan.Shared(); len(shared) > 0 {
				ui.Textf(c, "%s: %X", locales.WorldmapSharedMovement(), shared).
					FontSize(12)
			}
			if changes := plan.RoadChanges(); len(changes) > 0 {
				ui.Textf(c, "%s: %d", locales.WorldmapRoadChangedCells(), len(changes)).
					FontSize(13)
				if shared := plan.RoadShared(); len(shared) > 0 {
					ui.Textf(c, "%s: %X", locales.WorldmapSharedRoad(), shared).
						FontSize(12)
				}
			}
			ui.PrimaryButton(c.Key("apply-route"), locales.WorldmapApplyRoute()).
				Disabled(p.state.Busy || s.invalid() || !plan.Current(p.state.Session.WorldMap)).
				OnClick(func() {
					s.err = p.state.Session.WorldMap.ApplyRoute(plan)
					s.plan = nil
					if s.err == nil {
						p.state.HistoryEpoch++
						reload()
					}
				})
		})
}
