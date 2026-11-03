package maps_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestTilesetConfigFileWorkflow(t *testing.T) {
	for _, version := range testutil.Games() {
		t.Run(version.Name, func(t *testing.T) {
			f := testutil.NewMapFixture(version.Name)
			// The file is the raw little-endian configuration, with no header.
			original := make([]byte, 0x2400)
			copy(
				original,
				[]byte{0, 0, 0, 0x14, 0, 0x28, 0, 0x3C, 1, 0, 1, 0, 1, 0, 1, 0},
			)
			original[0x2000], original[0x2001] = 1, 0x21
			copy(
				original[0x1FF8:],
				[]byte{0xFF, 0xFF, 0xEF, 0xBE, 0xAA, 0x55, 0x23, 0x81},
			)
			original[0x23FF] = 0xE7
			compressed, err := gba.CompressLZ77(original)
			if err != nil {
				t.Fatal(err)
			}
			f.WriteAt(
				f.GetPosition("map_config"),
				append(compressed, 0xDE, 0xAD, 0xBE, 0xEF),
			)
			stride := 148
			if version.Name == "FE6" {
				stride = 68
			}
			if version.Name == "FE7U" {
				stride = 152
			}
			chapter := f.GetPosition("chapters")
			f.WriteAt(chapter+2*stride, f.Bytes()[chapter:chapter+stride])
			f.WriteU8(chapter+2*stride+7, 9)
			f.WritePointer(
				f.GetPosition("map_config_list")+9*4,
				f.GetPosition("map_config"),
			)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			draft, err := s.Maps.NewTilesetDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			if got := draft.Config().Bytes(); !bytes.Equal(got, original) {
				t.Fatal("export differs from independent raw file bytes")
			}
			if err := draft.SetTerrain(1, 0x77); err != nil {
				t.Fatal(err)
			}
			source := draft.Config()
			exported := source.Bytes()
			wantExport := bytes.Clone(original)
			wantExport[0x2001] = 0x77
			if source.Size() != 9216 || !bytes.Equal(exported, wantExport) {
				t.Fatal("export does not capture valid manual edits")
			}
			exported[0] = 0xFF
			if err := draft.SetTerrain(1, 0x88); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(source.Bytes(), wantExport) {
				t.Fatal("file snapshot aliases exported bytes or live draft")
			}
			content := bytes.Clone(original)
			copy(content[:8], []byte{1, 0, 0, 0x18, 0, 0x24, 1, 0x4C})
			content[0x2000] = 0x7F
			path := filepath.Join(t.TempDir(), "tiles.mapchip_config")
			if err := os.WriteFile(path, content, 0o600); err != nil {
				t.Fatal(err)
			}
			input, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			imported, err := source.Import(input)
			if err != nil {
				t.Fatal(err)
			}
			input[0] = 0xFF
			if got, _ := draft.Definition(
				1,
			); got.Terrain != 0x88 ||
				!bytes.Equal(source.Bytes(), wantExport) {
				t.Fatal("import mutated the manual draft or file snapshot")
			}
			if !bytes.Equal(imported.Config().Bytes(), content) {
				t.Fatal("imported bytes differ from raw file")
			}
			view, err := imported.Preview()
			if err != nil || view.Cells[0].Terrain != 0x7F {
				t.Fatal("replacement preview", err)
			}
			plan, err := s.Maps.PlanTileset(imported)
			if err != nil {
				t.Fatal(err)
			}
			if changes := plan.Changes(); len(changes) != 1 ||
				changes[0].Index != 0 {
				t.Fatal(
					"import did not replace the whole configuration",
					changes,
				)
			}
			if uses := plan.Uses(); len(uses) != 3 || !uses[0].Updated ||
				!uses[1].Updated ||
				uses[2].Updated {
				t.Fatal("shared and independent entries", uses)
			}
			if err := imported.SetTerrain(0, 1); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("file import or planning changed ROM")
			}
			if err := s.Maps.ApplyTileset(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			decoded, err := gba.DecompressLZ77(
				after.Data[plan.Destination():],
				0x2400,
			)
			if err != nil || !bytes.Equal(decoded, content) {
				t.Fatal(
					"frozen import bytes, custom words or terrain changed",
					err,
				)
			}
			preserved := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				preserved[plan.Pointer():plan.Pointer()+4],
				before.Data[plan.Pointer():plan.Pointer()+4],
			)
			if !bytes.Equal(preserved, before.Data) {
				t.Fatal(
					"old payload, unknown tail, independent entry or other resources changed",
				)
			}
			if source.Current(s.Maps) || !s.Document.Undo() ||
				s.Document.State().CanUndo ||
				!bytes.Equal(s.Document.Snapshot().Data, before.Data) ||
				!s.Document.Redo() ||
				!bytes.Equal(s.Document.Snapshot().Data, after.Data) {
				t.Fatal("atomic history or file snapshot validity")
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range []int{0, 1, 2} {
				next, err := reopened.Maps.NewTilesetDraft(id)
				if err != nil {
					t.Fatal(err)
				}
				want := content
				if id == 2 {
					want = original
				}
				if !bytes.Equal(next.Config().Bytes(), want) {
					t.Fatal("saved configuration or independent entry", id)
				}
			}
		})
	}
}

func TestTilesetConfigImportRejectsInvalidFilesWithoutChangingDrafts(
	t *testing.T,
) {
	f := testutil.NewMapFixture("FE8U")
	// A short configuration has only three terrain entries. Remaining words
	// cannot be classified as editable definitions and must be kept verbatim.
	config := make([]byte, 0x2003)
	copy(
		config,
		[]byte{0, 0, 0, 0x14, 0, 0x28, 0, 0x3C, 1, 0, 1, 0, 1, 0, 1, 0},
	)
	config[0x2000], config[0x2001] = 1, 0x21
	config[17] = 0xF0
	config[100] = 0xAB
	compressed, err := gba.CompressLZ77(config)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteAt(f.GetPosition("map_config"), compressed)
	f.WriteU8(
		f.GetPosition("map_layout"),
		0,
	) // Optional map preview is damaged.
	s := testutil.Open(t, f)
	draft, err := s.Maps.NewTilesetDraft(0)
	if err != nil {
		t.Fatal(err)
	}
	if err := draft.SetTerrain(0, 0x77); err != nil {
		t.Fatal(err)
	}
	source := draft.Config()
	before, history := s.Document.Snapshot(), s.Document.State()
	for _, scenario := range []struct {
		name   string
		change func([]byte) []byte
		code   error
	}{
		{"empty", func(b []byte) []byte { return nil }, maps.ErrTilesetConfigSize},
		{"truncated", func(b []byte) []byte { return b[:len(b)-1] }, maps.ErrTilesetConfigSize},
		{"extra-tail", func(b []byte) []byte { return append(b, 0) }, maps.ErrTilesetConfigSize},
		{"full-config-expansion", func(b []byte) []byte { return append(b, make([]byte, 0x2400-len(b))...) }, maps.ErrTilesetConfigSize},
		{"unknown-word", func(b []byte) []byte { b[100] ^= 1; return b }, maps.ErrTilesetConfigData},
		{"missing-tile", func(b []byte) []byte { b[0] = 2; return b }, maps.ErrTilesetConfigData},
		{"graphics-gap", func(b []byte) []byte { b[0], b[1] = 0, 1; return b }, maps.ErrTilesetConfigData},
		{"new-palette", func(b []byte) []byte { b[1] = 0x50; return b }, maps.ErrTilesetConfigData},
		{"changed-custom-palette", func(b []byte) []byte { b[17] = 0xE0; return b }, maps.ErrTilesetConfigData},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			got, err := source.Import(scenario.change(bytes.Clone(config)))
			if got != nil || !errors.Is(err, scenario.code) {
				t.Fatal("invalid file accepted or wrong code", err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				history != s.Document.State() ||
				!bytes.Equal(draft.Config().Bytes(), source.Bytes()) {
				t.Fatal(
					"failed import changed document, history or manual draft",
				)
			}
		})
	}
	unchanged, err := source.Import(config)
	if err != nil || unchanged.Modified() {
		t.Fatal(
			"unchanged file normalized custom words or retained manual edits",
			err,
		)
	}
	if _, err := s.Maps.PlanTileset(unchanged); !errors.Is(err, edit.ErrValue) {
		t.Fatal("unchanged file created a write", err)
	}
	content := bytes.Clone(config)
	content[16] = 1 // Change tile while retaining the original custom palette.
	content[0x2002] = 0xFE
	imported, err := source.Import(content)
	if err != nil || imported.Count() != 3 {
		t.Fatal("valid short configuration rejected", err)
	}
	if _, err := imported.Preview(); err == nil {
		t.Fatal("damaged optional map preview accepted")
	}
	plan, err := s.Maps.PlanTileset(imported)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Maps.ApplyTileset(plan); err != nil {
		t.Fatal(err)
	}
	next, err := s.Maps.NewTilesetDraft(0)
	if err != nil || !bytes.Equal(next.Config().Bytes(), content) {
		t.Fatal("short configuration length or unknown words changed", err)
	}
}
