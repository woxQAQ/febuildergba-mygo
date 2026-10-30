package records

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

type profileMatrix struct {
	stats, groups []string
	fields        map[string]records.Field
}

func profileStats(table *records.Table) profileMatrix {
	result := profileMatrix{fields: make(map[string]records.Field)}
	present := make(map[string]bool)
	for _, group := range []string{records.GroupBase, records.GroupMaximum, records.GroupGrowth, records.GroupPromotion} {
		found := false
		for _, field := range table.Fields {
			prefix, stat, ok := strings.Cut(field.ID, ".")
			if !ok || prefix != group || field.Group != group {
				continue
			}
			result.fields[field.ID] = field
			present[stat] = true
			found = true
		}
		if found {
			result.groups = append(result.groups, group)
		}
	}
	if table.Kind == records.Units {
		present[records.StatMovement] = true
	}
	for _, stat := range []string{records.StatHP, records.StatPower, records.StatMagic, records.StatSkill, records.StatSpeed, records.StatDefense, records.StatResistance, records.StatLuck, records.StatConstitution, records.StatMovement} {
		if present[stat] {
			result.stats = append(result.stats, stat)
			delete(present, stat)
		}
	}
	for _, field := range table.Fields {
		_, stat, ok := strings.Cut(field.ID, ".")
		if ok && present[stat] {
			result.stats = append(result.stats, stat)
			delete(present, stat)
		}
	}
	return result
}

func (a *form) profileAbilities(
	c *ui.Context,
	kind records.Kind,
	row records.Row,
	table *records.Table,
	entry *profileState,
) {
	matrix := profileStats(table)
	target, parseErr := strconv.Atoi(entry.target)
	reference, referenceErr := a.state.Session.Records.ReferenceAbilities(
		kind,
		row.ID,
		target,
		entry.estimate,
	)
	if entry.estimate && parseErr != nil {
		referenceErr = edit.ErrValue
	}
	width, _ := c.Size()
	available := max(float32(300), width-a.sidebar-40)
	ui.Column(c.Key("profile-abilities")).Gap(6).Children(func() {
		ui.Row(c).Wrap().Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.Abilities()).Bold().FontSize(13).Grow(1)
			if kind == records.Units {
				ui.Checkbox(c, &entry.source, locales.RecordClassSource()).
					FontSize(12)
			}
			ui.Checkbox(c, &entry.estimate, locales.RecordEstimate()).
				FontSize(12).
				Tooltip(locales.RecordEstimateHelp())
			if entry.estimate {
				a.profileTarget(c, kind, entry, referenceErr)
			}
		})
		if referenceErr != nil && (kind == records.Units || entry.estimate) {
			ui.Text(c, locales.RecordReferenceUnavailable()+": "+components.ErrorText(referenceErr)).
				FontSize(12).
				TextColor(c.Theme().TextMuted)
		}
		ui.ScrollHorizontal(c).
			Width(available).
			MaxWidthPercent(100).
			Children(func() {
				tracks := []ui.Track{ui.Fixed(92)}
				for range matrix.stats {
					tracks = append(tracks, ui.Fr(1))
				}
				ui.Grid(c).
					Width(max(available, 92+float32(len(matrix.stats))*54)).
					ColumnTracks(tracks...).
					GapX(6).
					GapY(3).
					AlignItems(ui.Center).
					Children(func() {
						ui.Text(c, locales.RecordAbilityRow()).
							FontSize(12).
							TextColor(c.Theme().TextMuted)
						for _, stat := range matrix.stats {
							ui.Text(c, locales.Label(stat)).
								FontSize(12).
								TextAlign(ui.End).
								PaddingX(6)
						}
						for _, group := range matrix.groups {
							label := locales.Label("matrix_" + group)
							if group == records.GroupBase &&
								kind == records.Units {
								label = locales.PersonalBase()
							}
							ui.Text(c, label).FontSize(13)
							for _, stat := range matrix.stats {
								field, exists := matrix.fields[group+"."+stat]
								if !exists {
									profileMissing(c)
									continue
								}
								value, err := a.state.Session.Records.Value(
									kind,
									row.ID,
									field.ID,
								)
								if err != nil {
									ui.Text(c, components.ErrorText(err))
									continue
								}
								a.numberField(c, kind, row, field, value)
							}
						}
						if kind == records.Units && entry.source {
							a.profileReferenceRow(
								c,
								matrix,
								locales.RecordClassBase(),
								reference.ClassBase,
								false,
							)
						}
						if kind == records.Units || entry.estimate {
							label := locales.RecordInitialReference()
							if entry.estimate {
								label = locales.RecordEstimate()
							}
							values := reference.Values
							if referenceErr != nil {
								values = nil
							}
							a.profileReferenceRow(
								c,
								matrix,
								label,
								values,
								entry.estimate,
							)
						}
					})
			})
	})
}

func (a *form) profileTarget(
	c *ui.Context,
	kind records.Kind,
	entry *profileState,
	err error,
) {
	label := locales.RecordTargetLevel()
	if kind == records.Classes {
		label = locales.RecordGrowthCount()
	}
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).
			FontSize(12).
			OnClick(func() { entry.targetInput.Focus() })
		input := components.TextDraft(
			a.state,
			ui.TextInput(c, &entry.target).
				Width(48).
				Height(26).
				Padding(3, 6).
				TextAlign(ui.End).
				FontSize(13).
				FontFeatures("tnum").
				Label(label).Bind(&entry.targetInput).
				Disabled(a.state.Busy),
		)
		if err != nil {
			input.Error(locales.RecordEstimateRange())
		}
	})
}

func (a *form) profileReferenceRow(
	c *ui.Context,
	matrix profileMatrix,
	label string,
	values []records.AbilityValue,
	decimal bool,
) {
	ui.Text(c, label).FontSize(13).Tooltip(locales.RecordEstimateHelp())
	for _, stat := range matrix.stats {
		found := false
		for _, value := range values {
			if value.Stat != stat {
				continue
			}
			text := strconv.FormatInt(value.Hundredths/100, 10)
			if decimal && value.Hundredths%100 != 0 {
				text = strings.TrimRight(strings.TrimRight(strconv.FormatFloat(
					float64(value.Hundredths)/100,
					'f',
					2,
					64,
				), "0"), ".")
			}
			ui.Text(c, text).
				FontSize(13).
				FontFeatures("tnum").
				TextAlign(ui.End).
				PaddingX(6).
				Height(26)
			found = true
			break
		}
		if !found {
			profileMissing(c)
		}
	}
}

func profileMissing(c *ui.Context) {
	ui.Text(c, "—").
		FontSize(13).
		TextAlign(ui.End).
		PaddingX(6).
		TextColor(c.Theme().TextMuted).
		Height(26)
}
