# Native MyGo fixes

These snippets use APIs checked in `github.com/egoist/mygo` **v0.7.3**, particularly
`ui/doc.go`, `ui/theme.go`, `ui/element.go`, `ui/api_gen.go` and `ui/widgets.go`.
Source: [MyGo v0.7.3](https://github.com/egoist/mygo/tree/v0.7.3/ui).
Use the target project's pinned source as authority. Snippets are composition
examples, not an instruction to introduce hard-coded English or a new theme.
Route product strings through the project's existing localization workflow.

## Flatten a region without losing the workflow

Before: several decorative layers around one operation.

```go
ui.Column(c).Padding(24).Radius(24).Background(ui.Hex("#eef2ff")).Children(func() {
    ui.Column(c).Padding(24).Radius(20).Background(ui.Hex("#ffffff")).Children(func() {
        ui.Text(c, title).FontSize(32)
        renderFields(c)
        renderActions(c)
    })
})
```

After: grouping comes from the relationship between the title, fields and actions.
Use the project's existing spacing scale; the values below are illustrative.

```go
t := c.Theme()
ui.Column(c).Padding(t.Space(3)).Gap(t.Space(3)).Children(func() {
    ui.Text(c, title).Bold()
    renderFields(c)
    renderActions(c)
})
```

A viewport, image preview or independently scrolling editor may still deserve
its own border. Before removing a container, check whether it owns clipping,
scroll state, a key, command scope, focus binding or local state. Move those
responsibilities deliberately. Changes in sibling position can change implicit
identity: preserve explicit keys before construction and stable list item keys.
Do not retain `ui.Element` values across rebuilds to bridge old and new layouts.

## Quiet a status region while keeping its meaning

Before: warning text, background and border all repeat the warning color.

```go
ui.Row(c).
    Background(t.Warning.Alpha(0.12)).
    Border(2, t.Warning).
    Radius(16).
    Children(func() {
        ui.Text(c, message).TextColor(t.Warning)
    })
```

After: neutral region, explicit status, readable consequence and a recovery action.

```go
ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).
    Background(t.Background).
    BorderWidth(0, 0, 1, 0).BorderColor(t.Border).
    Children(func() {
        ui.Text(c, warningTitle).Bold().TextColor(t.Warning)
        ui.Text(c, message).Grow(1)
        ui.Button(c, reloadLabel).Disabled(busy).OnClick(reload)
    })
```

Check whether this row needs wrapping at the supported minimum width. Preserve
`Danger`, `Warning`, `Success`, `Selection`, `Focus` and their textual cues.
Do not substitute the product accent for every state. A copied theme should
retain its appearance-specific base; hard-coded white and black often break dark
mode. Use `t.Surface` for control faces and `t.Background` for the window where
those roles match the product, rather than making a colored card for every row.

## Keep compared values in aligned columns

```go
ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
    ui.Text(c, fieldLabel).Width(labelWidth)
    ui.Textf(c, "%d", value).
        Width(valueWidth).TextAlign(ui.End).FontFeatures("tnum")
})
```

Use the same column widths or the existing table/grid across compared rows.
`tnum` is the OpenType feature for equal-width digits; font support determines
whether it takes effect. Keep addresses and raw bytes in `Font("monospace")`.
Keep normal labels in the existing UI font. Do not make read-only reference values
faint or replace an aligned matrix with a collection of rounded stat cards.

Use existing localized formatting for units and quantities. Unknown and missing
values must remain distinguishable from zero. Keep identity, selection and list
virtualization when restructuring a data view.

## Preserve real control behavior

A clickable text label needs more than a pointer cursor:

```go
// Before: inspect keyboard and accessibility behavior.
ui.Text(c, openLabel).OnClick(open)

// After: the widget supplies the normal control behavior.
ui.Button(c, openLabel).Disabled(busy).OnClick(open)
```

If the project needs a custom appearance, inspect the pinned `ButtonBase` API and
existing components. Do not replace a working base widget with a painted box.
Preserve its accessible name, Tab order, Enter/Space activation, focus indicator,
disabled semantics and command scope. Use `ui.Handle` for persistent focus or
committed bounds; drawing callbacks should only paint.

Changing a container can also change undo/shortcut routing or which text input
owns the current draft. Exercise the affected focus transitions, not just the
mouse click. Bound values and change notices follow MyGo's rebuild model; do not
recompute editable locals every pass and lose their updates.

## Make copy describe the operation

| Generic wording                                     | Concrete wording, when accurate                                                    |
| --------------------------------------------------- | ---------------------------------------------------------------------------------- |
| “🚀 Unlock your editing potential” / “释放编辑潜能” | “Open a ROM” / “打开 ROM”                                                          |
| “Seamlessly apply your changes” / “无缝应用更改”    | “Apply preview” / “应用预览”                                                       |
| “Everything is ready!” / “一切就绪！”               | Omit an unchanging slogan; show actual job status when useful                      |
| “An error occurred” / “发生错误”                    | Name the failed operation, its cause when known, and the available recovery action |

Never invent performance claims, counts or capabilities while replacing generic
copy. Distinguish applying to an in-memory document from saving to disk. Preserve
warnings about shared references, stale previews and destructive consequences.
Do not rename a generic commit action “Save” unless it actually saves.

Edit source catalog values in all supported languages, preserve interpolation
and plural forms, then run the project's generation and translation checks.
Do not hand-edit generated Go. In this repository those commands are
`just generate` and `just i18n-check` inside `devenv`.

## Put help beside the decision

Move repeated background explanations to existing field help or a disclosure,
while retaining a short direct label. Keep current validation errors, conflict
states, incomplete verification and data-loss consequences visible at the action.
A tooltip alone is insufficient for critical information or an unexplained
unavailable command. Verify keyboard access to help and restore focus when a
section containing the focused control disappears.

## Review the result

Use a bounded check of the changed workflow: content still fits at the supported
minimum size and zoom; long translated labels remain readable; selection and
focus are distinguishable; controls retain disabled/keyboard behavior; empty,
loading, error and populated states still explain what to do. Observe actual
motion when changing an animation. Keep native verification separate from source
inspection and scanner results, and report any part that could not be exercised.
