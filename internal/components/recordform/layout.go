package recordform

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/locales"
)

type statMatrix struct {
	columns, rows []string
	fields        map[string]game.Field
}

func arrangeStats(fields []game.Field) statMatrix {
	matrix := statMatrix{fields: make(map[string]game.Field)}
	var order []string
	present := make(map[string]bool)
	for _, group := range []string{game.GroupBase, game.GroupGrowth, game.GroupMaximum, game.GroupPromotion} {
		found := false
		for _, f := range fields {
			prefix, stat, ok := strings.Cut(f.ID, ".")
			if f.Group != group || !ok || prefix != group || stat == "" {
				continue
			}
			matrix.fields[f.ID] = f
			found = true
			if !present[stat] {
				present[stat] = true
				order = append(order, stat)
			}
		}
		if found {
			matrix.columns = append(matrix.columns, group)
		}
	}
	for _, stat := range []string{game.StatHP, game.StatPower, game.StatMagic, game.StatSkill, game.StatSpeed, game.StatLuck, game.StatDefense, game.StatResistance, game.StatConstitution, game.StatMovement} {
		if present[stat] {
			matrix.rows = append(matrix.rows, stat)
			delete(present, stat)
		}
	}
	// Additional adapter fields retain their schema order after the common
	// abilities.
	for _, stat := range order {
		if present[stat] {
			matrix.rows = append(matrix.rows, stat)
		}
	}
	return matrix
}

func (a *Form) recordFields(
	c *ui.Context,
	kind game.Kind,
	row game.Row,
	table *game.Table,
) {
	matrix := arrangeStats(table.Fields)
	sections := make(map[string][]game.Field)
	var extra []string
	for _, f := range table.Fields {
		if _, inMatrix := matrix.fields[f.ID]; inMatrix {
			continue
		}
		group := f.Group
		if group == game.GroupBase {
			group = game.GroupGeneral
		}
		if group != game.GroupGeneral && group != game.GroupCombat &&
			group != game.GroupWeapon &&
			group != game.GroupAdvanced &&
			sections[group] == nil {
			extra = append(extra, group)
		}
		sections[group] = append(sections[group], f)
	}
	width, _ := c.Size()
	available := max(float32(240), width-a.sidebar-40)
	matrixWidth := float32(124 + len(matrix.columns)*94)
	withWeapons := len(matrix.columns) > 0 &&
		len(sections[game.GroupWeapon]) > 0
	ui.Column(c.Key("field-groups")).Gap(24).Children(func() {
		if withWeapons {
			weaponWidth := available
			if available >= matrixWidth+248 {
				weaponWidth = available - matrixWidth - 24
			}
			columns := 4
			if weaponWidth < 440 {
				columns = 2
			}
			ui.Row(c.Key("abilities-and-weapons")).
				Wrap().
				Gap(24).
				AlignItems(ui.Start).
				Children(func() {
					a.abilityMatrix(c, kind, row, matrix).
						Width(matrixWidth).
						MaxWidthPercent(100).
						Shrink(0)
					a.inlineSection(c, kind, row, game.GroupWeapon, sections[game.GroupWeapon], columns).
						Width(weaponWidth).
						MaxWidth(600).
						Shrink(0)
				})
		} else if len(matrix.columns) > 0 {
			a.abilityMatrix(c, kind, row, matrix)
		}
		groups := append(
			[]string{game.GroupCombat, game.GroupWeapon, game.GroupGeneral},
			extra...)
		groups = append(groups, game.GroupAdvanced)
		for _, group := range groups {
			fields := sections[group]
			if len(fields) == 0 ||
				(group == game.GroupAdvanced && !a.showRaw) ||
				(group == game.GroupWeapon && withWeapons) {
				continue
			}
			columns := 2
			if group == game.GroupWeapon {
				columns = 4
			}
			if available < 440 {
				columns = max(1, columns/2)
			}
			a.inlineSection(c, kind, row, group, fields, columns).MaxWidth(600)
		}
	})
}

func (a *Form) inlineSection(
	c *ui.Context,
	kind game.Kind,
	row game.Row,
	group string,
	fields []game.Field,
	columns int,
) ui.Element {
	// Share input and action widths so each row keeps its values aligned.
	inputWidth, actionWidth := float32(56), float32(0)
	for _, f := range fields {
		if f.Width > 2 {
			inputWidth = 104
		} else if f.Width == 2 {
			inputWidth = max(inputWidth, 72)
		}
		if f.Reference != "" || f.Text != game.NoText {
			actionWidth = 48
		}
	}
	return ui.Column(c.Key(group)).Gap(8).Children(func() {
		ui.Text(c, locales.Label(group)).Bold().FontSize(13)
		ui.Grid(c).Columns(columns).GapX(16).GapY(4).Children(func() {
			for _, f := range fields {
				a.inlineField(c, kind, row, f, inputWidth, actionWidth)
			}
		})
	})
}

func (a *Form) abilityMatrix(
	c *ui.Context,
	kind game.Kind,
	row game.Row,
	matrix statMatrix,
) ui.Element {
	tracks := []ui.Track{ui.Fixed(124)}
	for range matrix.columns {
		tracks = append(tracks, ui.Fr(1))
	}
	return ui.Column(c.Key("abilities")).
		MaxWidth(float32(124 + len(matrix.columns)*94)).
		Gap(4).
		Children(func() {
			ui.Grid(c.Key("head")).ColumnTracks(tracks...).GapX(6).
				Padding(0, 0, 6, 0).
				BorderWidth(0, 0, 1, 0).
				BorderColor(c.Theme().Border).Children(func() {
				ui.Text(c, locales.Abilities()).FontSize(13).Bold()
				for _, group := range matrix.columns {
					label := "matrix_" + group
					if group == game.GroupBase && kind == game.Units {
						label = "personal_base"
					}
					ui.Text(c, locales.Label(label)).
						FontSize(12).
						TextAlign(ui.End).
						PaddingX(6).
						SingleLine().
						Tooltip(locales.Label(label))
				}
			})
			ui.Grid(c.Key("values")).
				ColumnTracks(tracks...).
				GapX(6).
				GapY(2).
				AlignItems(ui.Center).
				Children(func() {
					for _, stat := range matrix.rows {
						ui.Text(c.Key(stat), locales.Label(stat)).
							FontSize(13).
							SingleLine()
						for _, group := range matrix.columns {
							id := group + "." + stat
							f, exists := matrix.fields[id]
							if !exists {
								ui.Text(c.Key(id), "—").
									TextAlign(ui.End).
									PaddingX(6).
									Height(26).
									TextColor(c.Theme().TextMuted).
									Label(locales.Label(stat) + ": " + locales.NotApplicable())
								continue
							}
							value, err := a.state.Session.Value(
								kind,
								row.ID,
								f.ID,
							)
							if err != nil {
								ui.Text(c.Key(id), components.ErrorText(err))
								continue
							}
							a.numberField(c, kind, row, f, value)
						}
					}
				})
		})
}

func (a *Form) inlineField(
	c *ui.Context,
	kind game.Kind,
	row game.Row,
	f game.Field,
	inputWidth, actionWidth float32,
) {
	ui.Row(c.Key(f.ID)).Gap(6).MinHeight(28).Children(func() {
		key := fieldKey{kind, row.ID, f.ID}
		label := locales.Label(f.ID)
		help := label
		if kind == game.Chapters &&
			(f.ID == "objective_text_id" || f.ID == "objective_detail_text_id") {
			help += "\n" + locales.ChapterObjectiveHelp()
		}
		ui.Text(c, label).
			FontSize(13).
			SingleLine().
			MaxWidth(140).
			MinWidth(0).
			Grow(1).
			Tooltip(help).
			OnClick(func() { a.focusField(key) })
		value, err := a.state.Session.Value(kind, row.ID, f.ID)
		if err != nil {
			ui.Text(c, components.ErrorText(err))
			return
		}
		a.numberField(c, kind, row, f, value).Width(inputWidth).Shrink(0)
		switch {
		case f.Reference != "":
			ui.Button(c, locales.Jump()).
				Size(actionWidth, 26).
				Padding(3, 6).
				Shrink(0).
				Disabled(a.state.Busy).
				OnClick(func() { a.jump(f.Reference, int(value)) })
		case f.Text != game.NoText:
			ui.Button(c, locales.Jump()).
				Size(actionWidth, 26).
				Padding(3, 6).
				Shrink(0).
				Disabled(a.state.Busy || value <= 0 || value >= 0x7FFF).
				OnClick(func() {
					if value > 0 && value < 0x7FFF {
						a.openText(uint16(value))
					}
				})
		case actionWidth > 0:
			ui.Box(c).Width(actionWidth).Shrink(0)
		}
	})
}
