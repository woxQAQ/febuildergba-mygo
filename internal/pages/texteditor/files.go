package texteditor

import (
	"errors"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (p *Page) SetFileActions(
	importFile func(*text.File, func(*text.Import, string, error)),
	exportFile func(*text.File, []uint16),
) {
	p.importFile, p.exportFile = importFile, exportFile
}

func (p *Page) Deactivate() { p.fileEpoch++ }

func (p *Page) chooseFile() {
	if p.state.Busy || p.importFile == nil {
		return
	}
	p.fileEpoch++
	session, epoch := p.state.Session, p.fileEpoch
	source, err := session.Texts.File()
	if err != nil {
		p.fileErr = err
		p.state.Report("editor.operation", err)
		return
	}
	p.importFile(source, func(draft *text.Import, name string, err error) {
		if p.state.Session != session || p.fileEpoch != epoch ||
			!source.Current(session.Texts) {
			return
		}
		if err != nil {
			p.fileErr = err
			return
		}
		if draft == nil {
			return
		}
		p.fileDraft, p.fileName, p.fileRows = draft, name, draft.Entries()
		p.fileSelection, p.fileList = 0, ui.ListState{}
		p.filePlan, p.fileErr = session.Texts.PlanImport(draft)
		if p.filePlan != nil {
			p.fileRows = p.filePlan.Entries()
		}
		if errors.Is(p.fileErr, edit.ErrValue) {
			p.fileErr = nil
		}
		p.state.Report("editor.operation", p.fileErr)
		p.reviewFocus.Focus()
	})
}

func (p *Page) fileActions(c *ui.Context) {
	var ids []uint16
	pending := false
	for _, entry := range p.rows {
		if entry.Err != nil {
			continue
		}
		ids = append(ids, entry.ID)
		if draft := p.drafts[entry.ID]; draft != nil &&
			draft.text != draft.original {
			pending = true
		}
	}
	ui.Column(c.Key("text-files")).Gap(6).Shrink(0).Children(func() {
		ui.Row(c).Wrap().Gap(8).Children(func() {
			ui.Button(c.Key("export"), locales.TextFileExport()).
				Disabled(p.state.Busy || p.exportFile == nil || len(ids) == 0).
				OnClick(func() {
					source, err := p.state.Session.Texts.File()
					p.fileErr = err
					p.state.Report("editor.operation", err)
					if err == nil {
						p.exportFile(source, ids)
					}
				})
			ui.Button(c.Key("import"), locales.TextFileImport()).
				Bind(&p.filesFocus).
				Disabled(p.state.Busy || p.importFile == nil).
				OnClick(p.chooseFile)
		})
		if skipped := len(p.rows) - len(ids); skipped > 0 {
			ui.Text(c, locales.TextFileSkipped(skipped)).
				FontSize(components.FontCaption)
		}
		if pending {
			ui.Text(c, locales.TextFileExportDrafts()).
				FontSize(components.FontCaption)
		}
		if p.fileErr != nil {
			ui.Text(c, components.ErrorText(p.fileErr)).
				TextColor(c.Theme().Danger).
				Tooltip(p.fileErr.Error())
		}
	})
}

func (p *Page) closeFileReview(c *ui.Context) {
	p.fileDraft, p.filePlan, p.fileRows, p.fileErr = nil, nil, nil, nil
	p.fileName = ""
	p.fileEpoch++
	p.filesFocus.Focus(c)
}

func fileRowStatus(row text.FileEntry) string {
	if row.Err != nil {
		return locales.TextFileErrors()
	}
	if row.Changed {
		return locales.TextFileChanged()
	}
	return locales.TextFileUnchanged()
}

func (p *Page) fileReview(c *ui.Context) {
	changed, invalid := 0, 0
	for _, row := range p.fileRows {
		if row.Err != nil {
			invalid++
		} else if row.Changed {
			changed++
		}
	}
	current := p.fileDraft.Current(p.state.Session.Texts)
	ui.Scroll(c.Key("file-review")).
		Grow(1).
		MinHeight(0).
		Padding(12).
		Gap(8).
		Children(func() {
			ui.Text(c.Key("filename"), p.fileName).
				Bold().
				SingleLine().
				Tooltip(p.fileName)
			if p.fileErr != nil {
				ui.Text(c, components.ErrorText(p.fileErr)).
					TextColor(c.Theme().Danger).
					Tooltip(p.fileErr.Error())
			}
			ui.Row(c.Key("actions")).Wrap().Gap(8).Children(func() {
				ui.PrimaryButton(c, locales.TextFileApply()).
					Disabled(p.state.Busy || p.fileErr != nil || !p.filePlan.Current(p.state.Session.Texts)).
					OnClick(func() {
						p.fileErr = p.state.Edit(
							func() error { return p.state.Session.Texts.ApplyImport(p.filePlan) },
						)
						if p.fileErr != nil {
							return
						}
						for _, row := range p.fileRows {
							if row.Changed {
								delete(p.drafts, row.ID)
							}
						}
						p.state.HistoryEpoch++
						p.closeFileReview(c)
						p.refresh()
					})
				ui.Button(c.Key("choose"), locales.TextFileChoose()).
					Bind(&p.reviewFocus).
					Disabled(p.state.Busy || p.importFile == nil).
					OnClick(p.chooseFile)
				ui.Button(c, locales.TextFileReturn()).
					Disabled(p.state.Busy).
					OnClick(func() { p.closeFileReview(c) })
			})
			if !current {
				ui.Text(c, locales.DeployStale()).TextColor(c.Theme().Danger)
			}
			for _, row := range p.fileRows {
				if draft := p.drafts[row.ID]; row.Changed && row.Err == nil &&
					draft != nil &&
					draft.text != draft.original {
					ui.Text(c, locales.TextFileReplaceDrafts()).
						TextColor(c.Theme().Warning)
					break
				}
			}
			ui.Row(c.Key("summary")).
				Wrap().
				Gap(12).
				AlignItems(ui.Center).
				Children(func() {
					ui.Textf(
						c,
						"%s: %d · %s: %d · %s: %d",
						locales.TextFileChanged(),
						changed,
						locales.TextFileUnchanged(),
						len(p.fileRows)-changed-invalid,
						locales.TextFileErrors(),
						invalid,
					)
					if p.filePlan != nil {
						ui.Textf(c, "+%d B", p.filePlan.AddedBytes()).
							FontSize(components.FontCaption).
							TextColor(c.Theme().TextMuted)
					}
				})
			p.fileList.Selected = &p.fileSelection
			p.fileList.Key = func(index int) any { return p.fileRows[index].ID }
			ui.Table(c.Key("entries"), &p.fileList, []ui.TableColumn{
				{ID: "id", Title: "ID", Width: 70},
				{ID: "line", Title: locales.TextFileLine(), Width: 100},
				{ID: "status", Title: locales.TextFileStatus(), Width: 100},
				{ID: "text", Title: locales.TextContent()},
			}, len(p.fileRows), func(index, column int) {
				row := p.fileRows[index]
				switch column {
				case 0:
					ui.Textf(c, "%04X", row.ID).FontFeatures("tnum")
				case 1:
					ui.Textf(c, "%d", row.Line).FontFeatures("tnum")
				case 2:
					ui.Text(c, fileRowStatus(row))
				default:
					if row.Err != nil {
						ui.Text(c, components.ErrorText(row.Err)).
							SingleLine().
							TextColor(c.Theme().Danger)
					} else {
						ui.Text(c, strings.ReplaceAll(row.After, "\n", " ")).
							SingleLine()
					}
				}
			}).Height(float32(min(220, max(80, (len(p.fileRows)+1)*30)))).Disabled(p.state.Busy)
			ui.Column(c.Key("details")).Gap(8).Children(func() {
				if p.fileSelection < 0 || p.fileSelection >= len(p.fileRows) {
					return
				}
				row := p.fileRows[p.fileSelection]
				if row.Err != nil {
					ui.Text(c, components.ErrorText(row.Err)).
						TextColor(c.Theme().Danger)
				}
				ui.Column(c.Key(row.ID)).Gap(8).Children(func() {
					ui.Row(c.Key("comparison")).
						Wrap().
						Gap(12).
						AlignItems(ui.Start).
						Children(func() {
							ui.Column(c.Key("before-column")).
								Width(360).
								Grow(1).
								MaxWidthPercent(100).
								Gap(6).
								Children(func() {
									ui.Text(c, locales.Before()).Bold()
									before := row.Before
									ui.TextArea(c.Key("before"), &before).
										ReadOnly(true).
										Height(140).Label(locales.Before())
								})
							ui.Column(c.Key("after-column")).
								Width(360).
								Grow(1).
								MaxWidthPercent(100).
								Gap(6).
								Children(func() {
									ui.Text(c, locales.After()).Bold()
									after := row.After
									ui.TextArea(c.Key("after"), &after).
										ReadOnly(true).
										Height(140).Label(locales.After())
								})
						})
					if p.filePlan != nil && row.Changed {
						ui.Text(c, locales.TextEditSummary(fmt.Sprintf("0x%04X", row.ID), len(row.Uses))).
							FontSize(components.FontCaption)
						ui.Row(c.Key("uses")).Wrap().Gap(8).Children(func() {
							for _, use := range row.Uses {
								ui.Button(c.Key(use), fmt.Sprintf("%s %02X · %s", locales.Label(use.Kind), use.ID, locales.Label(use.Field))).
									Disabled(p.state.Busy || !current || p.openRecord == nil).
									OnClick(func() { p.openRecord(records.Kind(use.Kind), use.ID) })
							}
						})
					}
				})
			})
		})
}
