package chapters

import (
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type chapterMode int

const (
	configurationMode chapterMode = iota
	mapMode
	mapChangesMode
	deploymentMode
	triggerMode
	talkMode
	locationMode
	graphicsMode
	paletteMode
)

type chapterNavigation struct {
	selected, previous int
	width              float32
	locale             string
}

func (v *Page) mode() chapterMode { return chapterMode(v.navigation.selected) }

func (v *Page) selectMode(mode chapterMode) {
	if v.mode() != mode {
		v.resourceEpoch++
	}
	v.navigation.selected = int(mode)
}

func (v *Page) tabs(c *ui.Context, width float32) {
	labels := []string{
		locales.DeployConfiguration(),
		locales.MapEditor(),
		locales.MapChangesEditor(),
		locales.Deployments(),
		locales.ChapterTriggers(),
		locales.TalkEvents(),
		locales.LocationEvents(),
		locales.MapGraphicsEditor(),
		locales.MapPaletteEditor(),
	}
	navigation := &v.navigation
	follow := navigation.previous != navigation.selected ||
		navigation.width != width ||
		navigation.locale != c.Locale()
	ui.ScrollHorizontal(c.Key("chapter-navigation")).
		Scrollbars(false).
		Width(width).
		MaxWidthPercent(100).
		Children(func() {
			tabs := ui.TabsBase(
				c.Key("chapter-mode"),
				&navigation.selected,
				len(labels),
			)
			items := make([]ui.Element, len(labels))
			tabs.List.Label(locales.Chapters()).
				Gap(c.Theme().Space(1)).
				OnChange(func() { v.resourceEpoch++ }).
				Children(func() {
					for index, label := range labels {
						tab := tabs.Tab(index).
							Padding(c.Theme().Space(2), c.Theme().Space(3)).
							Label(label).
							BorderWidth(0, 0, 2, 0).
							BorderColor(ui.Transparent).
							TextColor(c.Theme().TextMuted)
						if index == navigation.selected {
							tab.Bold().
								TextColor(c.Theme().Text).
								BorderColor(c.Theme().Accent)
						} else if tab.Hovered() || tab.Pressed() {
							tab.TextColor(c.Theme().Text)
						}
						items[index] = tab
						tab.Children(func() { ui.Text(c, label).SingleLine() })
					}
				})
			if follow || navigation.previous != navigation.selected {
				items[navigation.selected].ScrollIntoView()
			}
		})
	navigation.previous, navigation.width, navigation.locale = navigation.selected, width, c.Locale()
}
