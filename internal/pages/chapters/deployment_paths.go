package chapters

import (
	"fmt"
	"image"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/mapview"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type pathState struct {
	selected, lastSelected int
	list                   ui.ListState
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
	path.selected = min(
		path.selected,
		max(0, len(entry.data.Path(entry.selected))-1),
	)
	return path
}

func (entry *entryDraft) pathChanged() {
	entry.preview = nil
	for key := range entry.inputs {
		if key.index == entry.selected &&
			strings.HasPrefix(key.field, "path/") {
			delete(entry.inputs, key)
		}
	}
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
) {
	if !entry.data.SupportsPaths() {
		v.unitList(c, state, entry, units)
		return
	}
	ui.Tabs(c.Key("deployment-sidebar"), &entry.pathMode, locales.MoveUnits(), locales.MovePath()).
		Label(locales.Deployments() + " / " + locales.MovePath()).
		OnChange(func() {
			entry.lastSelected = -1
			if entry.pathMode == 1 {
				entry.pathState().lastSelected = -1
			}
		})
	if entry.pathMode == 0 {
		v.unitList(c, state, entry, units)
		return
	}
	if len(units) == 0 {
		ui.Text(c, locales.DeployEmpty()).TextColor(c.Theme().TextMuted)
		return
	}
	choices := make([]string, len(units))
	for index, unit := range units {
		choices[index] = fmt.Sprintf(
			"%02X · %s",
			index,
			nameOf(state.Session, records.Units, unit.Character),
		)
	}
	if !slices.Contains(choices, entry.pathUnitChoice) ||
		entry.pathUnitChoice != choices[entry.selected] {
		entry.pathUnitChoice = choices[entry.selected]
	}
	ui.Select(c.Key("path-unit"), &entry.pathUnitChoice, choices).
		MaxWidthPercent(100).
		Label(locales.MovePath() + " / " + locales.MoveUnits()).
		Disabled(state.Busy).
		OnChange(func() {
			if index := slices.Index(
				choices,
				entry.pathUnitChoice,
			); index >= 0 {
				entry.selected = index
				entry.lastSelected = -1
			}
		})
	v.pathEditor(c, state, entry)
}

func (v *Page) pathEditor(
	c *ui.Context,
	state *editor.State,
	entry *entryDraft,
) {
	path := entry.pathState()
	steps := entry.data.Path(entry.selected)
	disabled := state.Busy || !entry.data.Current(state.Session.Deployments)
	if len(steps) > 0 {
		path.list.Selected = &path.selected
		path.list.Key = func(index int) any { return index }
		columns := []ui.TableColumn{
			{ID: "step", Title: "#", Width: 34, Fixed: true},
			{ID: "position", Title: locales.DeployPosition()},
			{ID: "sync", Title: locales.Label(deployment.MoveSyncUnit)},
			{
				ID:    "delay",
				Title: locales.Label(deployment.MoveDelay),
				Width: 86,
				Fixed: true,
			},
		}
		ui.Table(c.Key("path-steps"), &path.list, columns, len(steps), func(index, column int) {
			step := steps[index]
			switch column {
			case 0:
				ui.Textf(c, "%02X", index).FontFeatures("tnum")
			case 1:
				ui.Textf(c, "%d, %d", step.Position.X, step.Position.Y).
					FontFeatures("tnum")
			case 2:
				if step.SyncUnit != 0 {
					ui.Text(c, characterCaption(state.Session, step.SyncUnit, false)).
						SingleLine()
				}
			case 3:
				ui.Textf(c, "%d", step.Delay).FontFeatures("tnum")
			}
		}).
			Height(min(144, c.Theme().Space(9)*float32(len(steps)+1))).
			Label(locales.MoveSteps()).
			Tooltip(locales.MoveScope())
	} else {
		ui.Text(c, locales.MoveEmpty()).TextColor(c.Theme().TextMuted)
	}
	ui.Row(c).Wrap().Gap(6).Children(func() {
		for _, action := range []struct {
			label     string
			enabled   bool
			run       func() error
			selection int
		}{
			{locales.MoveAdd(), len(steps) < deployment.MaxMovementSteps, func() error { return entry.data.AddPathStep(entry.selected) }, len(steps)},
			{locales.MoveDuplicate(), len(steps) > 0 && len(steps) < deployment.MaxMovementSteps, func() error { return entry.data.DuplicatePathStep(entry.selected, path.selected) }, path.selected + 1},
			{locales.MoveRemove(), len(steps) > 0, func() error { return entry.data.RemovePathStep(entry.selected, path.selected) }, max(0, path.selected-1)},
			{locales.MoveUp(), path.selected > 0, func() error { return entry.data.MovePathStep(entry.selected, path.selected, path.selected-1) }, path.selected - 1},
			{locales.MoveDown(), path.selected+1 < len(steps), func() error { return entry.data.MovePathStep(entry.selected, path.selected, path.selected+1) }, path.selected + 1},
		} {
			ui.Button(c, action.label).
				Disabled(disabled || !action.enabled).
				OnClick(func() {
					state.Err = action.run()
					if state.Err == nil {
						path.selected = action.selection
						entry.pathChanged()
					}
				})
		}
		point, selected := v.mapView.Selection()
		ui.Button(c, locales.MovePlace()).
			Disabled(disabled || !selected || len(steps) == 0).
			OnClick(func() {
				if state.Err = entry.data.SetPathField(
					entry.selected,
					path.selected,
					deployment.MoveX,
					int64(point.X),
				); state.Err == nil {
					state.Err = entry.data.SetPathField(
						entry.selected,
						path.selected,
						deployment.MoveY,
						int64(point.Y),
					)
				}
				entry.pathChanged()
			})
	})
	if len(steps) > deployment.MaxMovementSteps {
		ui.Text(c, locales.MoveLimit()).FontSize(12).TextColor(c.Theme().Danger)
	}
	if len(steps) == 0 {
		return
	}
	ui.Row(c).Wrap().Gap(12).Children(func() {
		for _, field := range entry.data.MovementFields() {
			ui.Row(c.Key(field.ID)).
				Width(190).
				MaxWidthPercent(100).
				Gap(6).
				AlignItems(ui.Center).
				Children(func() {
					help := locales.MoveScope()
					switch field.ID {
					case deployment.MoveSyncUnit, deployment.MoveSyncStep:
						help = locales.MoveSyncHelp()
					case deployment.MoveDelay:
						help = locales.MoveDelayHelp()
					case deployment.MoveConfig:
						help = locales.MoveConfigHelp()
					}
					ui.Text(c, locales.Label(field.ID)).
						Grow(1).
						FontSize(13).
						Tooltip(help)
					unit, node := entry.selected, path.selected
					value, _ := entry.data.PathValue(unit, node, field.ID)
					deploymentNumber(
						c,
						state,
						entry,
						field,
						inputKey{
							unit,
							fmt.Sprintf("path/%d/%s", node, field.ID),
						},
						value,
						func(value int64) error {
							err := entry.data.SetPathField(
								unit,
								node,
								field.ID,
								value,
							)
							if err == nil {
								path.lastSelected = -1
							}
							return err
						},
					)
				})
		}
	})
	ui.Textf(c, "% X", steps[path.selected].Raw).
		Font("monospace").
		FontSize(12).
		Selectable().
		Tooltip(locales.MoveScope())
}
