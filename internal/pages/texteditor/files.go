package texteditor

import (
	"errors"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
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
		p.fileComparison = nil
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
	p.fileComparison = nil
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

// comparison is the read-only comparison state of the selected import entry:
// it keeps identity by import source, entry and Before/After strings, and
// preserves the DiffView scroll position and text selection across builds.
// The buffers are display copies of Before/After, never a source of truth
// for the import, the document or history.
type comparison struct {
	source        *text.Import
	id            uint16
	before, after string
	diff          *editor.DiffState
}

// comparisonSplitWidth is the available logical width in DIP at which the
// comparison lays the two texts side by side instead of unified. The
// fileReview scroll view pads its content by 12 on each side.
const comparisonSplitWidth = 760

// comparisonFor returns the comparison state of the selected entry,
// rebuilding the buffers and DiffState only when the import, entry or the
// actual strings change. Both sides stay read-only, without hunk actions,
// and the full texts are never collapsed.
func (p *Page) comparisonFor(row text.FileEntry) *editor.DiffState {
	cmp := p.fileComparison
	if cmp == nil || cmp.source != p.fileDraft || cmp.id != row.ID ||
		cmp.before != row.Before || cmp.after != row.After {
		cmp = &comparison{
			source: p.fileDraft,
			id:     row.ID,
			before: row.Before,
			after:  row.After,
			diff: &editor.DiffState{
				Old:             editor.NewBuffer(row.Before),
				New:             editor.NewBuffer(row.After),
				Editable:        false,
				HunkActions:     false,
				Wrap:            true,
				ExpandUnchanged: true,
			},
		}
		p.fileComparison = cmp
	}
	return cmp.diff
}

// comparisonView presents the full Before/After of the selected entry as a
// DiffView: side by side on wide windows and unified on narrow ones, in a
// bounded height that scrolls internally so the full texts stay reachable.
// Theme and font size follow the window defaults.
func (p *Page) comparisonView(c *ui.Context, row text.FileEntry) {
	diff := p.comparisonFor(row)
	width, _ := c.Size()
	if width-24 >= comparisonSplitWidth {
		diff.Layout = editor.DiffSplit
		ui.Row(c.Key("sides")).Gap(12).Children(func() {
			ui.Text(c.Key("before"), locales.Before()).Bold().Grow(1)
			ui.Text(c.Key("after"), locales.After()).Bold().Grow(1)
		})
	} else {
		diff.Layout = editor.DiffUnified
		ui.Text(c.Key("sides"), locales.Before()+" → "+locales.After()).
			Bold()
	}
	editor.DiffView(c.Key("comparison"), diff).FillWidth().Height(240)
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
					p.comparisonView(c, row)
					if p.filePlan != nil && row.Changed {
						ui.Text(c, locales.TextEditSummary(fmt.Sprintf("0x%04X", row.ID), len(row.Uses))).
							FontSize(components.FontCaption)
						components.TextUses(
							c.Key("file-uses"),
							p.state,
							row.Uses,
							!row.UsesIncomplete,
							!current,
							p.openUse,
						)
					}
				})
			})
		})
}
