package components

import (
	"fmt"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/internal/input"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

// NumberCell describes one numeric table cell. Key must be comparable and stay
// with its record through reordering. Domain validation and writes stay in Set.
type NumberCell struct {
	Key             any
	Value, Min, Max int64
	Label, Help     string
	Selected        bool
	Focus           *ui.Handle
	CanEdit         func() bool
	Select          func()
	Changed         func()
	Set             func(int64) error
}

func (cell NumberCell) editable() bool {
	return cell.Set != nil && (cell.CanEdit == nil || cell.CanEdit())
}

type numberCellDraft struct {
	text    string
	value   int64
	invalid bool
	failure error
	cell    NumberCell
}

type numberCellEdit struct {
	key   any
	fresh bool
}

// NumberCells owns on-demand numeric inputs, unfinished text and pointer
// activation for a table. The page owns the rows, selection and domain draft.
type NumberCells struct {
	drafts map[any]*numberCellDraft
	active *numberCellEdit
	focus  ui.Handle
	click  struct {
		key   any
		at    time.Time
		scope *ui.Handle
	}
}

// Pending reports an open editor, including valid text not yet confirmed.
func (cells *NumberCells) Pending() bool { return cells.active != nil }

func (cells *NumberCells) Invalid() bool {
	for _, draft := range cells.drafts {
		if draft.invalid || draft.failure != nil {
			return true
		}
	}
	return false
}

// Copy preserves unfinished text independently; the destination receives its
// own domain binding when it is rendered.
func (cells *NumberCells) Copy(source, destination any) {
	if draft := cells.drafts[source]; draft != nil {
		cells.drafts[destination] = &numberCellDraft{
			text:    draft.text,
			value:   draft.value,
			invalid: draft.invalid,
			failure: draft.failure,
		}
	}
}

func (cells *NumberCells) Forget(key any) {
	delete(cells.drafts, key)
	if cells.active != nil && cells.active.key == key {
		cells.active = nil
	}
	if cells.click.key == key {
		cells.click.at = time.Time{}
	}
}

// Flush confirms focus leaving before the page constructs preview/apply
// actions. A stale or busy domain binding retains its unconfirmed input.
func (cells *NumberCells) Flush(c *ui.Context) {
	if cells.click.scope != nil && !cells.click.scope.FocusWithin(c) {
		cells.click.at = time.Time{}
	}
	edit := cells.active
	if edit == nil || edit.fresh || cells.focus.FocusWithin(c) {
		return
	}
	draft := cells.drafts[edit.key]
	if !draft.cell.editable() {
		return
	}
	cells.commit()
	cells.active = nil
}

// Begin is also the keyboard entry point, after the target cell is rendered.
func (cells *NumberCells) Begin(key any) {
	draft := cells.drafts[key]
	if draft == nil || !draft.cell.editable() {
		return
	}
	if cells.active != nil {
		if cells.active.key == key {
			return
		}
		if !cells.drafts[cells.active.key].cell.editable() {
			return
		}
		cells.commit()
	}
	if draft.cell.Select != nil {
		draft.cell.Select()
	}
	cells.click.at = time.Time{}
	cells.active = &numberCellEdit{key: key, fresh: true}
}

func (draft *numberCellDraft) problem() string {
	if draft.failure != nil {
		return ErrorText(draft.failure)
	}
	if draft.invalid {
		return fmt.Sprintf(
			"%s: %d…%d",
			locales.AllowedRange(),
			draft.cell.Min,
			draft.cell.Max,
		)
	}
	return ""
}

func (draft *numberCellDraft) validate() (int64, bool) {
	value, err := input.ParseInteger(draft.text)
	draft.failure = nil
	draft.invalid = err != nil || value < draft.cell.Min ||
		value > draft.cell.Max
	return value, !draft.invalid
}

func (cells *NumberCells) commit() bool {
	draft := cells.drafts[cells.active.key]
	if !draft.cell.editable() {
		return false
	}
	value, valid := draft.validate()
	if !valid {
		return false
	}
	if value != draft.value {
		if err := draft.cell.Set(value); err != nil {
			draft.failure = err
			return false
		}
	}
	draft.text, draft.value = strconv.FormatInt(value, 10), value
	return true
}

// Cell renders plain text until activation. Header, text and input share the
// native table's trailing padding; the entire cell is the pointer target.
func (cells *NumberCells) Cell(c *ui.Context, cell NumberCell) ui.Element {
	if cells.drafts == nil {
		cells.drafts = make(map[any]*numberCellDraft)
	}
	draft := cells.drafts[cell.Key]
	if draft == nil {
		draft = &numberCellDraft{
			text:  strconv.FormatInt(cell.Value, 10),
			value: cell.Value,
		}
		cells.drafts[cell.Key] = draft
	} else if draft.value != cell.Value {
		draft.text, draft.value = strconv.FormatInt(cell.Value, 10), cell.Value
		draft.invalid, draft.failure = false, nil
	}
	draft.cell = cell
	edit := cells.active
	editing := edit != nil && edit.key == cell.Key && cell.editable()
	vertical, horizontal := c.Theme().Space(1.5), c.Theme().Space(2.5)
	box := ui.Row(c.Key("cell")).Grow(1).MinWidth(0).Height(26+2*vertical).
		Margin(-vertical, -horizontal).
		Padding(vertical, horizontal).AlignItems(ui.Center)
	var control ui.Element
	box.Children(func() {
		if editing {
			control = ui.TextInput(c.Key("input"), &draft.text).
				Bind(&cells.focus).
				Grow(1).
				MinWidth(0).
				Height(26).
				Margin(0, -1).
				Padding(3, 0).
				TextAlign(ui.End).
				FontFeatures("tnum").
				TextColor(c.Theme().Text).
				Label(cell.Label).
				OnChange(func() {
					draft.validate()
					if cell.Changed != nil {
						cell.Changed()
					}
				}).
				OnTextKey(func(key ui.TextKey) bool {
					if key.Mods != 0 {
						return false
					}
					switch key.Key {
					case ui.KeyEscape:
						draft.text = strconv.FormatInt(draft.value, 10)
						draft.invalid, draft.failure = false, nil
					case ui.KeyEnter:
						draft.text = key.Text()
						if !cells.commit() {
							return true
						}
					default:
						return false
					}
					cells.active = nil
					if cell.Focus != nil {
						cell.Focus.Focus(c)
					}
					return true
				})
			if edit.fresh {
				cells.focus.Focus(c)
				control.SetTextSelection(0, len([]rune(draft.text)))
				edit.fresh = false
			}
			if problem := draft.problem(); problem != "" {
				control.Error(problem).Tooltip(problem)
			}
		} else {
			ui.Text(c, draft.text).
				WidthPercent(100).
				SingleLine().
				TextAlign(ui.End).
				FontFeatures("tnum")
		}
	})
	if editing {
		return control
	}
	box.Label(cell.Label).Tooltip(cell.Help)
	if problem := draft.problem(); problem != "" {
		box.Background(c.Theme().Background).TextColor(c.Theme().Text).
			Border(1, c.Theme().Danger).
			Radius(c.Theme().Radius).Error(problem).Tooltip(problem)
	} else if cell.Selected && cell.Focus != nil && cell.Focus.FocusWithin(c) && cell.editable() {
		color := c.Theme().AccentText
		box.DrawOver(func(p *ui.Painter, r ui.Rect) {
			p.Stroke(
				ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2},
				color,
				2,
				1,
			)
		})
	}
	box.OnClick(func() {
		if cell.Select != nil {
			cell.Select()
		}
		previousKey, previousTime := cells.click.key, cells.click.at
		now := c.Now()
		cells.click.key, cells.click.at, cells.click.scope = cell.Key, now, cell.Focus
		if previousKey == cell.Key && !previousTime.IsZero() &&
			now.Sub(previousTime) <= 500*time.Millisecond {
			cells.Begin(cell.Key)
		} else if cell.Focus != nil {
			cell.Focus.Focus(c)
		}
	})
	return box
}
