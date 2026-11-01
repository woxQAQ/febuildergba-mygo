package supports

import (
	"fmt"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/support"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type bonusState struct {
	draft  *support.BonusDraft
	plan   *support.BonusPlan
	err    error
	inputs map[string]*valueInput
}

func (s *bonusState) invalid() bool {
	for _, value := range s.inputs {
		if value.invalid {
			return true
		}
	}
	return false
}

func affinityCaption(id uint8) string {
	labels := []string{
		locales.AffinityNone(),
		locales.AffinityFire(),
		locales.AffinityThunder(),
		locales.AffinityWind(),
		locales.AffinityIce(),
		locales.AffinityDark(),
		locales.AffinityLight(),
		locales.AffinityAnima(),
	}
	if int(id) < len(labels) {
		return fmt.Sprintf("%02X %s", id, labels[id])
	}
	return fmt.Sprintf("%02X", id)
}

func bonusLabels() [6]string {
	return [6]string{
		locales.SupportBonusAttack(),
		locales.SupportBonusDefense(),
		locales.SupportBonusHit(),
		locales.SupportBonusAvoid(),
		locales.SupportBonusCritical(),
		locales.SupportBonusDodge(),
	}
}

func (p *Page) bonusView(c *ui.Context) {
	e := p.state.Session.Supports
	entry := p.bonuses
	if entry != nil && entry.draft != nil && !entry.draft.Current(e) &&
		!entry.draft.Modified() &&
		!entry.invalid() {
		entry = nil
	}
	if entry == nil {
		entry = &bonusState{inputs: make(map[string]*valueInput)}
		entry.draft, entry.err = e.NewBonusDraft()
		p.bonuses = entry
	}
	reload := func() { p.bonuses = nil }
	ui.Scroll(c.Key(entry)).Fill().Padding(16).Gap(12).Children(func() {
		if entry.draft == nil {
			ui.Text(c, components.ErrorText(entry.err)).
				TextColor(c.Theme().Danger)
			ui.Button(c, locales.QuoteReload()).
				Disabled(p.state.Busy).
				OnClick(reload)
			return
		}
		stale := !entry.draft.Current(e)
		if stale {
			entry.plan = nil
			ui.Text(c, locales.SupportStale()).TextColor(c.Theme().Danger)
		}
		ui.Row(c).Wrap().Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.SupportBonusHelp()).
				FontSize(13).
				Tooltip(locales.SupportBonusScope())
			ui.Checkbox(c, &p.advanced, locales.Advanced())
		})
		if entry.draft.Modified() || entry.invalid() || stale {
			ui.Row(c).Wrap().Gap(8).Children(func() {
				ui.Button(c, locales.SupportPreview()).
					Disabled(p.state.Busy || stale || entry.invalid() || !entry.draft.Modified()).
					OnClick(func() { entry.plan, entry.err = e.PlanBonuses(entry.draft) })
				ui.Button(c, locales.DeployReload()).
					Disabled(p.state.Busy).
					OnClick(reload)
				if entry.invalid() {
					ui.Text(c, locales.SupportBonusRange()).
						FontSize(12).
						TextColor(c.Theme().Danger)
				}
			})
		}
		if entry.err != nil {
			ui.Text(c, components.ErrorText(entry.err)).
				TextColor(c.Theme().Danger)
		}
		if entry.plan != nil {
			p.bonusPreview(c, entry, reload)
		}
		rows := entry.draft.Rows()
		if len(rows) == 0 {
			ui.Text(c, locales.SupportBonusEmpty())
		}
		width, _ := c.Size()
		available := max(float32(300), width-32)
		ui.ScrollHorizontal(c.Key("bonus-table")).
			Width(available).
			MaxWidthPercent(100).
			Children(func() {
				ui.Grid(c).
					Width(max(available, 720)).
					ColumnTracks(ui.Fixed(144), ui.Fr(1), ui.Fr(1), ui.Fr(1), ui.Fr(1), ui.Fr(1), ui.Fr(1)).
					GapX(8).
					GapY(4).
					Padding(0, 0, 8, 0).
					AlignItems(ui.Center).
					Children(func() {
						ui.Text(c, locales.Affinity()).FontSize(13).Bold()
						for _, label := range bonusLabels() {
							ui.Text(c, label).
								FontSize(12).
								TextAlign(ui.End).
								PaddingX(6)
						}
						for _, row := range rows {
							caption := affinityCaption(row.Affinity)
							label := ui.Text(c, caption).
								FontSize(13).
								SingleLine()
							if !row.Effective {
								label.TextColor(c.Theme().TextMuted).
									Tooltip(locales.SupportBonusShadowed())
							}
							for kind, value := range row.Values {
								p.bonusField(
									c,
									entry,
									row,
									support.BonusKind(kind),
									value,
									caption,
									stale,
								)
							}
						}
					})
			})
		if p.advanced {
			ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Textf(c, "0x%08X", entry.draft.Address()).
					Font("monospace").
					FontSize(12)
				ui.Button(c, locales.EditBytes()).
					Disabled(p.state.Busy || stale || p.openHex == nil).
					OnClick(func() { p.openHex(entry.draft.Address(), len(entry.draft.Raw())) })
			})
		}
	})
}

func (p *Page) bonusField(
	c *ui.Context,
	entry *bonusState,
	row support.AffinityBonuses,
	kind support.BonusKind,
	value uint8,
	caption string,
	stale bool,
) {
	key := fmt.Sprintf("%d/%d", row.Index, kind)
	field := entry.inputs[key]
	if field == nil {
		field = &valueInput{text: strconv.Itoa(int(value))}
		entry.inputs[key] = field
	}
	help := locales.SupportBonusScope()
	if !row.Effective {
		help = locales.SupportBonusShadowed() + "\n" + help
	}
	control := components.TextDraft(
		p.state,
		ui.TextInput(c.Key(key), &field.text).
			Height(26).
			Padding(3, 6).FontSize(13).FontFeatures("tnum").TextAlign(ui.End).
			Label(caption+" / "+bonusLabels()[kind]).Tooltip(help).
			Disabled(p.state.Busy || stale).OnChange(func() {
			entry.plan = nil
			parsed, err := input.ParseInteger(field.text)
			field.invalid = err != nil || parsed < 0 || parsed > 255
			if !field.invalid {
				entry.err = entry.draft.Set(row.Index, kind, int(parsed))
			}
		}),
	)
	if field.invalid {
		control.Error(locales.SupportBonusRange())
	}
}

func (p *Page) bonusPreview(c *ui.Context, entry *bonusState, reload func()) {
	ui.Column(c.Key("bonus-preview")).
		Gap(8).
		Padding(8, 0).
		BorderWidth(1, 0).
		BorderColor(c.Theme().Border).
		Children(func() {
			ui.Text(c, locales.SupportBonusApplyHelp()).FontSize(12)
			if entry.plan.UsesError() != nil {
				ui.Text(c, locales.SupportBonusUsersIncomplete()+" "+components.ErrorText(entry.plan.UsesError())).
					FontSize(12)
			}
			for _, change := range entry.plan.Changes() {
				ui.Column(c.Key(change.Index)).Gap(4).Children(func() {
					ui.Text(c, affinityCaption(change.Affinity)).
						FontSize(13).
						Bold()
					ui.Row(c).Wrap().Gap(12).Children(func() {
						for kind, value := range change.After {
							if value != change.Before[kind] {
								ui.Textf(c, "%s: %d → %d", bonusLabels()[kind], change.Before[kind], value).
									FontSize(12)
							}
						}
					})
					if !change.Effective {
						ui.Text(c, locales.SupportBonusShadowed()).FontSize(12)
					}
					if len(change.Units) > 0 {
						ui.Scroll(c).MaxHeight(100).Gap(2).Children(func() {
							for _, unit := range change.Units {
								ui.Button(c.Key(unit.ID), fmt.Sprintf("%02X %s", unit.ID, unit.Name)).
									AlignSelf(ui.Start).
									Disabled(p.state.Busy || p.openSupport == nil).
									OnClick(func() { p.openSupport(unit.ID) })
							}
						})
					}
				})
			}
			if p.advanced {
				ui.Textf(c, "0x%08X → 0x%08X", entry.plan.Before(), entry.plan.Destination()).
					Font("monospace").
					FontSize(12)
			}
			ui.Button(c, locales.SupportApply()).AlignSelf(ui.Start).
				Disabled(p.state.Busy || entry.invalid() || !entry.plan.Current(p.state.Session.Supports)).
				OnClick(func() {
					entry.err = p.state.Session.Supports.ApplyBonuses(
						entry.plan,
					)
					if entry.err == nil {
						reload()
						p.state.HistoryEpoch++
						p.state.Session.Refresh()
					}
				})
		})
}
