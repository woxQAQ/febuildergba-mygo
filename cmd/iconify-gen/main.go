// Command iconify-gen converts the @iconify-json/* icon sets installed
// by pnpm into Go source under internal/components/icons: one file per
// set, exposing cached icon methods as icons.<Set>.<Icon>().
// Run it through `just icons` after changing the icon dependencies;
// commit the generated files.
package main

import (
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const iconsDir = "internal/components/icons"

// setDoc mirrors the Iconify icons.json format. Name-only aliases inherit
// their parent's geometry. Entries using transforms or alias overrides are
// rejected until needed; hidden entries are skipped.
type setDoc struct {
	Prefix string `json:"prefix"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Icons  map[string]struct {
		Body   string `json:"body"`
		Left   int    `json:"left"`
		Top    int    `json:"top"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
		Rotate int    `json:"rotate"`
		HFlip  bool   `json:"hFlip"`
		VFlip  bool   `json:"vFlip"`
		Hidden bool   `json:"hidden"`
	} `json:"icons"`
	Aliases map[string]json.RawMessage `json:"aliases"`
}

func main() {
	sets, err := iconifySets("package.json")
	if err != nil {
		fatal(err)
	}
	if len(sets) == 0 {
		fatal(fmt.Errorf("package.json declares no @iconify-json/* dependency"))
	}
	for _, set := range sets {
		data, err := os.ReadFile(
			filepath.Join("node_modules", "@iconify-json", set, "icons.json"),
		)
		if err != nil {
			fatal(
				fmt.Errorf(
					"read set %s (run pnpm install first): %w",
					set,
					err,
				),
			)
		}
		version, license := packageInfo(set)
		source, err := generate(set, version, license, data)
		if err != nil {
			fatal(err)
		}
		target := filepath.Join(iconsDir, set+"_gen.go")
		if err := os.MkdirAll(iconsDir, 0o755); err != nil {
			fatal(err)
		}
		if err := os.WriteFile(target, source, 0o600); err != nil {
			fatal(err)
		}
		fmt.Printf("%s -> %s\n", set, target)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "iconify-gen:", err)
	os.Exit(1)
}

// iconifySets returns the set names declared as @iconify-json/*
// dependencies in the given package.json.
func iconifySets(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	seen := map[string]bool{}
	var sets []string
	for _, deps := range []map[string]string{manifest.Dependencies, manifest.DevDependencies} {
		for name := range deps {
			set, ok := strings.CutPrefix(name, "@iconify-json/")
			if ok && !seen[set] {
				seen[set] = true
				sets = append(sets, set)
			}
		}
	}
	sort.Strings(sets)
	return sets, nil
}

// packageInfo reads the installed set's version and license for the
// generated file header.
func packageInfo(set string) (version, license string) {
	data, err := os.ReadFile(
		filepath.Join("node_modules", "@iconify-json", set, "package.json"),
	)
	if err != nil {
		return "unknown", "unknown license"
	}
	var manifest struct {
		Version string `json:"version"`
		License string `json:"license"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "unknown", "unknown license"
	}
	return manifest.Version, manifest.License
}

// generate renders one Go file with a zero-size namespace and independent
// caches, so unused icons, synchronization state and SVG strings can be linked out.
func generate(set, version, license string, data []byte) ([]byte, error) {
	var doc setDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse set %s: %w", set, err)
	}
	// Resolve name-only aliases before generating methods, including alias chains.
	parents := make(map[string]string, len(doc.Aliases))
	for name, data := range doc.Aliases {
		var properties map[string]json.RawMessage
		if err := json.Unmarshal(data, &properties); err != nil {
			return nil, fmt.Errorf("set %s alias %q: %w", set, name, err)
		}
		var parent string
		if len(properties) != 1 ||
			json.Unmarshal(properties["parent"], &parent) != nil ||
			parent == "" {
			return nil, fmt.Errorf(
				"set %s alias %q: only name-only aliases are supported",
				set,
				name,
			)
		}
		if _, exists := doc.Icons[name]; exists {
			return nil, fmt.Errorf(
				"set %s alias %q duplicates an icon",
				set,
				name,
			)
		}
		parents[name] = parent
	}
	for len(parents) > 0 {
		resolved := false
		for name, parent := range parents {
			if icon, exists := doc.Icons[parent]; exists {
				doc.Icons[name] = icon
				delete(parents, name)
				resolved = true
			}
		}
		if !resolved {
			return nil, fmt.Errorf(
				"set %s has aliases with missing or cyclic parents",
				set,
			)
		}
	}
	if doc.Prefix == "" {
		doc.Prefix = set
	}
	if doc.Width == 0 {
		doc.Width = 24
	}
	if doc.Height == 0 {
		doc.Height = 24
	}

	var b strings.Builder
	fmt.Fprintf(
		&b,
		"// Code generated by cmd/iconify-gen from @iconify-json/%s@%s (%s); DO NOT EDIT.\n\npackage icons\n\nimport \"github.com/egoist/mygo/ui\"\n",
		set,
		version,
		license,
	)

	names := make([]string, 0, len(doc.Icons))
	for name, icon := range doc.Icons {
		if icon.Hidden {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	type field struct {
		name, body               string
		left, top, width, height int
	}
	var fields []field
	identifiers := make(map[string]int, len(names))
	for _, name := range names {
		icon := doc.Icons[name]
		if icon.Rotate != 0 || icon.HFlip || icon.VFlip {
			return nil, fmt.Errorf(
				"set %s icon %q uses rotate/flip, which we do not transform yet",
				set,
				name,
			)
		}
		if strings.ContainsRune(icon.Body, '`') {
			return nil, fmt.Errorf(
				"set %s icon %q has a backtick in its body",
				set,
				name,
			)
		}
		width, height := icon.Width, icon.Height
		if width == 0 {
			width = doc.Width
		}
		if height == 0 {
			height = doc.Height
		}
		entry := field{
			name:   name,
			body:   icon.Body,
			left:   icon.Left,
			top:    icon.Top,
			width:  width,
			height: height,
		}
		identifier := pascal(name)
		if index, exists := identifiers[identifier]; exists {
			previous := fields[index]
			previous.name = name
			if previous != entry {
				return nil, fmt.Errorf(
					"set %s icons %q and %q share Go name %s",
					set,
					fields[index].name,
					name,
					identifier,
				)
			}
			continue
		}
		identifiers[identifier] = len(fields)
		fields = append(fields, entry)
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("set %s has no visible icons", set)
	}

	setName := pascal(doc.Prefix)
	privateName := strings.ToLower(setName[:1]) + setName[1:]
	fmt.Fprintf(
		&b,
		"\n// %s provides lazily parsed icons from the %s set.\nvar %s %sSet\n\ntype %sSet struct{}\n",
		setName,
		doc.Prefix,
		setName,
		privateName,
		privateName,
	)
	for _, f := range fields {
		name := pascal(f.name)
		fmt.Fprintf(
			&b,
			"\nvar %s%s cachedIcon\n\n// %s returns the cached %q icon of the %s set.\nfunc (%sSet) %s() *ui.SVG {\n\t%s%s.once.Do(func() {\n\t\t%s%s.svg = ui.MustParseSVG([]byte(`<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"%d %d %d %d\">%s</svg>`))\n\t})\n\treturn %s%s.svg\n}\n",
			privateName,
			name,
			name,
			f.name,
			doc.Prefix,
			privateName,
			name,
			privateName,
			name,
			privateName,
			name,
			f.left,
			f.top,
			f.width,
			f.height,
			f.body,
			privateName,
			name,
		)
	}

	source, err := format.Source([]byte(b.String()))
	if err != nil {
		return nil, fmt.Errorf("format generated set %s: %w", set, err)
	}
	return source, nil
}

// pascal converts an Iconify kebab-case name into an exported Go
// identifier.
func pascal(name string) string {
	var b strings.Builder
	for _, part := range strings.FieldsFunc(name, func(r rune) bool {
		return r == '-' || r == '_'
	}) {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		b.WriteString(part[1:])
	}
	result := b.String()
	if result == "" || result[0] >= '0' && result[0] <= '9' {
		return "Icon" + result
	}
	return result
}
