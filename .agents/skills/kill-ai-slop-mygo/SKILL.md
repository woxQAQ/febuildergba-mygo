---
name: kill-ai-slop-mygo
description: >-
  Find and remove generic AI visual and copy patterns from native Go applications
  built with github.com/egoist/mygo/ui. Use for MyGo de-slop requests, template-like
  desktop interfaces, excessive cards, gradients, badges, oversized spacing,
  weak data hierarchy, or generic Chinese and English UI copy. Includes a read-only
  Go scanner and native UI remediation guidance that preserves focus, keyboard
  input, accessibility, theme semantics, and immediate-mode lifetimes.
compatibility: Go 1.21 or later for the optional standard-library scanner; use the target project's pinned toolchain and MyGo version.
---

# Kill AI Slop for MyGo

Remove decoration and wording that obscure the user's task. Judge the actual
workflow: an editor needs readable values, comparison, selection and efficient
commands. A source pattern is a review candidate, never proof that a design is bad.

This adapts the web-oriented `kill-ai-slop` approach to MyGo's native `ui` package.
For a MyGo app with a web frontend, use the web skill on that frontend instead.

## Principles

- Start with the task, the selected object, its data and the next action.
- Subtract redundant surfaces, headings and explanations before adding styling.
- Use the existing theme and system font unless the product calls for another
  choice. The default MyGo theme is a valid design, not a finding by itself.
- Keep semantic colors distinct. Danger, warning, success, selection and keyboard
  focus have different jobs; “one accent” does not mean making them one color.
- Build hierarchy through alignment, grouping, weight and useful whitespace.
  Dense editors need neither landing-page headlines nor enormous spacing ratios.
- Keep actual content colors, pixel art, diagrams, syntax highlighting, palette
  swatches, record IDs, ordered instructions and measured counts intact.
- Write concrete labels and consequences in the project's supported languages.
  Keep data-loss warnings, conflict information and recovery actions visible.

## Workflow

### 1. Establish scope and evidence

Read the project's instructions, design guidance and current diff. Preserve
existing work and read-only dependencies. Infer the target from the request and
active work; default to native views, shared components, theme setup and source
translation catalogs. Clarify only when choosing between unrelated apps would
materially change the work.

Read `go.mod`; locate the pinned source in the project's development environment:

```sh
go list -m -f '{{.Dir}}' github.com/egoist/mygo
```

Read that module's `ui/doc.go`, `ui/theme.go` and the relevant APIs before using
examples from [fixes.md](references/fixes.md). They were checked against MyGo
`v0.7.3`; do not upgrade MyGo to make a styling example compile. Apply the
`mygo-maintenance` skill when available for lifetime and input changes.

Identify the affected workflow, minimum supported window size, zoom settings,
appearances and languages. Use existing screenshots if relevant; capture a small
before/after comparison only when needed and permitted by project instructions.

### 2. Scan and inspect

Resolve bundled paths relative to this skill directory. The scanner uses Go's
standard library, does not load the target's dependencies and never edits files.
Run it with the project's toolchain; no Node or additional package is required:

```sh
go run /path/to/kill-ai-slop-mygo/scripts/scan.go -root .
go run /path/to/kill-ai-slop-mygo/scripts/scan.go -root . -json
go run /path/to/kill-ai-slop-mygo/scripts/scan.go -root . -only M01,M10,M11
go run /path/to/kill-ai-slop-mygo/scripts/scan.go -root . -exclude internal/generated
```

Read [detection.md](references/detection.md) for scope, rule thresholds, filters,
suppression and blind spots. Check scan errors before interpreting an empty
report. Open every relevant hit, follow shared helpers and catalog consumers,
and read [taxonomy.md](references/taxonomy.md) for manual review areas the scanner
cannot judge. Never equate fewer hits with a better interface.

### 3. Report confirmed problems

Before editing, give a concise grouped report with source locations, the effect
on the workflow and the proposed change. Separate confirmed problems from
intentional choices and unresolved observations. For example:

```text
change  internal/pages/library.go:84  nested panels squeeze the value columns → one region, aligned rows
change  internal/locales/en.json:31   “Unlock your potential” hides the action → “Open a project”
keep    internal/pages/palette.go:52  gradient displays the selected color ramp
review  internal/pages/search.go:92   clickable label → check Tab, Enter and accessible name
```

If the user asked for fixes, this report is a progress update: continue with
small, authorized changes. If they requested only an audit, stop after the report.
Ask only for a consequential product choice that cannot be inferred, such as
replacing an established visual identity. Do not impose a separate approval step
for routine cleanup already requested.

### 4. Fix at the right owner

Prefer existing theme roles and shared components over per-page overrides. Keep
page-specific grouping local; do not invent a generic design-system framework.
Use [fixes.md](references/fixes.md) for native patterns and behavioral checks.

- Preserve selection, input drafts, stable keys, focus order, shortcuts, disabled
  states, scrolling, virtualization and accessible names when flattening views.
- Keep `ui.Element` and custom parts within their build pass. Store data, widget
  state and `ui.Handle`; use `c.Services()` for persistent service callbacks.
- Set `c.Key` before building controls; set container keys before children and
  local state. Keep list item identity independent of row position.
- Prefer widgets and base widgets to hand-drawn clickable labels or boxes.
  Do not remove focus/selection feedback just to make a surface quieter.
- Keep effects in handled actions, painting in drawing callbacks, and slow work
  off the UI thread. Return background results through `Window.Update` with the
  project's stale-result checks. A visual cleanup must not change data ownership.
- Edit source catalogs and regenerate through project commands; never patch
  generated translations. Preserve placeholders, plural rules and precise domain
  terms. Explanations may move to contextual help only when safe discovery remains.

### 5. Verify and finish

Re-run the same scoped scan and review the diff. Explain remaining intentional
hits rather than chasing zero or adding suppressions to improve a count.

Use project validation rules. Pure spacing or color changes do not justify new
permanent tests. When control composition or behavior changes, check the affected
input/focus workflow and run relevant existing checks, including MyGo vet where
available. Do not add permanent UI tests if the project forbids them.

For visual changes, inspect the affected view where practical: normal and minimum
window widths, supported zoom/appearance/languages, empty and populated states,
selected/focused/disabled controls, errors and loading. Include only applicable
states. A source scan cannot verify clipping, contrast, keyboard behavior or a
spinner's motion; state explicitly when native visual checks were not performed.

Finish with what changed and why, checks actually performed, and any material
remaining uncertainty. Keep temporary screenshots and one-off reports out of the
repository unless the project requests them. Never run broad staging commands or
reformat unrelated files.

## Bundled resources

- [Taxonomy](references/taxonomy.md): desktop-specific tells and legitimate uses.
- [Detection](references/detection.md): automated evidence and manual coverage.
- [Fixes](references/fixes.md): MyGo patterns and preservation checks.
- [Scanner](scripts/scan.go): dependency-free, read-only CLI.
- [CLI workflow tests](scripts/scan_test.go): temporary fixtures; no native UI.

In this repository, use the `devenv` environment. From its root:

```sh
devenv shell -- go run .agents/skills/kill-ai-slop-mygo/scripts/scan.go -root .
devenv shell -- go test ./.agents/skills/kill-ai-slop-mygo/scripts
```

Pi discovers this project skill under `.agents/skills`. Reload skills with
`/reload`, then invoke `/skill:kill-ai-slop-mygo` followed by the target workflow.
