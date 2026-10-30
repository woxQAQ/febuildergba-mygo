package chapters

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type entryDraft struct {
	data                   *deployment.Draft
	preview                *deployment.Edit
	selected, lastSelected int
	list                   ui.ListState
	inputs                 map[inputKey]*numberDraft
	pathMode               int
	pathUnitChoice         string
	paths                  map[int]*pathState
}

type chapterSelection struct {
	choice  string
	pointer int
}

func (v *Page) OpenDeployment(state *editor.State, chapter, site int) {
	table := state.Session.Records.Table(records.Chapters)
	if table == nil || table.Err != nil {
		state.Err = edit.ErrLayout
		return
	}
	for _, row := range table.Rows {
		if row.ID == chapter {
			v.load(state, row)
			v.mode = 1
			v.selections[chapter] = &chapterSelection{pointer: site}
			return
		}
	}
	state.Err = edit.ErrField
}

func (v *Page) draft(
	state *editor.State,
	group deployment.Group,
) *entryDraft {
	entry := v.entries[group.Pointer]
	if entry != nil && !entry.data.Modified() && !entry.invalid() &&
		!entry.data.Current(state.Session.Deployments) {
		delete(v.entries, group.Pointer)
		entry = nil
	}
	if entry == nil {
		data, err := state.Session.Deployments.NewDraft(
			v.chapter,
			group.Pointer,
		)
		if err != nil {
			state.Err = err
			return nil
		}
		entry = &entryDraft{data: data, lastSelected: -1}
		v.entries[group.Pointer] = entry
	}
	return entry
}

func (v *Page) caption(group deployment.Group) string {
	count, suffix := len(group.Units), ""
	if entry := v.entries[group.Pointer]; entry != nil {
		count = entry.data.Count()
		if entry.data.Modified() {
			suffix = " *"
		}
	}
	return fmt.Sprintf(
		"%s · %d · 0x%08X%s",
		locales.Label(group.Label),
		count,
		group.Pointer,
		suffix,
	)
}

func (v *Page) group(
	c *ui.Context,
	state *editor.State,
) (deployment.Group, bool) {
	if v.err != nil {
		ui.Text(c, components.ErrorText(v.err)).
			Tooltip(v.err.Error()).
			TextColor(c.Theme().Danger)
		return deployment.Group{}, false
	}
	if len(v.resource.Groups) == 0 {
		return deployment.Group{}, false
	}
	selection := v.selections[v.chapter]
	if selection == nil {
		selection = &chapterSelection{pointer: v.resource.Groups[0].Pointer}
		for _, group := range v.resource.Groups {
			if group.Err == nil && len(group.Units) > 0 {
				selection.pointer = group.Pointer
				break
			}
		}
		v.selections[v.chapter] = selection
	}
	options := make([]string, len(v.resource.Groups))
	selected := 0
	for i, group := range v.resource.Groups {
		options[i] = v.caption(group)
		if group.Pointer == selection.pointer {
			selected = i
		}
	}
	if !slices.Contains(options, selection.choice) {
		selection.choice = options[selected]
	}
	ui.Select(c.Key("deployment-entry"), &selection.choice, options).
		Label(locales.Deployments() + " / " + locales.DeployEntry()).
		MaxWidthPercent(100).Disabled(state.Busy).
		OnChange(func() {
			if index := slices.Index(options, selection.choice); index >= 0 {
				selection.pointer = v.resource.Groups[index].Pointer
			}
		})
	for _, group := range v.resource.Groups {
		if group.Pointer == selection.pointer {
			return group, true
		}
	}
	selection.pointer = v.resource.Groups[selected].Pointer
	return v.resource.Groups[selected], true
}

func (e *entryDraft) invalid() bool {
	for _, input := range e.inputs {
		if input.invalid {
			return true
		}
	}
	return false
}

func (v *Page) actions(
	c *ui.Context,
	state *editor.State,
	group deployment.Group,
	entry *entryDraft,
) {
	stale := !entry.data.Current(state.Session.Deployments)
	disabled := state.Busy || stale
	v.unitActions(c, state, entry)
	if stale {
		ui.Text(c, locales.DeployStale()).TextColor(c.Theme().Danger)
	}
	if entry.data.Modified() || stale || entry.invalid() {
		ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Button(c, locales.DeployPreview()).
				Disabled(disabled || entry.invalid() || !entry.data.Modified()).
				OnClick(func() {
					entry.preview, state.Err = state.Session.Deployments.Plan(
						entry.data,
					)
				})
			ui.Button(c, locales.DeployReload()).
				Disabled(state.Busy).
				OnClick(func() { delete(v.entries, group.Pointer) })
			if entry.data.Modified() {
				ui.Text(c, locales.DeployDraft()).
					FontSize(12).
					TextColor(c.Theme().TextMuted)
			}
		})
	}
}

func (v *Page) preview(
	c *ui.Context,
	state *editor.State,
	group deployment.Group,
	entry *entryDraft,
	jump func(records.Kind, int),
) {
	if entry.preview == nil {
		return
	}
	preview := entry.preview
	ui.Column(c.Key("deployment-preview")).Gap(8).Children(func() {
		ui.Text(c, locales.DeployPreview()).Bold()
		ui.Textf(c, "%s: %d → %d · 0x%08X · %d B", locales.Deployments(), preview.Before(), preview.After(), preview.Destination(), preview.Length()).
			FontFeatures("tnum").
			Selectable()
		ui.Text(c, locales.DeployPreviewScope()).
			FontSize(12).
			TextColor(c.Theme().TextMuted)
		ui.Text(c, locales.DeployUses()).Bold()
		ui.Row(c).Wrap().Gap(8).Children(func() {
			for _, use := range preview.Uses() {
				ui.Button(c, fmt.Sprintf("%02X %s", use.Chapter, nameOf(state.Session, records.Chapters, use.Chapter))).
					Disabled(state.Busy).
					OnClick(func() { jump(records.Chapters, use.Chapter) })
			}
		})
		ui.Button(c, locales.DeployApply()).
			Disabled(state.Busy || !entry.data.Current(state.Session.Deployments) || entry.invalid()).
			OnClick(func() {
				if state.Err = state.Session.Deployments.Apply(
					preview,
				); state.Err == nil {
					delete(v.entries, group.Pointer)
					state.HistoryEpoch++
				}
			})
	})
}
