package eventeditor

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

func (page *Page) instructionTable(
	c *ui.Context,
	draft *draftState,
	rows []event.ScriptInstruction,
) {
	_, height := c.Size()
	rowHeight := c.Theme().Space(9)
	viewportHeight := min(330, max(rowHeight*4, height/3))
	draft.list.Selected = &draft.selected
	draft.list.Key = func(index int) any { return rows[index].ID }
	columns := []ui.TableColumn{
		{
			ID:    "address",
			Title: locales.ScriptAddress(),
			Width: 100,
			Fixed: true,
		},
		{ID: "command", Title: locales.ScriptCommand()},
		{ID: "operands", Title: locales.ScriptOperands()},
	}
	ui.Table(c.Key("script-instructions"), &draft.list, columns, len(rows), func(index, column int) {
		row := rows[index]
		switch column {
		case 0:
			if row.Address == 0 {
				ui.Text(c, "+")
			} else {
				ui.Textf(c, "%08X", row.Address).
					Font("monospace")
			}
		case 1:
			ui.Text(c, locales.Label(string(row.Op))).
				SingleLine().
				Tooltip(locales.Label(string(row.Op)))
		case 2:
			if row.Op == event.ScriptUnknown || row.Op == event.ScriptASM {
				ui.Textf(c, "% X", row.Raw).
					SingleLine().
					Font("monospace")
			} else {
				var values []string
				for _, field := range draft.data.Fields(index) {
					value, _ := draft.data.Value(index, field.ID)
					format := "%d"
					if field.Hex && value >= 0 {
						format = "0x%X"
					}
					values = append(values, fmt.Sprintf(format, value))
				}
				ui.Text(c, strings.Join(values, " · ")).
					SingleLine().
					FontFeatures("tnum")
			}
		}
	}).
		Height(min(viewportHeight, rowHeight*float32(len(rows)+1))).
		Label(locales.ScriptInstructions())
}
