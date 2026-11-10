package quotes

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/quote"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/components/icons"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type battleContent struct {
	scriptOpen bool
	menu       ui.Handle
}

// Text selection is an independent task. Dismissing it leaves the row's pending
// fields intact, and confirmation resolves the stable row identity again.
type battleTextEdit struct {
	entry     *tableState
	row       uint64
	id        uint32
	reference components.TextReferenceState
	open      bool
}

func (p *Page) battleDetail(
	c *ui.Context,
	table quote.Table,
	entry *tableState,
) {
	width, _ := c.Size()
	// Context size is window space; columns use the editor's remaining width.
	available := min(float32(880), max(0, width-p.sidebar.Width()-40))
	stale := entry.draft == nil || !entry.draft.Current(p.state.Session.Quotes)
	disabled := stale || p.state.Busy
	ui.Scroll(c.Key(entry)).Fill().Children(func() {
		ui.Column(c).
			FillWidth().
			MaxWidth(920).
			Padding(20).
			Gap(16).
			Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, p.battleTitle(entry)).
						FontSize(components.FontPanelTitle).Bold().Grow(1)
					components.ActionToggle(
						c,
						&p.showHelp,
						icons.Lucide.CircleHelp(),
						locales.QuoteMatchingRules(),
					)
				})
				if p.showHelp {
					ui.Text(c, p.tableHelp(table.Kind)).
						FontSize(components.FontCaption)
				}
				if entry.err != nil {
					ui.Text(c, components.ErrorText(entry.err)).
						TextColor(c.Theme().Danger).Tooltip(entry.err.Error())
				}
				if stale && entry.draft != nil {
					ui.Text(c, locales.DeployStale()).
						TextColor(c.Theme().Danger)
				}
				if entry.draft == nil {
					p.actions(c, entry, stale)
					return
				}
				index := entry.index()
				if index < 0 {
					ui.Text(c, locales.QuoteNoMatches()).
						TextColor(c.Theme().TextMuted)
					p.actions(c, entry, stale)
					return
				}
				row := entry.rows[index]
				ui.Column(c.Key(row.ID)).Gap(16).Children(func() {
					p.battleFields(
						c.Key("participants"),
						entry,
						index,
						disabled,
						available,
						true,
					)
					p.battleFields(
						c.Key("conditions"),
						entry,
						index,
						disabled,
						available,
						false,
					)
					p.battleBody(c.Key("dialog"), entry, index, disabled)
				})
				p.actions(c, entry, stale)
			})
	})
}

func (p *Page) battleTitle(entry *tableState) string {
	if entry.draft != nil && entry.index() >= 0 {
		if condition, err := entry.draft.BattleCondition(
			entry.index(),
		); err == nil {
			switch condition.Participants {
			case quote.Pair:
				return locales.QuotePairs()
			case quote.Attacker:
				return locales.QuoteAttackerOnly()
			case quote.Defender:
				return locales.QuoteDefenderOnly()
			case quote.Neither:
				return locales.QuoteInactive()
			}
		}
	}
	switch p.tables[p.category].Kind {
	case quote.Battle:
		return locales.QuotePairs()
	case quote.General:
		return locales.QuoteSingles()
	case quote.Triangle:
		return locales.QuoteTriangle()
	default:
		return p.tableLabel(p.tables[p.category].Kind)
	}
}

func (p *Page) battleFields(
	c *ui.Context,
	entry *tableState,
	index int,
	disabled bool,
	available float32,
	participants bool,
) {
	var fields []quote.Field
	for _, field := range entry.draft.Fields() {
		person := field.ID == quote.Unit || field.ID == quote.Opponent
		if person != participants || field.ID == quote.Text ||
			field.ID == quote.Script ||
			field.ID == quote.FinalText {
			continue
		}
		fields = append(fields, field)
	}
	paired := participants && len(fields) == 2 && available >= 520
	var flow ui.Element
	if participants && !paired || !participants && available < 380 {
		flow = ui.Column(c).Gap(16)
	} else {
		flow = ui.Row(c).Gap(16).AlignItems(ui.Center)
	}
	flow.Children(func() {
		for position, field := range fields {
			width := available
			switch {
			case paired:
				width = (available - 48) / 2
				if position > 0 {
					ui.Icon(c, icons.Lucide.Swords()).Size(16, 16).Shrink(0)
				}
			case participants && len(fields) == 1:
				width = min(420, available)
			case !participants && available >= 380 && field.ID == quote.Flag:
				width = 112
			case !participants && available >= 380:
				width = min(420, available-128)
			}
			ui.Row(c.Key(field.ID)).
				Width(width).
				MinWidth(0).
				Shrink(0).
				MaxWidthPercent(100).
				Gap(8).
				AlignItems(ui.Center).
				Children(func() {
					fieldWidth := width
					if participants {
						value, _ := entry.draft.Value(index, field.ID)
						if value != 0 && value <= 255 {
							portrait := p.thumbnails.Portrait(
								p.state.Session,
								int(value),
							)
							if portrait.Available() {
								portrait.View(c, 32, p.unitName(value))
								fieldWidth -= 40
							}
						}
					}
					if field.ID != quote.Flag {
						fieldWidth -= 34
					}
					ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
						p.field(
							c,
							entry,
							index,
							field,
							disabled,
							max(100, fieldWidth),
							!participants,
						)
					})
				})
		}
	})
}

func (p *Page) battleBody(
	c *ui.Context,
	entry *tableState,
	index int,
	disabled bool,
) {
	row := entry.rows[index]
	content := entry.content[row.ID]
	if content == nil {
		content = &battleContent{}
		entry.content[row.ID] = content
	}
	textID, textErr := entry.draft.Value(index, quote.Text)
	script, scriptErr := entry.draft.Value(index, quote.Script)
	ui.Column(c).Gap(10).Children(func() {
		ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.QuoteText()).
				FontSize(components.FontPanelTitle).
				Bold()
			if textID != 0 && textErr == nil {
				ui.Textf(c, "%04X", textID).
					FontFeatures("tnum").
					TextColor(c.Theme().TextMuted)
			}
			ui.Box(c).Grow(1)
			if textID != 0 && textErr == nil {
				label := locales.QuoteEditText()
				if entry.draft.Modified() {
					label = locales.DefeatApplyThenEdit()
				}
				ui.Button(c, label).
					Disabled(disabled || entry.invalid() || textID > 65535 || p.openText == nil).
					OnClick(func() { p.editBattleText(entry, textID) })
			}
			label := locales.QuoteChangeText()
			if textID == 0 {
				label = locales.QuoteSetText()
			}
			ui.Button(c, label).Bind(&p.textAction).
				Disabled(disabled || textErr != nil).
				OnClick(func() {
					p.textEdit = &battleTextEdit{
						entry: entry,
						row:   row.ID,
						id:    textID,
						open:  true,
					}
				})
			if scriptErr == nil || textErr == nil && textID != 0 {
				components.ActionButton(c, icons.Lucide.Ellipsis(), locales.QuoteContentActions()).
					Bind(&content.menu).
					Disabled(disabled).
					PopoverMenuButton(func(menu *ui.Menu) {
						if scriptErr == nil &&
							menu.Item(locales.QuoteLinkScript()).
								Disabled(disabled).
								Chosen() {
							content.scriptOpen = true
						}
						if menu.Item(locales.QuoteClearText()).
							Disabled(disabled || textID == 0 || textErr != nil).
							Chosen() {
							p.state.SetErr(
								entry.draft.Set(index, quote.Text, 0),
							)
							entry.preview, entry.editText = nil, 0
							p.textAction.Focus()
						}
					})
			}
		})
		if textID != 0 {
			p.battleTextPreview(c, textID)
		} else if script != 0 {
			ui.Text(c, locales.QuoteUsingScript()).
				TextColor(c.Theme().TextMuted)
		} else {
			ui.Text(c, locales.QuoteNoContent()).TextColor(c.Theme().TextMuted)
		}
		if scriptErr == nil && (script != 0 || content.scriptOpen) {
			for _, field := range entry.draft.Fields() {
				if field.ID == quote.Script {
					p.field(
						c.Key(field.ID),
						entry,
						index,
						field,
						disabled,
						240,
						true,
					)
				}
			}
		}
	})
}

func (p *Page) battleTextPreview(c *ui.Context, id uint32) {
	if id > 65535 {
		ui.Text(c, locales.GameInvalidValue()).TextColor(c.Theme().Danger)
		return
	}
	value, err := p.state.Session.Texts.Value(uint16(id))
	if err != nil {
		ui.Text(c, components.ErrorText(err)).TextColor(c.Theme().Danger)
		return
	}
	body := value.Plain
	if strings.TrimSpace(body) == "" {
		body = locales.QuoteControlOnly()
	}
	height := min(
		float32(360),
		max(float32(88), float32(strings.Count(body, "\n")+1)*20+24),
	)
	ui.Scroll(c.Key("body")).Height(height).Padding(12, 0).
		BorderWidth(1, 0, 1, 0).BorderColor(c.Theme().Border).
		Label(locales.QuoteReadingPreview()).Children(func() {
		ui.Text(c, body).MaxWidth(560).Selectable()
	})
}

func (p *Page) editBattleText(entry *tableState, id uint32) {
	if p.state.Busy || id == 0 || id > 65535 || p.openText == nil ||
		!entry.draft.Current(p.state.Session.Quotes) || entry.invalid() {
		return
	}
	if !entry.draft.Modified() {
		p.openText(uint16(id))
		return
	}
	entry.preview, entry.err = p.state.Session.Quotes.Plan(entry.draft)
	p.state.Report("quote.preview", entry.err)
	if entry.err == nil {
		entry.editText = uint16(id)
	}
}

func (p *Page) closeBattleTextTask() {
	if task := p.textEdit; task != nil {
		task.open = false
		p.textEdit = nil
		p.textAction.Focus()
	}
}

func (p *Page) battleTextTask(c *ui.Context) {
	task := p.textEdit
	if task == nil {
		return
	}
	if !task.open {
		p.closeBattleTextTask()
		return
	}
	width, height := c.Size()
	modal := ui.Modal(c.Key(task), &task.open, func() {
		stale := !task.entry.draft.Current(p.state.Session.Quotes)
		ui.Column(c).
			Width(min(float32(560), max(float32(0), width-64))).
			Gap(12).
			Children(func() {
				ui.Text(c, locales.QuoteChangeText()).
					FontSize(components.FontPanelTitle).
					Bold()
				if stale {
					ui.Text(c, locales.DeployStale()).
						TextColor(c.Theme().Danger)
				}
				ui.Scroll(c).
					MaxHeight(max(float32(100), height-200)).
					Children(func() {
						components.TextReference(
							c,
							p.state,
							&task.reference,
							task.id,
							locales.QuoteChangeText()+" / "+locales.QuoteText(),
							stale,
							func(id uint16) { task.id = uint32(id) },
							nil,
						)
					})
				ui.Row(c).Gap(8).Children(func() {
					ui.PrimaryButton(c, locales.QuoteChangeText()).
						Disabled(stale || p.state.Busy || task.id > 65535).
						OnClick(func() {
							if p.state.Busy ||
								!task.entry.draft.Current(
									p.state.Session.Quotes,
								) {
								return
							}
							for index, row := range task.entry.draft.Entries() {
								if row.ID == task.row {
									p.state.SetErr(
										task.entry.draft.Set(
											index,
											quote.Text,
											task.id,
										),
									)
									if p.state.Err == nil {
										task.entry.preview, task.entry.editText = nil, 0
										p.closeBattleTextTask()
									}
									return
								}
							}
						})
					ui.Button(c, locales.Cancel()).
						OnClick(p.closeBattleTextTask)
				})
			})
	})
	if modal.Dismissed() {
		p.closeBattleTextTask()
	}
}
