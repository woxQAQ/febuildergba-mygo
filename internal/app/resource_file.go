package app

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages"
)

func readResourceFile(
	path string,
	limit int,
	tooLarge fault.Code,
) (result []byte, err error) {
	defer func() { err = fault.Wrap(err, "app.read_resource", fault.Context{Path: path}) }()
	file, err := os.Open(
		path,
	) // #nosec G304 -- path is selected in the native import dialog.
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	content, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(content) > limit {
		return nil, tooLarge
	}
	return content, nil
}

type resourceExport struct {
	title, suffix, extension string
	pathError, romError      fault.Code
}

// Export captures paths on the UI thread. The worker encodes an immutable
// snapshot and checks existing files against every open ROM before writing.
func (m *application) exportResourceContent(
	win *mygo.Window,
	workspace *pages.Workspace,
	file resourceExport,
	encode func() ([]byte, error),
) {
	if workspace.Editor.Session == nil || workspace.Editor.Busy {
		return
	}
	path := workspace.Editor.Session.Document.State().Path
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	defaultPath := filepath.Join(
		filepath.Dir(path),
		stem+"-"+file.suffix+"."+file.extension,
	)
	var romPaths []string
	for _, workspace := range m.windows {
		if workspace.Editor.Session != nil {
			romPaths = append(
				romPaths,
				workspace.Editor.Session.Document.State().Path,
			)
		}
	}
	question, replace, cancel := locales.ReplaceQuestion(), locales.Replace(), locales.Cancel()
	background(win, workspace, "app.export_resource", func() error {
		content, err := encode()
		if err != nil {
			return err
		}
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:      win,
			Title:       file.title,
			DefaultPath: defaultPath,
			Filters: []mygo.FileFilter{
				{
					Name:       strings.ToUpper(file.extension),
					Extensions: []string{file.extension},
				},
			},
		})
		if err != nil || path == "" {
			return err
		}
		if !strings.EqualFold(filepath.Ext(path), "."+file.extension) {
			return file.pathError
		}
		existing, err := os.Stat(path)
		if err == nil {
			for _, romPath := range romPaths {
				romInfo, statErr := os.Stat(romPath)
				if statErr == nil && os.SameFile(existing, romInfo) {
					return file.romError
				}
			}
			answer, dialogErr := mygo.Dialog.Message(mygo.MessageOptions{
				Parent:       win,
				Type:         mygo.MessageQuestion,
				Message:      question,
				Detail:       path,
				Buttons:      []string{replace, cancel},
				CancelButton: 1,
			})
			if dialogErr != nil || answer.Button != 0 {
				return dialogErr
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return fault.Wrap(
			os.WriteFile(path, content, 0o600),
			"app.write_resource",
			fault.Context{Path: path},
		)
	}, nil)
}
