// Command legacy-inventory records entry points in the pinned legacy checkout.
// The presence of a form or source file does not imply behavior coverage.
package main

import (
	"encoding/csv"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type project struct {
	Groups []struct {
		Compile []struct {
			Include string `xml:"Include,attr"`
			SubType string `xml:"SubType"`
			Parent  string `xml:"DependentUpon"`
		} `xml:"Compile"`
	} `xml:"ItemGroup"`
}

type entry struct {
	id, kind, path, title, callers string
	line                           int
}

func main() {
	root := flag.String("root", "../FEBuilderGBA/FEBuilderGBA", "legacy C# project directory")
	out := flag.String("out", "../.cache/legacy-inventory.csv", "generated source inventory")
	flag.Parse()
	if err := run(*root, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(directory, out string) (err error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	var entries []entry
	for _, collect := range []func(*os.Root) ([]entry, error){sourceEntries, commandEntries, configEntries} {
		items, collectErr := collect(root)
		if collectErr != nil {
			return collectErr
		}
		entries = append(entries, items...)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].id < entries[j].id })
	if err = writeInventory(out, entries); err != nil {
		return err
	}
	printCounts(out, entries)
	return nil
}

func sourceEntries(root *os.Root) ([]entry, error) {
	content, err := root.ReadFile("FEBuilderGBA.csproj")
	if err != nil {
		return nil, err
	}
	var manifest project
	if err = xml.Unmarshal(content, &manifest); err != nil {
		return nil, err
	}
	titlePattern := regexp.MustCompile(`this\.Text\s*=\s*"([^"\r\n]*)"`)
	jumpPattern := regexp.MustCompile(`(?:JumpForm|JumpFormLow|JumpFormSafety)<(\w+)>|new\s+(\w+Form)\s*\(`)
	callers := map[string][]string{}
	var entries []entry
	for _, group := range manifest.Groups {
		for _, source := range group.Compile {
			path := strings.ReplaceAll(source.Include, "\\", "/")
			if source.Parent != "" || strings.HasSuffix(path, ".Designer.cs") {
				continue
			}
			body, readErr := root.ReadFile(filepath.FromSlash(path))
			if readErr != nil {
				return nil, readErr
			}
			kind := "source"
			switch source.SubType {
			case "Form":
				kind = "form"
			case "UserControl":
				kind = "control"
			}
			title := strings.TrimSuffix(filepath.Base(path), ".cs")
			if designer, readErr := root.ReadFile(strings.TrimSuffix(path, ".cs") + ".Designer.cs"); readErr == nil {
				if match := titlePattern.FindSubmatch(designer); match != nil {
					title = string(match[1])
				}
			}
			entries = append(entries, entry{id: "source:" + path, kind: kind, path: path, line: 1, title: title})
			for _, match := range jumpPattern.FindAllSubmatchIndex(body, -1) {
				start, end := match[2], match[3]
				if start < 0 {
					start, end = match[4], match[5]
				}
				callee := string(body[start:end])
				line := 1 + strings.Count(string(body[:match[0]]), "\n")
				callers[callee] = append(callers[callee], fmt.Sprintf("%s:%d", path, line))
			}
		}
	}
	for i := range entries {
		name := strings.TrimSuffix(filepath.Base(entries[i].path), ".cs")
		entries[i].callers = strings.Join(callers[name], "; ")
	}
	return entries, nil
}

func commandEntries(root *os.Root) ([]entry, error) {
	program, err := root.ReadFile("Program.cs")
	if err != nil {
		return nil, err
	}
	commandPattern := regexp.MustCompile(`ArgsDic\.ContainsKey\("(--[^"\r\n]+)"\)`)
	seen := make(map[string]int)
	var entries []entry
	for _, match := range commandPattern.FindAllSubmatchIndex(program, -1) {
		name := string(program[match[2]:match[3]])
		line := 1 + strings.Count(string(program[:match[0]]), "\n")
		if index, ok := seen[name]; ok {
			entries[index].callers += fmt.Sprintf("; Program.cs:%d", line)
			continue
		}
		seen[name] = len(entries)
		entries = append(entries, entry{id: "cli:" + name, kind: "command", path: "Program.cs", line: line, title: name, callers: fmt.Sprintf("Program.cs:%d", line)})
	}
	return entries, nil
}

func configEntries(root *os.Root) ([]entry, error) {
	var entries []entry
	err := fs.WalkDir(root.FS(), "bin/Debug/config", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".txt") {
			return nil
		}
		kind := "config"
		if strings.HasPrefix(d.Name(), "PATCH_") {
			kind = "patch-rule"
		}
		entries = append(entries, entry{id: kind + ":" + path, kind: kind, path: path, line: 1, title: d.Name()})
		return nil
	})
	return entries, err
}

func writeInventory(path string, entries []entry) (err error) {
	if err = os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	f, err := root.Create(filepath.Base(path))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	w := csv.NewWriter(f)
	if err = w.Write([]string{"id", "kind", "source", "line", "legacy_title", "static_callers", "assessment"}); err != nil {
		return err
	}
	for _, item := range entries {
		if err = w.Write([]string{item.id, item.kind, "../FEBuilderGBA/FEBuilderGBA/" + item.path, strconv.Itoa(item.line), item.title, item.callers, "unassessed"}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func printCounts(out string, entries []entry) {
	counts := map[string]int{}
	for _, item := range entries {
		counts[item.kind]++
	}
	fmt.Printf("%s: %d entries\n", out, len(entries))
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Printf("  %s: %d\n", key, counts[key])
	}
}
