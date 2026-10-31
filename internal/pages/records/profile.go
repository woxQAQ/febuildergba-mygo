package records

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type (
	choiceDraft struct {
		text, label string
		value       int64
		initialized bool
	}
	profileState struct {
		section          int
		estimate, source bool
		targetInput      ui.Handle
		target           string
		choices          map[string]*choiceDraft
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
					ui.Row(c.Key("profile-toolbar")).
						Gap(12).
						AlignItems(ui.Center).
						Children(func() {
							ui.Tabs(c.Key("profile-section"), &entry.section, locales.RecordAbilities(), locales.RecordRelated()).
								Label(locales.RecordSections())
							ui.Box(c).Grow(1)
							ui.Checkbox(c.Key("raw"), &a.showRaw, locales.Advanced()).
								FontSize(12)
						})
					if entry.section == 0 {
						a.profileAbilities(c, kind, row, table, entry)
						a.profileWeapons(c, kind, row, table)
						a.profileTraits(c, kind, row)
					} else {
						a.profileRelated(c, kind, row, table, entry)
					}
					if a.showRaw {
						a.profileRaw(c, kind, row, table)
					}
				})
		})
}

func (a *form) profileIdentity(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
	entry *profileState,
	preview *assetPreview,
) {
	width, _ := c.Size()
	tracks := []ui.Track{ui.Fr(1)}
	if width-a.sidebar-40 >= 660 {
		tracks = []ui.Track{ui.Fixed(352), ui.Fr(1)}
	}
	ui.Column(c.Key("identity")).MaxWidth(800).Gap(12).Children(func() {
		ui.Row(c.Key("heading")).
			Wrap().
			Gap(16).
			AlignItems(ui.Start).
			Children(func() {
				ui.Column(c).Width(136).Shrink(0).Gap(6).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
						if preview != nil && preview.bitmap != nil {
							bounds := preview.resource.Image.Bounds()
							scale := min(float32(1), 96/float32(bounds.Dx()))
							ui.Image(c, preview.bitmap).
								Size(float32(bounds.Dx())*scale, float32(bounds.Dy())*scale).
								Label(row.Name + " " + locales.ImagePreview())
						} else {
							ui.Box(c).Size(96, 80).Background(c.Theme().Surface)
						}
						if preview != nil && preview.mini != nil {
							ui.Image(c, preview.mini).
								Size(32, 32).
								Label(locales.Label("portrait_mini"))
						}
					})
					if preview != nil && preview.source != nil &&
						preview.source.Inherited {
						ui.Textf(c, "%s %04X", locales.Label("portrait_inherit"), preview.ref.ID).
							FontSize(12).
							TextColor(c.Theme().TextMuted)
					}
				})
				ui.Row(c).
					Width(260).
					MaxWidthPercent(100).
					Grow(1).
					MinWidth(0).
					Gap(8).
					AlignItems(ui.Start).
					Children(func() {
						ui.Textf(c, "%02X", row.ID).
							Padding(8, 0, 0, 0).
							FontSize(13).
							FontFeatures("tnum").
							TextColor(c.Theme().TextMuted)
						ui.Column(c).
							Grow(1).
							MinWidth(0).
							Gap(6).
							Children(func() {
								a.nameEditor(c, kind, row)
								a.descriptions(c, kind, row, table, 48)
							})
					})
			})
		ui.Grid(c.Key("fields")).ColumnTracks(tracks...).GapX(20).GapY(8).
			AlignItems(ui.Start).Children(func() {
			field := "class_id"
			if kind == records.Classes {
				field = "promotion_id"
			}
			a.profileClass(c, kind, row, table, entry, field)
			if kind == records.Units {
				ui.Row(c.Key("level-affinity")).
					Gap(12).
					AlignItems(ui.Center).
					Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, locales.Label("level")).
								Width(84).
								FontSize(12)
							a.profileNumber(c, kind, row, table, "level", 48)
						})
						a.profileEnum(
							c,
							kind,
							row,
							table,
							entry,
							"affinity",
							[]string{
								"affinity_none",
								"affinity_fire",
								"affinity_thunder",
								"affinity_wind",
								"affinity_ice",
								"affinity_dark",
								"affinity_light",
								"affinity_anima",
							},
						)
					})
			}
			a.profilePortraitReference(c, kind, row, table, preview)
		})
	})
}

func (a *form) profilePortraitReference(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
	preview *assetPreview,
) {
	ui.Row(c.Key("portrait-reference")).
		Gap(6).
		AlignItems(ui.Center).
		Children(func() {
			ui.Text(c, locales.Label("portrait_id")).Width(84).FontSize(12)
			a.profileNumber(c, kind, row, table, "portrait_id", 64)
			if a.openPortrait != nil {
				ui.Button(c, locales.OpenPortraitTable()).
					Height(26).
					FontSize(12).
					Disabled(a.state.Busy || preview == nil || preview.ref.ID == 0).
					OnClick(func() { a.openPortrait(preview.ref.ID) })
			}
		})
	if kind != records.Units {
		return
	}
	ui.Row(c.Key("mini-reference")).
		Wrap().
		Gap(6).
		AlignItems(ui.Center).
		Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, locales.Label("portrait_mini_source")).
					Width(84).
					FontSize(12)
				a.profileNumber(c, kind, row, table, "mini_portrait", 48)
			})
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
			ui.Text(c, label).FontSize(12).TextColor(c.Theme().TextMuted)
			if source.MiniErr != nil {
				ui.Text(c, components.ErrorText(source.MiniErr)).
					FontSize(12).
					TextColor(c.Theme().Danger)
			}
			if source.GenericMini != 0 && len(source.MiniResources) > 0 {
				resource := source.MiniResources[0]
				if !resource.Empty && resource.Err == nil {
					ui.Button(c, locales.Label("portrait_generic_bytes")).
						Height(26).
						FontSize(12).
						Disabled(a.state.Busy).
						OnClick(func() { a.openHex(resource.Address, resource.Length) })
				}
			}
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

func (a *form) profileRelated(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
	entry *profileState,
) {
	var fields []records.Field
	for _, field := range table.Fields {
		if field.Group != records.GroupGeneral || field.ID == "id" ||
			field.Reference != "" ||
			field.ID == "affinity" ||
			field.ID == "movement_speed" || field.ID == "portrait_id" || field.ID == "mini_portrait" {
			continue
		}
		fields = append(fields, field)
	}
	a.inlineSection(c, kind, row, records.GroupGeneral, fields, 2)
	if kind == records.Classes {
		a.profileEnum(
			c,
			kind,
			row,
			table,
			entry,
			"movement_speed",
			[]string{"record_walk_standard", "record_walk_slow"},
		)
	}
	links, err := a.state.Session.Records.RecordResources(kind, row.ID)
	if err != nil {
		ui.Text(c, components.ErrorText(err)).TextColor(c.Theme().Danger)
		return
	}
	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, locales.RecordResourceLinks()).Bold().FontSize(13)
		if kind == records.Units && a.openSupport != nil {
			ui.Button(c, locales.SupportEditor()).
				Disabled(a.state.Busy).
				OnClick(func() { a.openSupport(row.ID) })
		}
		for _, link := range links {
			ui.Row(c.Key(link.Label)).
				Wrap().
				Gap(10).
				AlignItems(ui.Center).
				Children(func() {
					ui.Text(c, locales.Label(link.Label)).
						Width(160).
						FontSize(13)
					switch {
					case link.Empty:
						ui.Text(c, locales.RecordNoResource()).
							Grow(1).
							FontSize(12).
							TextColor(c.Theme().TextMuted)
					case link.Err != nil:
						ui.Text(c, components.ErrorText(link.Err)).
							Grow(1).
							FontSize(12).
							TextColor(c.Theme().Danger)
					default:
						ui.Textf(c, "0x%08X", link.Address).
							Font("monospace").
							FontSize(13).
							Grow(1)
						ui.Button(c, locales.EditBytes()).
							Height(26).
							Disabled(a.state.Busy).
							OnClick(func() { a.openHex(link.Address, link.Length) })
					}
				})
		}
	})
}

func (a *form) profileRaw(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
) {
	var fields []records.Field
	for _, field := range table.Fields {
		if field.Group == records.GroupAdvanced || field.ID == "id" {
			fields = append(fields, field)
		}
	}
	a.inlineSection(c, kind, row, records.GroupAdvanced, fields, 2)
	data, err := a.state.Session.Bytes.Slice(row.Address, table.Stride)
	if err != nil {
		return
	}
	ui.Textf(c, "0x%08X · %d B\n% X", row.Address, table.Stride, data).
		Font("monospace").
		FontSize(12).
		Selectable()
	ui.Button(c, locales.EditBytes()).
		Disabled(a.state.Busy).
		OnClick(func() { a.openHex(row.Address, table.Stride) })
}
