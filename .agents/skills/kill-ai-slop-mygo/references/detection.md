# Scanner contract and limits

Run `scripts/scan.go` with Go 1.21+ and the target project's toolchain. The scanner
uses the standard library and reads files directly; it does not type-check,
compile, execute or modify the target app, fetch modules, or launch a window.
Go itself may use its normal build cache. Respect the project's toolchain policy;
in this repository run inside `devenv` with `GOTOOLCHAIN=local`.

## Inputs and output

```sh
go run /path/to/kill-ai-slop-mygo/scripts/scan.go -root /path/to/app
go run /path/to/kill-ai-slop-mygo/scripts/scan.go -root . -json -only M01,M03,M10
go run /path/to/kill-ai-slop-mygo/scripts/scan.go -root . -skip M08 -exclude internal/vendor-ui -exclude assets
```

- `-root` takes one directory, defaulting to the working directory. All flags may
  use one or two leading dashes. Positional arguments are rejected.
- `-only` and `-skip` take comma-separated, case-sensitive IDs; unknown IDs fail.
  `-skip` wins when an ID occurs in both.
- Repeat `-exclude` for root-relative files or directory prefixes. Matching uses
  path boundaries, not substrings or globs. Absolute and parent paths fail.
- Human output groups candidates by rule, then sorts by path, physical line and
  byte column. JSON contains `files`, `findings` and `errors`; each finding has
  `rule`, `file`, `line`, `column` and `snippet`. Paths are relative to `-root`.
- `files` counts parsed MyGo-importing files and eligible catalogs. It is not a
  count of all files visited. Empty output is not proof that the app is clean.
- The executable exits 0 when scanning completes, even with candidates; 1 for
  scan/read/output errors; 2 for invalid flags or IDs. `go run` may wrap a nonzero
  program status as its own exit code. JSON still contains scan errors and any
  successfully collected findings. Flag errors go to stderr before scanning.

## Scope

The scanner parses non-test `.go` files and inspects those importing
`github.com/egoist/mygo/ui`. It understands normal and aliased imports, and follows
fluent call chains across line breaks. Comments and import strings are not copy.
A dot import produces an explicit unsupported-file error.

Go-generated files with the standard `// Code generated … DO NOT EDIT.` marker
are skipped. Translation values are read from `.json` files under `locales`,
`i18n` or `translations`, including when such a directory is the scan root.
Nested catalog values and escaped Unicode are supported; keys are not copy.
Invalid Go or JSON is reported, not silently treated as a clean scan.

Hidden directories, `vendor`, `node_modules`, `build`, `dist`, `out`, `coverage`,
`testdata`, `FEBuilderGBA`, nested repositories/submodules with a `.git` entry,
symlinks and non-regular files are skipped. A symlink scan root is rejected;
provide the real directory. Hidden directories are skipped when encountered
below the root; explicitly scanning one as the root remains possible.

The scanner does not read Markdown, CSS, Go test fixtures, arbitrary JSON config,
generated translation Go, catalogs in other formats or differently named
catalog directories. Inspect these manually when the requested scope includes
them. The default exclusions prevent scanning the skill's own examples when it
is installed in `.agents/skills`.

## Rule evidence

All style rules require a call chain rooted in the imported MyGo `ui` package.
Numbers below are literal DIP values (device-independent layout units), not
physical pixels. They identify outliers to inspect, not mandatory limits.

| ID  | Code-level candidate                                                                                                                                                    |
| --- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| M01 | `.Gradient`, `.LinearGradient`, `.ConicGradient`, `.BorderLinearGradient`, `.BorderConicGradient`; literal `.Blur` ≥ 0.01                                               |
| M02 | `.Background`, `.TextColor`, `.BorderColor` or `.Border` with a direct `ui.Hex`, `ui.RGB`, `ui.RGBA` color expression, including a fluent color adjustment              |
| M03 | Any literal `.Radius` argument ≥ 12                                                                                                                                     |
| M04 | `.Shadow` or `.InsetShadow` with literal blur ≥ 16 or spread ≥ 4; argument order is x, y, blur, spread, color                                                           |
| M05 | Literal `.FontSize` ≥ 28                                                                                                                                                |
| M06 | Literal `.Font` naming Playfair, Cormorant, Lora, Georgia, DM Serif or Libre Baskerville                                                                                |
| M07 | Four-argument `.BorderWidth` with the left width ≥ 3; confirm whether other edges and real selection semantics justify it                                               |
| M08 | `ui.Badge`; even one is a candidate, repetition needs manual judgment                                                                                                   |
| M09 | Any literal `.Padding`, `.PaddingX`, `.PaddingY`, `.PaddingStart`, `.PaddingEnd` or `.Gap` argument ≥ 24                                                                |
| M10 | Literal UI arguments or catalog values with selected generic English/Chinese phrases; examples include “supercharge”, “seamless”, “not just … it's”, “赋能”, “无缝体验” |
| M11 | The same copy containing symbols in U+1F300–U+1FAFF or U+2600–U+27BF; this is a broad candidate range, not complete Unicode emoji classification                        |
| M12 | `.OnClick` on a chain rooted in `ui.Text`, `ui.Textf` or `ui.Box`                                                                                                       |

No dataflow or type inference is performed. Calls through variables or wrappers,
theme-field assignments, constants/expressions for numeric sizes, `Painter`
methods, rich text APIs and dynamically composed copy can be missed. Local
shadowing of an imported package name can cause a false positive. The font list
is a small set of display candidates, not a complete classification. An AST
(Abstract Syntax Tree) identifies syntax, not the design's intent.

Review theme definitions and shared helpers explicitly. Follow localized string
consumers to understand whether a phrase is a button, help text, an error, or
quoted content. Use the manual sections in [taxonomy.md](taxonomy.md) to assess
nesting, contrast, hierarchy, density, motion and control behavior.

## Intentional findings

Prefer leaving an intentional candidate visible with an explanation in the
review. For durable, justified exceptions, Go source supports rule-scoped line
comments:

```go
// deslop-ignore-next-line M03 -- Circular preview of the selected brush.
ui.Box(c).Radius(24)

ui.Badge(c, version) // deslop-ignore M08 -- Actual document format version.

// deslop-ignore-file M02 -- This file renders source palette swatches.
```

IDs are space-separated; `--` starts the human explanation. No IDs means no
suppression. Only `//` comments beginning with a directive apply; strings and
block comments cannot suppress candidates. Same-line and next-line directives
refer to the physical line of the reported selector, which may differ from the
constructor line in a multiline chain. File directives apply throughout the
file. Unknown suppression IDs have no effect. JSON does not allow comments;
review its intentional hits or use a narrowly scoped CLI filter instead.

Do not add exceptions merely to produce a zero count. The remaining count is not
a measure of design quality or review completeness.

## Scanner validation

Run the CLI workflow tests explicitly; Go's `./...` omits dot directories:

```sh
go test /path/to/kill-ai-slop-mygo/scripts/scan.go /path/to/kill-ai-slop-mygo/scripts/scan_test.go
```

The tests build and invoke the actual command against temporary source trees.
They check candidate locations, aliases and multiline syntax, decoded catalog
values, exclusions, rule-scoped suppressions, invalid inputs and unchanged source
bytes. They do not test individual implementation functions or require native UI.
