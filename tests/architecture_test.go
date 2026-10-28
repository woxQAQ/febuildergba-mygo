package tests

import (
	"os/exec"
	"strings"
	"testing"
)

// Package placement is checked against the compiled import graph, including
// page-private view stacks and the desktop application's dependency boundary.
func TestUIImportBoundaries(t *testing.T) {
	command := exec.Command(
		"go",
		"list",
		"-f",
		"{{.ImportPath}} {{join .Imports \" \"}}",
		"./internal/...",
	)
	command.Dir = ".."
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("inspect package imports: %v\n%s", err, output)
	}
	const root = "github.com/woxQAQ/febuildergba-mygo/internal/"
	for line := range strings.Lines(string(output)) {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		importer := strings.TrimPrefix(fields[0], root)
		for _, dependency := range fields[1:] {
			imported := strings.TrimPrefix(dependency, root)
			if importer == "app" && strings.HasPrefix(imported, "components") {
				t.Errorf("app depends directly on %s", imported)
			}
			private := imported == "components/recordform" ||
				imported == "components/chapterview" ||
				imported == "components/mapview"
			allowed := importer == "pages" || importer == "pages/records" ||
				strings.HasPrefix(importer, "components/")
			if private && !allowed {
				t.Errorf("%s imports page-private %s", importer, imported)
			}
		}
	}
}
