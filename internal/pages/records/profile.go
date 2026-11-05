package records

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

const (
	profileAbilitiesTab = iota
	profileSupportTab
	profileDefeatTab
	profileRawTab
	profileTerrainTab
)

const profileReferenceWidth float32 = 240

type profileTab struct {
	id    int
	label string
}

type (
	choiceDraft struct {
		text, label string
		value       int64
		initialized bool
	}
	profileState struct {
		section          int
		estimate, source bool
		portrait         ui.Handle
		targetInput      ui.Handle
		target           string
		choices          map[string]*choiceDraft
		terrain          terrainState
		support          supportState
	}
)

func (a *form) profileFor(kind records.Kind, id int) *profileState {
	if a.profiles == nil {
		a.profiles = make(map[fieldKey]*profileState)
	}
	key := fieldKey{Kind: kind, Record: id}
	entry := a.profiles[key]
	if entry == nil {
		target := "20"
		if kind == records.Classes {
			target = "19"
		}
		entry = &profileState{
			target:  target,
			choices: make(map[string]*choiceDraft),
		}
		a.profiles[key] = entry
	}
	return entry
}

func profileField(table *records.Table, id string) (records.Field, bool) {
	for _, field := range table.Fields {
		if field.ID == id {
			return field, true
		}
	}
	return records.Field{}, false
}

func (a *form) profile(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
) {
	entry := a.profileFor(kind, row.ID)
	tabs := profileTabs(table)
	entry.section = min(entry.section, len(tabs)-1)
	preview := a.previewFor(kind, row.ID)
	ui.Column(c.Key(fmt.Sprintf("profile/%s/%d", kind, row.ID))).
		Fill().
		Children(func() {
			ui.Scroll(c.Key("profile")).
				Grow(1).
				Padding(16).
				Gap(16).
				Children(func() {
					a.profileIdentity(c, kind, row, table, entry, preview)
					if preview != nil && preview.err != nil {
						ui.Text(c, components.ErrorText(preview.err)).
							TextColor(c.Theme().Danger)
					}
					sections := make([]string, len(tabs))
					for index, tab := range tabs {
						sections[index] = tab.label
					}
					ui.Tabs(c.Key("profile-section"), &entry.section, sections...).
						Label(locales.RecordSections())
					switch tabs[entry.section].id {
					case profileRawTab:
						a.profileRaw(c, kind, row, table)
					case profileAbilitiesTab:
						a.profileAbilities(c, kind, row, table, entry)
						a.profileTraits(c, kind, row)
					case profileTerrainTab:
						a.profileTerrain(c, row, entry)
					case profileDefeatTab:
						if a.defeatView != nil {
							width, _ := c.Size()
							a.defeatView(c, row.ID, width-a.sidebar-32)
						}
					case profileSupportTab:
						a.profileSupports(c, row, entry)
					}
				})
		})
	a.choosePortrait(c)
}

func profileTabs(table *records.Table) []profileTab {
	tabs := []profileTab{{profileAbilitiesTab, locales.RecordAbilities()}}
	if table.Kind == records.Classes {
		tabs = append(
			tabs,
			profileTab{profileTerrainTab, locales.TerrainEditor()},
		)
	} else {
		tabs = append(tabs,
			profileTab{profileSupportTab, locales.SupportEditor()},
			profileTab{profileDefeatTab, locales.Label("quote_death")},
		)
	}
	return append(tabs, profileTab{profileRawTab, locales.Advanced()})
}

func (a *form) selectProfileTab(kind records.Kind, id, target int) {
	table := a.state.Session.Records.Table(kind)
	if table == nil {
		return
	}
	for index, tab := range profileTabs(table) {
		if tab.id == target {
			a.profileFor(kind, id).section = index
			return
		}
	}
}

func (a *form) rawRecord(kind records.Kind, id int) bool {
	if kind == records.Units || kind == records.Classes {
		table := a.state.Session.Records.Table(kind)
		tabs := profileTabs(table)
		index := a.profileFor(kind, id).section
		return index < len(tabs) && tabs[index].id == profileRawTab
	}
	return false
}

func (a *form) profileIdentity(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
	entry *profileState,
	preview *assetPreview,
) {
	ui.Row(c.Key("identity")).
		WidthPercent(100).
		MinWidth(0).
		Wrap().
		Gap(12).
		AlignItems(ui.Start).
		Children(func() {
			a.profilePortrait(c, kind, row, entry, preview)
			ui.Column(c.Key("basic-info")).
				Basis(360).Grow(1).MinWidth(0).MaxWidthPercent(100).Gap(10).
				Children(func() {
					ui.Row(c.Key("name")).
						Gap(8).
						AlignItems(ui.Center).
						Children(func() {
							ui.Textf(c, "%02X", row.ID).
								FontSize(components.FontCaption).
								FontFeatures("tnum").
								TextColor(c.Theme().TextMuted)
							a.nameEditor(c, kind, row).Grow(1).MinWidth(0)
						})
					a.descriptions(c, kind, row, table, 44)
					ui.Row(c.Key("fields")).
						Wrap().Gap(12).AlignItems(ui.Start).
						Children(func() {
							field := "class_id"
							if kind == records.Classes {
								field = "promotion_id"
							}
							a.profileClass(c, kind, row, table, entry, field).
								Width(profileReferenceWidth).
								MinWidth(0).MaxWidthPercent(100)
							if kind == records.Units {
								ui.Column(c.Key("level")).
									Width(56).
									Gap(4).
									Children(func() {
										ui.Text(c, locales.Label("level")).
											FontSize(components.FontCaption)
										a.profileNumber(
											c,
											kind,
											row,
											table,
											"level",
											56,
										)
									})
								a.profileEnum(c, kind, row, table, entry, "affinity", []string{
									"affinity_none",
									"affinity_fire",
									"affinity_thunder",
									"affinity_wind",
									"affinity_ice",
									"affinity_dark",
									"affinity_light",
									"affinity_anima",
								}).
									Width(112)
							}
							a.profileMiniatureSource(
								c,
								kind,
								row,
								table,
								preview,
							)
							a.profileDetails(c, kind, row, table, entry)
						})
				})
		})
}

func (a *form) profileMiniatureSource(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
	preview *assetPreview,
) {
	if kind != records.Units {
		return
	}
	ui.Column(c.Key("mini-reference")).
		Width(120).
		MaxWidthPercent(100).
		Gap(4).
		Children(func() {
			ui.Text(c, locales.Label("portrait_mini_source")).
				FontSize(components.FontCaption)
			ui.Row(c).Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
				a.profileNumber(c, kind, row, table, "mini_portrait", 48)
				if preview == nil || preview.source == nil {
					return
				}
				source := preview.source
				label := locales.Label("portrait_mini_inherit")
				if source.GenericMini != 0 {
					label = fmt.Sprintf(
						"%s %d",
						locales.Label("portrait_mini_generic"),
						source.GenericMini,
					)
				}
				ui.Text(c, label).
					FontSize(components.FontCaption).
					TextColor(c.Theme().TextMuted)
				if source.MiniErr != nil {
					ui.Text(c, components.ErrorText(source.MiniErr)).
						FontSize(components.FontCaption).
						TextColor(c.Theme().Danger)
				}
			})
		})
}

func (a *form) profileNumber(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
	id string,
	width float32,
) {
	field, exists := profileField(table, id)
	if !exists {
		return
	}
	value, err := a.state.Session.Records.Value(kind, row.ID, id)
	if err != nil {
		ui.Text(c, components.ErrorText(err))
		return
	}
	a.numberField(c, kind, row, field, value).Width(width)
}

func profileRawField(field records.Field) bool {
	return field.Group == records.GroupAdvanced || field.Text != text.NoRole ||
		field.ID == "id" || field.ID == "sort_order" || field.ID == "visit_group"
}

func (a *form) profileDetails(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
	entry *profileState,
) {
	if kind == records.Classes {
		a.profileEnum(c, kind, row, table, entry, "movement_speed",
			[]string{"record_walk_standard", "record_walk_slow"}).
			Width(160).MinWidth(0).MaxWidthPercent(100)
	}
	for _, field := range table.Fields {
		if field.Group != records.GroupGeneral || profileRawField(field) ||
			field.Reference != "" || field.ID == "affinity" ||
			field.ID == "movement_speed" || field.ID == "portrait_id" || field.ID == "mini_portrait" {
			continue
		}
		ui.Column(c.Key(field.ID)).
			MinWidth(0).
			MaxWidthPercent(100).
			Gap(4).
			Children(func() {
				ui.Text(c, locales.Label(field.ID)).
					FontSize(components.FontCaption)
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					a.profileNumber(c, kind, row, table, field.ID, 64)
					if kind == records.Classes && field.ID == "map_sprite" {
						a.thumbnails.Standing(a.state.Session, row.ID).
							View(c, 26, locales.Label(field.ID))
					}
				})
			})
	}
}

func (a *form) profileRaw(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
) {
	var fields []records.Field
	for _, field := range table.Fields {
		if profileRawField(field) {
			fields = append(fields, field)
		}
	}
	a.inlineSection(c, kind, row, records.GroupAdvanced, fields, 2)
}
