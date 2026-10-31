// Command scan reports review candidates in MyGo source and translation catalogs.
// It uses only the standard library and never changes scanned files.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type rule struct {
	id, name, fix string
}

var rules = []rule{
	{
		"M01",
		"decorative gradient or blur",
		"check its information value; prefer theme surfaces",
	},
	{
		"M02",
		"inline chrome color",
		"reuse a theme role unless this is actual content color",
	},
	{
		"M03",
		"large corner radius",
		"compare with control size and the existing radius scale",
	},
	{
		"M04",
		"diffuse shadow",
		"keep elevation for overlays; simplify ordinary regions",
	},
	{
		"M05",
		"display-size text",
		"keep the current task and data visible at the minimum window size",
	},
	{
		"M06",
		"display font in controls",
		"prefer the existing UI font; preserve content typography",
	},
	{
		"M07",
		"thick left-border callout",
		"put the message and action beside the affected control",
	},
	{"M08", "badge", "keep only a meaningful state or count; check repetition"},
	{
		"M09",
		"large padding or gap",
		"group by relationship without wasting editing space",
	},
	{
		"M10",
		"generic promotional copy",
		"name the action, object and consequence",
	},
	{
		"M11",
		"decorative emoji candidate",
		"keep only meaningful content or a necessary signal",
	},
	{
		"M12",
		"clickable primitive",
		"check keyboard and accessibility; prefer a widget or base widget",
	},
}

var (
	promotional = regexp.MustCompile(
		`(?i)\b(not just .{1,60}it['’]?s|say goodbye to|meet your new|supercharge|unlock the power of|blazing[- ]fast|effortless(?:ly)?|seamless(?:ly)?|game[- ]?changer|next[- ]level)\b|不仅.{1,30}(?:更是|而是)|告别.{1,24}(?:迎接|拥抱)|(?:释放|解锁).{0,12}潜能|赋能|重塑.{0,12}体验|前所未有的|无缝体验`,
	)
	emoji       = regexp.MustCompile(`[\x{1F300}-\x{1FAFF}\x{2600}-\x{27BF}]`)
	displayFont = regexp.MustCompile(
		`(?i)\b(playfair|cormorant|lora|georgia|dm serif|libre baskerville)\b`,
	)
	jsonString = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
)

// A finding is evidence to inspect, not a design verdict.
type finding struct {
	Rule    string `json:"rule"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Snippet string `json:"snippet"`
}

type report struct {
	Files    int       `json:"files"`
	Findings []finding `json:"findings"`
	Errors   []string  `json:"errors"`
}

type paths []string

func (p *paths) String() string { return strings.Join(*p, ",") }
func (p *paths) Set(value string) error {
	clean := filepath.ToSlash(filepath.Clean(value))
	if clean == "." || filepath.IsAbs(value) || clean == ".." ||
		strings.HasPrefix(clean, "../") {
		return fmt.Errorf(
			"exclude must be a relative file or directory within root: %q",
			value,
		)
	}
	*p = append(*p, clean)
	return nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("scan", flag.ContinueOnError)
	flags.SetOutput(errOut)
	root := flags.String("root", ".", "project directory")
	asJSON := flags.Bool(
		"json",
		false,
		"emit JSON with findings and scan errors",
	)
	only := flags.String("only", "", "comma-separated rule IDs, e.g. M01,M10")
	skip := flags.String("skip", "", "comma-separated rule IDs to omit")
	var excludes paths
	flags.Var(
		&excludes,
		"exclude",
		"relative file/directory to skip (repeatable; no globs)",
	)
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintln(
			errOut,
			"use -root for the project directory; positional arguments are not supported",
		)
		return 2
	}
	active := make(map[string]bool)
	for _, r := range rules {
		active[r.id] = *only == ""
	}
	for _, filter := range []struct {
		value string
		on    bool
	}{{*only, true}, {*skip, false}} {
		if filter.value == "" {
			continue
		}
		for _, id := range strings.Split(filter.value, ",") {
			id = strings.TrimSpace(id)
			if _, ok := active[id]; !ok {
				_, _ = fmt.Fprintf(errOut, "unknown rule ID %q\n", id)
				return 2
			}
			active[id] = filter.on
		}
	}

	result := report{Findings: []finding{}, Errors: []string{}}
	rootPath, err := filepath.Abs(*root)
	if err == nil {
		var info os.FileInfo
		info, err = os.Lstat(rootPath)
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			err = fmt.Errorf("root must be a directory, not a file or symlink")
		}
	}
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
	} else {
		err = filepath.WalkDir(
			rootPath,
			func(path string, entry fs.DirEntry, walkErr error) error {
				rel, relErr := filepath.Rel(rootPath, path)
				if relErr != nil {
					return relErr
				}
				rel = filepath.ToSlash(rel)
				if walkErr != nil {
					result.Errors = append(
						result.Errors,
						fmt.Sprintf("%s: %v", rel, walkErr),
					)
					return nil
				}
				if entry.Type()&os.ModeSymlink != 0 {
					return nil
				}
				if rel != "." && excluded(rel, entry, excludes) {
					if entry.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if entry.IsDir() {
					if rel != "." {
						// Nested repositories and submodules have their own ownership.
						if _, statErr := os.Lstat(
							filepath.Join(path, ".git"),
						); statErr == nil {
							return filepath.SkipDir
						} else if !os.IsNotExist(
							statErr,
						) {
							result.Errors = append(
								result.Errors,
								fmt.Sprintf("%s: %v", rel, statErr),
							)
							return filepath.SkipDir
						}
					}
					return nil
				}
				if !entry.Type().IsRegular() {
					return nil
				}
				isGo := strings.HasSuffix(rel, ".go") &&
					!strings.HasSuffix(rel, "_test.go")
				isCatalog := strings.HasSuffix(rel, ".json") &&
					catalogPath(rel, rootPath)
				if !isGo && !isCatalog {
					return nil
				}
				data, readErr := os.ReadFile(path)
				if readErr != nil {
					result.Errors = append(
						result.Errors,
						fmt.Sprintf("%s: %v", rel, readErr),
					)
					return nil
				}
				var hits []finding
				var scanned bool
				var scanErr error
				if isGo {
					hits, scanned, scanErr = scanGo(rel, data)
				} else {
					hits, scanErr = scanCatalog(rel, data)
					scanned = true
				}
				if scanned {
					result.Files++
				}
				if scanErr != nil {
					result.Errors = append(
						result.Errors,
						fmt.Sprintf("%s: %v", rel, scanErr),
					)
				}
				for _, hit := range hits {
					if active[hit.Rule] {
						result.Findings = append(result.Findings, hit)
					}
				}
				return nil
			},
		)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
		}
	}
	sort.Slice(result.Findings, func(i, j int) bool {
		a, b := result.Findings[i], result.Findings[j]
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Column < b.Column
	})
	if *asJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			_, _ = fmt.Fprintln(errOut, err)
			return 1
		}
	} else {
		var lines []string
		for _, r := range rules {
			shown := false
			for _, hit := range result.Findings {
				if hit.Rule != r.id {
					continue
				}
				if !shown {
					lines = append(
						lines,
						fmt.Sprintf("%s %s → %s", r.id, r.name, r.fix),
					)
					shown = true
				}
				lines = append(lines, fmt.Sprintf(
					"  %s:%d:%d  %s",
					safeText(
						hit.File,
					),
					hit.Line,
					hit.Column,
					safeText(hit.Snippet),
				))
			}
		}
		lines = append(lines, fmt.Sprintf(
			"%d files scanned; %d candidates; %d errors. Candidates require review.",
			result.Files,
			len(result.Findings),
			len(result.Errors),
		))
		if _, err := io.WriteString(
			out,
			strings.Join(lines, "\n")+"\n",
		); err != nil {
			_, _ = fmt.Fprintln(errOut, err)
			return 1
		}
		for _, message := range result.Errors {
			_, _ = fmt.Fprintln(errOut, safeText(message))
		}
	}
	if len(result.Errors) != 0 {
		return 1
	}
	return 0
}

func excluded(rel string, entry fs.DirEntry, excludes paths) bool {
	for _, prefix := range excludes {
		if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
			return true
		}
	}
	if !entry.IsDir() {
		return false
	}
	if strings.HasPrefix(entry.Name(), ".") {
		return true
	}
	switch entry.Name() {
	case "vendor",
		"node_modules",
		"build",
		"dist",
		"out",
		"coverage",
		"testdata",
		"FEBuilderGBA":
		return true
	}
	return false
}

func catalogPath(rel, root string) bool {
	for _, part := range strings.Split(filepath.Base(root)+"/"+filepath.ToSlash(filepath.Dir(rel)), "/") {
		switch part {
		case "locales", "i18n", "translations":
			return true
		}
	}
	return false
}

func scanGo(path string, data []byte) ([]finding, bool, error) {
	positions := token.NewFileSet()
	file, err := parser.ParseFile(
		positions,
		path,
		data,
		parser.ParseComments|parser.SkipObjectResolution,
	)
	if err != nil {
		return nil, false, err
	}
	if ast.IsGenerated(file) {
		return nil, false, nil
	}
	alias := ""
	for _, spec := range file.Imports {
		importPath, _ := strconv.Unquote(spec.Path.Value)
		if importPath == "github.com/egoist/mygo/ui" {
			alias = "ui"
			if spec.Name != nil {
				alias = spec.Name.Name
			}
		}
	}
	if alias == "" || alias == "_" {
		return nil, false, nil
	}
	if alias == "." {
		return nil, false, fmt.Errorf(
			"dot import of MyGo ui is not supported; inspect this file manually",
		)
	}
	var hits []finding
	lines := strings.Split(string(data), "\n")
	add := func(id string, pos token.Pos) {
		location := positions.PositionFor(pos, false)
		if suppressed(file, positions, id, location.Line) {
			return
		}
		hits = append(
			hits,
			finding{
				id,
				path,
				location.Line,
				location.Column,
				strings.TrimSpace(lines[location.Line-1]),
			},
		)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		root := uiRoot(call, alias)
		if root == "" {
			return true
		}
		method := selector.Sel.Name
		mark := func(id string) { add(id, selector.Sel.Pos()) }
		switch method {
		case "Gradient",
			"LinearGradient",
			"ConicGradient",
			"BorderLinearGradient",
			"BorderConicGradient":
			mark("M01")
		case "Blur":
			if numberAt(call.Args, 0, 0.01) {
				mark("M01")
			}
		case "Background", "TextColor", "BorderColor", "Border":
			for _, arg := range call.Args {
				if color, ok := arg.(*ast.CallExpr); ok {
					name := uiRoot(color, alias)
					if name == "Hex" || name == "RGB" || name == "RGBA" {
						mark("M02")
						break
					}
				}
			}
		case "Radius":
			for i := range call.Args {
				if numberAt(call.Args, i, 12) {
					mark("M03")
					break
				}
			}
		case "Shadow", "InsetShadow":
			if numberAt(call.Args, 2, 16) || numberAt(call.Args, 3, 4) {
				mark("M04")
			}
		case "FontSize":
			if numberAt(call.Args, 0, 28) {
				mark("M05")
			}
		case "Font":
			if len(call.Args) > 0 &&
				displayFont.MatchString(stringValue(call.Args[0])) {
				mark("M06")
			}
		case "BorderWidth":
			if len(call.Args) == 4 && numberAt(call.Args, 3, 3) {
				mark("M07")
			}
		case "Badge":
			if root == "Badge" {
				mark("M08")
			}
		case "Padding",
			"PaddingX",
			"PaddingY",
			"PaddingStart",
			"PaddingEnd",
			"Gap":
			for i := range call.Args {
				if numberAt(call.Args, i, 24) {
					mark("M09")
					break
				}
			}
		case "OnClick":
			if root == "Text" || root == "Textf" || root == "Box" {
				mark("M12")
			}
		}
		// Inspect literal arguments in UI calls, not comments, imports or logs.
		for _, arg := range call.Args {
			value := stringValue(arg)
			if value == "" {
				continue
			}
			for _, id := range copyRules(value) {
				add(id, arg.Pos())
			}
		}
		return true
	})
	return hits, true, nil
}

// uiRoot follows fluent receivers only; it does not infer types or chase variables.
func uiRoot(call *ast.CallExpr, alias string) string {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	if name, ok := selector.X.(*ast.Ident); ok && name.Name == alias {
		return selector.Sel.Name
	}
	if receiver, ok := selector.X.(*ast.CallExpr); ok {
		return uiRoot(receiver, alias)
	}
	return ""
}

func numberAt(args []ast.Expr, index int, threshold float64) bool {
	if index >= len(args) {
		return false
	}
	literal, ok := args[index].(*ast.BasicLit)
	if !ok || (literal.Kind != token.INT && literal.Kind != token.FLOAT) {
		return false
	}
	value, err := strconv.ParseFloat(
		strings.ReplaceAll(literal.Value, "_", ""),
		64,
	)
	return err == nil && value >= threshold
}

func stringValue(expr ast.Expr) string {
	literal, ok := expr.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return ""
	}
	value, _ := strconv.Unquote(literal.Value)
	return value
}

func copyRules(value string) []string {
	var ids []string
	if promotional.MatchString(value) {
		ids = append(ids, "M10")
	}
	if emoji.MatchString(value) {
		ids = append(ids, "M11")
	}
	return ids
}

func suppressed(
	file *ast.File,
	positions *token.FileSet,
	id string,
	line int,
) bool {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if !strings.HasPrefix(comment.Text, "//") {
				continue
			}
			fields := strings.Fields(
				strings.TrimSpace(strings.TrimPrefix(comment.Text, "//")),
			)
			if len(fields) < 2 {
				continue
			}
			commentLine := positions.PositionFor(comment.Pos(), false).Line
			applies := fields[0] == "deslop-ignore-file" ||
				(fields[0] == "deslop-ignore" && commentLine == line) ||
				(fields[0] == "deslop-ignore-next-line" && commentLine+1 == line)
			if !applies {
				continue
			}
			for _, field := range fields[1:] {
				if field == "--" {
					break
				}
				if field == id {
					return true
				}
			}
		}
	}
	return false
}

func scanCatalog(path string, data []byte) ([]finding, error) {
	if !json.Valid(data) {
		return nil, fmt.Errorf("invalid JSON catalog")
	}
	var hits []finding
	positions := token.NewFileSet().AddFile(path, -1, len(data))
	positions.SetLinesForContent(data)
	for _, match := range jsonString.FindAllIndex(data, -1) {
		rest := strings.TrimLeft(string(data[match[1]:]), " \t\r\n")
		if strings.HasPrefix(rest, ":") {
			continue // Catalog keys are identifiers, not displayed copy.
		}
		var value string
		if err := json.Unmarshal(data[match[0]:match[1]], &value); err != nil {
			return nil, err
		}
		location := positions.PositionFor(positions.Pos(match[0]), false)
		for _, id := range copyRules(value) {
			hits = append(
				hits,
				finding{id, path, location.Line, location.Column, value},
			)
		}
	}
	return hits, nil
}

func safeText(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return ' '
		}
		return r
	}, value)
}
