// Package components owns window chrome, per-window editing state and shared
// UI widgets. Page content and navigation belong to pages; system interfaces
// belong to app, and pure value parsing belongs to internal/input.
package components

import (
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

const HeaderHeight = 48

func ApplyTheme(c *ui.Context) {
	t := *c.Theme()
	t.Spacing, t.Radius = 3, 4
	t.Accent = ui.Hex("#C44B2C")
	t.AccentHover = t.Accent.Mix(ui.RGB(0, 0, 0), 0.1)
	t.AccentPressed = t.Accent.Mix(ui.RGB(0, 0, 0), 0.2)
	t.Success = ui.Hex("#4A7C59")
	t.Warning = ui.Hex("#D4941E")
	t.Danger = ui.Hex("#B3332D")
	c.SetTheme(&t)
}

func Header(c *ui.Context, state *editor.State, controls func()) {
	t := c.Theme()
	bar := c.TitleBar()
	ui.Row(c).Height(max(HeaderHeight, bar.Height)).Shrink(0).
		Padding(0, bar.Right+16, 0, bar.Left+16).Gap(12).AlignItems(ui.Center).
		Background(t.Background).
		BorderWidth(0, 0, 1, 0).
		BorderColor(t.Border).DragWindow().Children(func() {
		title := locales.AppTitle()
		if state.Session != nil {
			title = filepath.Base(state.Session.Document.State().Path)
		}
		ui.Text(c, title).
			FontSize(14).
			Bold().
			SingleLine().
			Width(0).
			Grow(1).
			MinWidth(0)
		ui.Row(c).Gap(12).Shrink(0).AlignItems(ui.Center).Children(func() {
			if state.Session != nil && state.Session.Document.State().Dirty {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Width(6).Height(6).Radius(3).Background(t.Warning)
					ui.Text(c, locales.Dirty()).
						FontSize(12).
						TextColor(t.Warning)
				})
				ui.Box(c).Width(1).Height(16).Background(t.Border)
			}
			controls()
			language, tag := "English", "en"
			if i18n.Base(i18n.Locale()) != "zh" {
				language, tag = "中文", "zh-Hans"
			}
			ui.Button(c.Key("interface-language"), language).FontSize(12).
				OnClick(func() { mygo.App.SetLocale(tag) })
		})
	})
}

func ErrorBanner(c *ui.Context, state *editor.State) {
	if state.Err == nil {
		return
	}
	t := c.Theme()
	ui.Row(c).
		Padding(10, 16).
		Gap(8).
		AlignItems(ui.Center).
		Background(t.Background).
		BorderWidth(0, 0, 1, 0).
		BorderColor(t.Border).
		Children(func() {
			ui.Text(c, locales.ErrorTitle()).Bold().TextColor(t.Danger)
			ui.Text(c, ErrorText(state.Err)).
				Grow(1).
				Selectable().
				Tooltip(state.Err.Error())
			ui.Button(c, locales.Dismiss()).OnClick(func() { state.Err = nil })
		})
}

func ExternalChangeBanner(c *ui.Context, state *editor.State, reload func()) {
	if !state.External {
		return
	}
	t := c.Theme()
	ui.Row(c).
		Padding(10, 16).
		Gap(12).
		AlignItems(ui.Center).
		Background(t.Background).
		BorderWidth(0, 0, 1, 0).
		BorderColor(t.Border).
		Children(func() {
			ui.Text(c, locales.External()).Grow(1)
			ui.Button(c, locales.Reload()).
				Disabled(state.Busy || reload == nil).
				OnClick(func() {
					if reload != nil {
						reload()
					}
				})
		})
}

func StatusBar(c *ui.Context, state *editor.State, run func()) {
	document := state.Session.Document.State()
	ui.Row(c).
		Padding(8, 16).
		Gap(12).
		AlignItems(ui.Center).
		BorderWidth(1, 0, 0, 0).
		BorderColor(c.Theme().Border).
		Children(func() {
			ui.Textf(c, "%s  %.2f MiB", state.Session.Version(), float64(document.Size)/(1<<20)).
				TextColor(c.Theme().TextMuted)
			ui.Box(c).Grow(1)
			status := locales.Ready()
			if document.Dirty {
				status = locales.Dirty()
			}
			if state.Busy {
				status = locales.Working()
			}
			ui.Text(c, status)
			ui.Button(c, locales.Run()).
				Disabled(state.Busy || run == nil).
				OnClick(func() {
					if run != nil {
						run()
					}
				})
		})
}
