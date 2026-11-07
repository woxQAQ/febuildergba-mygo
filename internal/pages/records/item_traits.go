package records

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (a *form) itemTraits(c *ui.Context, row records.Row, available float32) {
	flags, err := a.state.Session.Records.Value(records.Items, row.ID, "flags")
	if err != nil {
		ui.Text(c, components.ErrorText(err)).TextColor(c.Theme().Danger)
		return
	}
	traits := a.state.Session.Records.RecordTraits(records.Items)
	var known int64
	for _, trait := range traits {
		if trait.Known {
			known |= int64(1) << trait.Bit
		}
	}
	columns := 3
	if available < 600 {
		columns = 2
	}
	if available < 420 {
		columns = 1
	}
	ui.Column(c.Key("item-traits")).Gap(8).Children(func() {
		ui.Text(c, locales.RecordTraits()).
			Bold().
			Tooltip(locales.ItemTraitsHelp())
		ui.Grid(c).Columns(columns).GapX(16).GapY(12).Children(func() {
			for _, group := range []string{records.ItemTraitUsage, records.ItemTraitCombat, records.ItemTraitRestrictions} {
				ui.Column(c.Key(group)).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, locales.Label(group)).
						FontSize(components.FontCaption).
						TextColor(c.Theme().TextMuted)
					for _, trait := range traits {
						if !trait.Known || trait.Group != group {
							continue
						}
						on := flags&(int64(1)<<trait.Bit) != 0
						label := locales.Label(trait.Label)
						ui.Checkbox(c.Key(trait.Bit), &on, label).
							Height(23).
							Label(locales.RecordTraits() + " / " + label).
							Tooltip(locales.Label(trait.Help)).
							Disabled(a.state.Busy).
							OnClick(func() {
								a.state.SetErr(a.state.Edit(func() error {
									return a.state.Session.Records.SetRecordTrait(
										records.Items,
										row.ID,
										trait.Bit,
										on,
									)
								}))
							})
					}
				})
			}
		})
		if unknown := flags &^ known; unknown != 0 {
			ui.Text(c, locales.ItemUnknownTraits(fmt.Sprintf("0x%08X", unknown))).
				FontSize(components.FontCaption).
				TextColor(c.Theme().TextMuted)
		}
	})
}
