package chapters

import (
	"fmt"
	"image"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/icons"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/mapview"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type pathState struct {
	selected, lastSelected int
	list                   ui.ListState
	focus                  ui.Handle
	field                  string
	cells                  components.NumberCells
	// Stable row IDs keep cell drafts attached to segments through reordering.
	rows    []uint64
	nextRow uint64
}

func (entry *entryDraft) pathState() *pathState {
	if entry.paths == nil {
		entry.paths = make(map[int]*pathState)
	}
	path := entry.paths[entry.selected]
	if path == nil {
		path = &pathState{lastSelected: -1}
		entry.paths[entry.selected] = path
	}
	count := len(entry.data.Path(entry.selected))
	for len(path.rows) < count {
		path.rows = append(path.rows, path.newRow())
	}
	path.rows = path.rows[:count]
	path.selected = max(0, min(path.selected, count-1))
	return path
}

func (path *pathState) newRow() uint64 {
	path.nextRow++
	return path.nextRow
}

func pathInputKey(unit int, row uint64, field string) inputKey {
	return inputKey{unit, fmt.Sprintf("path/%d/%s", row, field)}
}

func (entry *entryDraft) pathChanged() {
	entry.preview = nil
	entry.pathState().lastSelected = -1
}

func (v *Page) pathOverlay(
	entry *entryDraft,
	overlay *mapview.Overlay,
	units []deployment.Unit,
) {
	if entry.pathMode != 1 || !entry.data.SupportsPaths() ||
		entry.selected >= len(units) {
		return
	}
	steps := entry.data.Path(entry.selected)
	path := entry.pathState()
	route := &mapview.Path{
		Start:    units[entry.selected].Start,
		Selected: path.selected,
	}
	for _, step := range steps {
		route.Steps = append(route.Steps, step.Position)
	}
	overlay.Path = route
	choose := overlay.Choose
	overlay.Choose = func(point image.Point) {
		for offset := 1; offset <= len(steps); offset++ {
			index := (path.selected + offset) % len(steps)
			if steps[index].Position == point {
				path.selected = index
				return
			}
		}
		if choose != nil {
			choose(point)
		}
	}
	if len(steps) > 0 &&
		(path.selected != path.lastSelected || entry.selected != entry.lastSelected) {
		path.lastSelected = path.selected
		path.list.ScrollIntoView(path.selected)
		overlay.Focus = new(steps[path.selected].Position)
	}
}

func (v *Page) unitSidebar(
	c *ui.Context,
	state *editor.State,
	entry *entryDraft,
	units []deployment.Unit,
	jump func(records.Kind, int),
) {
	if len(units) == 0 {
		return
	}
	if entry.data.SupportsPaths() {
		ui.Tabs(c.Key("deployment-sidebar"), &entry.pathMode, locales.DeployUnitDetails(), locales.MovePath()).
			Label(locales.Deployments() + " / " + locales.DeployUnitDetails()).
			OnChange(func() {
				entry.lastSelected = -1
				if entry.pathMode == 1 {
					entry.pathState().lastSelected = -1
				}
			})
	}
	ui.Column(c.Key(entry.selected)).Gap(10).Children(func() {
		if entry.pathMode == 1 && entry.data.SupportsPaths() {
			v.pathEditor(c, state, entry)
		} else {
			v.fields(c, state, entry, jump)
		}
	})
}

func (v *Page) pathEditor(
	c *ui.Context,
	state *editor.State,
	entry *entryDraft,
) {
	path := entry.pathState()
	steps := entry.data.Path(entry.selected)
	fields := entry.data.MovementFields()
	disabled := state.Busy || !entry.data.Current(state.Session.Deployments)
	if len(steps) > 0 {
		if path.field == "" {
			path.field = fields[0].ID
		}
		path.list.Selected = &path.selected
		path.list.Key = func(index int) any { return path.rows[index] }
		columns := []ui.TableColumn{
			{ID: "step", Title: "#", Width: 34, Fixed: true, Align: ui.End},
		}
		for _, field := range fields {
			title, width := locales.Label(field.ID), float32(100)
			switch field.ID {
			case deployment.MoveX:
				title, width = "X", 72
			case deployment.MoveY:
				title, width = "Y", 72
			case deployment.MoveConfig, deployment.MoveSyncUnit:
				width = 88
			case deployment.MoveSyncStep:
				width = 104
			}
			columns = append(
				columns,
				ui.TableColumn{
					ID:       field.ID,
					Title:    title,
					Width:    width,
					MinWidth: width,
					Align:    ui.End,
				},
			)
		}
		columns = append(
			columns,
			ui.TableColumn{ID: "place", Width: 36, Fixed: true},
		)
		begin := func() {
			if path.selected < 0 || path.selected >= len(path.rows) {
				return
			}
			for _, field := range fields {
				if field.ID == path.field {
					path.cells.Begin(
						pathInputKey(
							entry.selected,
							path.rows[path.selected],
							field.ID,
						),
					)
					return
				}
			}
		}
		ui.Table(c.Key("path-steps"), &path.list, columns, len(steps), func(node, column int) {
			if column == 0 {
				ui.Textf(c, "%02X", node).FontFeatures("tnum")
				return
			}
			if column > len(fields) {
				point, selected := v.mapView.Selection()
				components.ActionButton(c, icons.Lucide.Crosshair(), fmt.Sprintf("%s %02X", locales.MovePlace(), node)).
					Disabled(disabled || !selected).
					OnClick(func() { state.SetErr(entry.placePathStep(node, point)) })
				return
			}
			v.pathCell(c, state, entry, node, fields[column-1])
		}).
			Height(min(360, c.Theme().Space(9)+float32(len(steps))*38)).
			Bind(&path.focus).
			Label(locales.MoveSteps()).
			OnSubmit(begin).
			OnShortcut(0, ui.KeyF2, begin).
			OnShortcut(0, ui.KeyLeft, func() { path.moveField(fields, -1) }).
			OnShortcut(0, ui.KeyRight, func() { path.moveField(fields, 1) })
	} else {
		ui.Text(c, locales.MoveEmpty()).TextColor(c.Theme().TextMuted)
	}
	ui.Row(c).Wrap().Gap(6).Children(func() {
		for _, action := range []struct {
			icon    *ui.SVG
			label   string
			enabled bool
			run     func() error
		}{
			{icons.Lucide.Plus(), locales.MoveAdd(), len(steps) < deployment.MaxMovementSteps, entry.addPathStep},
			{icons.Lucide.Copy(), locales.MoveDuplicate(), len(steps) > 0 && len(steps) < deployment.MaxMovementSteps, entry.duplicatePathStep},
			{icons.Lucide.Trash(), locales.MoveRemove(), len(steps) > 0, entry.removePathStep},
			{icons.Lucide.ArrowUp(), locales.MoveUp(), path.selected > 0, func() error { return entry.movePathStep(path.selected - 1) }},
			{icons.Lucide.ArrowDown(), locales.MoveDown(), path.selected+1 < len(steps), func() error { return entry.movePathStep(path.selected + 1) }},
		} {
			components.ActionButton(c, action.icon, action.label).
				Disabled(disabled || !action.enabled).
				OnClick(func() { state.SetErr(action.run()) })
		}
	})
	if len(steps) > deployment.MaxMovementSteps {
		ui.Text(c, locales.MoveLimit()).
			FontSize(components.FontCaption).
			TextColor(c.Theme().Danger)
	}
}

func (v *Page) flushPathCells(c *ui.Context) {
	flush := func(entry *entryDraft) {
		for _, path := range entry.paths {
			path.cells.Flush(c)
		}
	}
	for _, entry := range v.entries {
		flush(entry)
	}
	for _, event := range v.events {
		for _, entry := range event.units {
			flush(entry)
		}
	}
}

func (entry *entryDraft) pendingPathInput() bool {
	for _, path := range entry.paths {
		if path.cells.Pending() {
			return true
		}
	}
	return false
}

func (path *pathState) moveField(fields []deployment.Field, direction int) {
	index := slices.IndexFunc(
		fields,
		func(field deployment.Field) bool { return field.ID == path.field },
	)
	path.field = fields[(index+direction+len(fields))%len(fields)].ID
}

func (v *Page) pathCell(
	c *ui.Context,
	state *editor.State,
	entry *entryDraft,
	node int,
	field deployment.Field,
) {
	path := entry.pathState()
	unit, row := entry.selected, path.rows[node]
	value, _ := entry.data.PathValue(unit, node, field.ID)
	help := locales.MoveScope()
	switch field.ID {
	case deployment.MoveSyncUnit, deployment.MoveSyncStep:
		help = locales.MoveSyncHelp()
	case deployment.MoveDelay:
		help = locales.MoveDelayHelp()
	case deployment.MoveConfig:
		help = locales.MoveConfigHelp()
	}
	canEdit := func() bool { return !state.Busy && entry.data.Current(state.Session.Deployments) }
	if canEdit() {
		help = locales.MoveEditCellHelp() + "\n" + help
	}
	components.TextDraft(state, path.cells.Cell(c, components.NumberCell{
		Key:   pathInputKey(unit, row, field.ID),
		Value: value,
		Min:   field.Min,
		Max:   field.Max,
		Label: fmt.Sprintf(
			"%s %02X / %s",
			locales.MovePath(),
			node,
			locales.Label(field.ID),
		),
		Help:     help,
		Focus:    &path.focus,
		Selected: path.selected == node && path.field == field.ID,
		CanEdit:  canEdit,
		Select:   func() { path.selected, path.field = node, field.ID },
		Changed:  func() { entry.preview = nil },
		Set: func(value int64) error {
			index := slices.Index(path.rows, row)
			err := entry.data.SetPathField(unit, index, field.ID, value)
			state.SetErr(err)
			if err == nil {
				entry.preview = nil
				path.lastSelected = -1
			}
			return err
		},
	}))
}

func (entry *entryDraft) addPathStep() error {
	path := entry.pathState()
	if err := entry.data.AddPathStep(entry.selected); err != nil {
		return err
	}
	path.rows = append(path.rows, path.newRow())
	path.selected = len(path.rows) - 1
	entry.pathChanged()
	return nil
}

func (entry *entryDraft) duplicatePathStep() error {
	path := entry.pathState()
	if err := entry.data.DuplicatePathStep(
		entry.selected,
		path.selected,
	); err != nil {
		return err
	}
	source := path.rows[path.selected]
	row := path.newRow()
	for _, field := range entry.data.MovementFields() {
		path.cells.Copy(
			pathInputKey(entry.selected, source, field.ID),
			pathInputKey(entry.selected, row, field.ID),
		)
	}
	path.selected++
	path.rows = slices.Insert(path.rows, path.selected, row)
	entry.pathChanged()
	return nil
}

func (entry *entryDraft) removePathStep() error {
	path := entry.pathState()
	if err := entry.data.RemovePathStep(
		entry.selected,
		path.selected,
	); err != nil {
		return err
	}
	row := path.rows[path.selected]
	for _, field := range entry.data.MovementFields() {
		path.cells.Forget(pathInputKey(entry.selected, row, field.ID))
	}
	path.rows = slices.Delete(path.rows, path.selected, path.selected+1)
	path.selected = min(path.selected, max(0, len(path.rows)-1))
	entry.pathChanged()
	return nil
}

func (entry *entryDraft) movePathStep(to int) error {
	path := entry.pathState()
	if err := entry.data.MovePathStep(
		entry.selected,
		path.selected,
		to,
	); err != nil {
		return err
	}
	row := path.rows[path.selected]
	path.rows = slices.Delete(path.rows, path.selected, path.selected+1)
	path.rows = slices.Insert(path.rows, to, row)
	path.selected = to
	entry.pathChanged()
	return nil
}

func (entry *entryDraft) placePathStep(node int, point image.Point) error {
	for _, field := range []struct {
		id    string
		value int
	}{{deployment.MoveX, point.X}, {deployment.MoveY, point.Y}} {
		if err := entry.data.SetPathField(
			entry.selected,
			node,
			field.id,
			int64(field.value),
		); err != nil {
			return err
		}
		entry.pathState().cells.Forget(
			pathInputKey(
				entry.selected,
				entry.pathState().rows[node],
				field.id,
			),
		)
	}
	entry.pathState().selected = node
	entry.pathChanged()
	return nil
}
