package maps_test

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestTilesetDefinitionEditingWorkflow(t *testing.T) {
	for _, version := range testutil.Games() {
		t.Run(version.Name, func(t *testing.T) {
			f := testutil.NewMapFixture(version.Name)
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
			f.WriteU8(
				chapter+2*stride+4,
				5,
			) // Independent entry has fewer graphics.
			f.WritePointer(
				f.GetPosition("map_config_list")+9*4,
				f.GetPosition("map_config"),
			)
			original, err := gba.DecompressLZ77(
				f.Bytes()[f.GetPosition("map_config"):],
				0x2400,
			)
			if err != nil {
				t.Fatal(err)
			}
			// An unused invalid definition and unknown terrain ID are retained.
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
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			layout, err := s.Maps.NewDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			draft, err := s.Maps.NewTilesetDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			definition, err := draft.Definition(0)
			if err != nil || draft.Count() != 1024 ||
				definition != (maps.TileDefinition{Parts: [4]maps.TilePart{{}, {Palette: 1, FlipX: true}, {Palette: 2, FlipY: true}, {Palette: 3, FlipX: true, FlipY: true}}, Terrain: 1}) {
				t.Fatal("native definition", definition, err)
			}
			parts := [4]maps.TilePart{
				{Tile: 1},
				{Palette: 1, FlipY: true},
				{Palette: 2, FlipX: true},
				{Tile: 1, Palette: 4, FlipX: true, FlipY: true},
			}
			for index, part := range parts {
				if err := draft.SetPart(0, index, part); err != nil {
					t.Fatal(err)
				}
			}
			if err := draft.SetTerrain(0, 0x7F); err != nil {
				t.Fatal(err)
			}
			preview, err := draft.Preview()
			if err != nil || preview.Cells[0].Terrain != 0x7F {
				t.Fatal("map preview", err)
			}
			for _, check := range []struct {
				x, y  int
				color color.RGBA
			}{
				{0, 0, color.RGBA{255, 0, 255, 255}},
				{8, 0, color.RGBA{0, 0, 255, 255}},
				{0, 8, color.RGBA{0, 255, 0, 255}},
			} {
				if got := preview.Image.RGBAAt(
					check.x,
					check.y,
				); got != check.color {
					t.Fatalf(
						"pixel (%d,%d) = %v, want %v",
						check.x,
						check.y,
						got,
						check.color,
					)
				}
			}
			if err := layout.Stroke(
				[]image.Point{{X: 1, Y: 0}},
				0,
			); err != nil {
				t.Fatal(err)
			}
			combined, err := layout.PreviewWithTileset(draft)
			if err != nil || combined.Cells[1].Tile != 0 ||
				combined.Cells[1].Terrain != 0x7F ||
				combined.Image.RGBAAt(16, 0) != (color.RGBA{255, 0, 255, 255}) {
				t.Fatal(
					"layout and definition edits were not rendered together",
					err,
				)
			}
			layoutOnly, err := layout.Preview()
			if err != nil || layoutOnly.Cells[1].Terrain != 1 {
				t.Fatal(
					"composing the preview changed the layout's source definitions",
					err,
				)
			}
			definitionOnly, err := draft.Preview()
			if err != nil || definitionOnly.Cells[1].Tile != 4 {
				t.Fatal(
					"composing the preview changed the definition draft's layout",
					err,
				)
			}
			otherChapter, err := s.Maps.NewTilesetDraft(1)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := layout.PreviewWithTileset(
				otherChapter,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatal(
					"combined preview accepted another chapter's resources",
					err,
				)
			}
			if !layout.Undo() || layout.Modified() {
				t.Fatal("combined preview changed layout history")
			}
			plan, err := s.Maps.PlanTileset(draft)
			if err != nil {
				t.Fatal(err)
			}
			uses := plan.Uses()
			if len(uses) != 3 || !uses[0].Updated || !uses[1].Updated ||
				uses[2].Updated {
				t.Fatal("shared/independent configuration entries", uses)
			}
			changes := plan.Changes()
			if len(changes) != 1 || changes[0].Index != 0 ||
				changes[0].After.Terrain != 0x7F {
				t.Fatal("changes", changes)
			}
			changes[0].After.Terrain = 0
			uses[0].Updated = false
			if plan.Changes()[0].After.Terrain != 0x7F ||
				!plan.Uses()[0].Updated {
				t.Fatal("mutable preview values")
			}
			if err := draft.SetTerrain(0, 2); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("draft/preview wrote ROM")
			}
			if err := s.Maps.ApplyTileset(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			at, ok := gba.Pointer(after.Data, plan.Pointer())
			if !ok || at != plan.Destination() ||
				len(after.Data) != at+plan.Length() ||
				layout.Current(s.Maps) {
				t.Fatal("pointer/size/stamp")
			}
			freshDefinitions, err := s.Maps.NewTilesetDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := layout.PreviewWithTileset(
				freshDefinitions,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatal("combined preview mixed document revisions", err)
			}
			decoded, err := gba.DecompressLZ77(after.Data[at:], 0x2400)
			want := bytes.Clone(original)
			copy(want[:8], []byte{1, 0, 0, 0x18, 0, 0x24, 1, 0x4C})
			want[0x2000] = 0x7F
			if err != nil || !bytes.Equal(want, decoded) {
				t.Fatal(
					"configuration bytes or unused definitions changed",
					err,
				)
			}
			preserved := bytes.Clone(after.Data[:len(before.Data)])
			copy(
				preserved[plan.Pointer():plan.Pointer()+4],
				before.Data[plan.Pointer():plan.Pointer()+4],
			)
			if !bytes.Equal(before.Data, preserved) {
				t.Fatal(
					"old payload, graphics, palette, layouts or independent entries changed",
				)
			}
			for _, chapter := range []int{0, 1, 2} {
				next, err := s.Maps.NewTilesetDraft(chapter)
				if err != nil {
					t.Fatal(err)
				}
				got, _ := next.Definition(0)
				terrain := 0x7F
				if chapter == 2 {
					terrain = 1
				}
				if got.Terrain != terrain {
					t.Fatal("shared chapter", chapter, got.Terrain)
				}
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				s.Document.State().CanUndo {
				t.Fatal("atomic undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("atomic redo")
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil ||
				!bytes.Equal(after.Data, reopened.Document.Snapshot().Data) {
				t.Fatal("save/reopen", err)
			}
			saved, err := reopened.Maps.NewTilesetDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			got, _ := saved.Definition(0)
			if got != (maps.TileDefinition{Parts: parts, Terrain: 0x7F}) {
				t.Fatal("reopened definition", got)
			}
		})
	}
}

func TestTilesetSharedGraphicsAndWriteFailuresAreAtomic(t *testing.T) {
	for _, scenario := range []string{"shared-graphics", "record-overlap", "palette-overlap", "other-resource-entry", "revision", "layout", "document", "capacity"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewMapFixture("FE8U")
			source := f.GetPosition("map_config")
			switch scenario {
			case "shared-graphics":
				f.WriteU8(f.GetPosition("chapters")+148+4, 5)
			case "record-overlap":
				f.WritePointer(
					f.GetPosition("map_config_pointer"),
					f.GetPosition("chapters")-12,
				)
				f.WritePointer(f.GetPosition("chapters"), source)
			case "palette-overlap":
				f.WritePointer(
					f.GetPosition("map_config_pointer"),
					f.GetPosition("map_palette")-12,
				)
				f.WritePointer(f.GetPosition("map_palette"), source)
			case "other-resource-entry":
				f.WritePointer(
					f.GetPosition("map_layout_pointer"),
					f.GetPosition("map_config_list"),
				)
				f.WriteU8(f.GetPosition("chapters")+148+8, 3)
			}
			s := testutil.Open(t, f)
			if scenario == "capacity" {
				if err := s.Document.ApplyEdit(
					s.Document.State().Revision,
					"grow",
					rom.Edit{Size: rom.MaxSize},
				); err != nil {
					t.Fatal(err)
				}
			}
			draft, err := s.Maps.NewTilesetDraft(0)
			if err != nil {
				t.Fatal("readable draft", err)
			}
			if err := draft.SetPart(0, 0, maps.TilePart{Tile: 1}); err != nil {
				t.Fatal(err)
			}
			sourceFile := draft.Config()
			draft, err = sourceFile.Import(sourceFile.Bytes())
			if err != nil {
				t.Fatal("configuration file import", err)
			}
			var plan *maps.TilesetPlan
			if scenario == "revision" || scenario == "layout" ||
				scenario == "document" {
				plan, err = s.Maps.PlanTileset(draft)
				if err != nil {
					t.Fatal(err)
				}
				switch scenario {
				case "revision":
					err = s.Records.Set(records.Units, []int{1}, "level", 9)
				case "layout":
					table := s.Records.Table(records.Chapters)
					err = s.Records.UseLayout(
						records.Chapters,
						records.Layout{
							Address: table.Address,
							Stride:  table.Stride,
							Count:   1,
						},
					)
				case "document":
					s = testutil.Open(t, f)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			_, err = s.Maps.PlanTileset(draft)
			if err == nil {
				t.Fatal("unsafe plan accepted")
			}
			if plan != nil {
				if !errors.Is(err, rom.ErrStale) {
					t.Fatal("stale draft", err)
				}
				if err := s.Maps.ApplyTileset(
					plan,
				); !errors.Is(
					err,
					rom.ErrStale,
				) {
					t.Fatal("stale plan", err)
				}
			}
			if scenario == "shared-graphics" &&
				!errors.Is(err, maps.ErrTilesetReference) {
				t.Fatal("shared graphics error code", err)
			}
			if scenario == "capacity" && !errors.Is(err, space.ErrCapacity) {
				t.Fatal("capacity", err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("failure changed bytes/history")
			}
		})
	}
}

func TestPartialTilesetConfigurationPreservesSource(t *testing.T) {
	f := testutil.NewMapFixture("FE8U")
	config, err := gba.DecompressLZ77(
		f.Bytes()[f.GetPosition("map_config"):],
		0x2400,
	)
	if err != nil {
		t.Fatal(err)
	}
	config = config[:0x2003]
	config[17] = 0xF0 // Existing custom palette bits in the third definition.
	compressed, err := gba.CompressLZ77(config)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteAt(f.GetPosition("map_config"), compressed)
	s := testutil.Open(t, f)
	draft, err := s.Maps.NewTilesetDraft(0)
	if err != nil || draft.Count() != 3 {
		t.Fatal("partial definition count", err)
	}
	if err := draft.SetPart(
		2,
		0,
		maps.TilePart{Tile: 1, Palette: 15},
	); err != nil {
		t.Fatal("unchanged custom palette", err)
	}
	if err := draft.SetPart(
		2,
		0,
		maps.TilePart{Tile: 1, Palette: 14},
	); !errors.Is(
		err,
		edit.ErrValue,
	) {
		t.Fatal("new non-native palette accepted", err)
	}
	if err := draft.SetPart(2, 0, maps.TilePart{Palette: 15}); err != nil {
		t.Fatal("restore original part", err)
	}
	if draft.Modified() {
		t.Fatal("restoration left changes")
	}
	if err := draft.SetTerrain(2, 255); err != nil {
		t.Fatal(err)
	}
	if err := draft.SetTerrain(3, 1); !errors.Is(err, edit.ErrValue) {
		t.Fatal("partial config expanded", err)
	}
	plan, err := s.Maps.PlanTileset(draft)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Maps.ApplyTileset(plan); err != nil {
		t.Fatal(err)
	}
	after := s.Document.Snapshot().Data
	decoded, err := gba.DecompressLZ77(after[plan.Destination():], 0x2400)
	want := bytes.Clone(config)
	want[0x2002] = 255
	if err != nil || !bytes.Equal(decoded, want) {
		t.Fatal("partial config length or unknown words changed", err)
	}
}

func TestTilesetBoundsAndIndependentLayoutPreview(t *testing.T) {
	f := testutil.NewMapFixture("FE8U")
	f.WriteU8(f.GetPosition("map_layout"), 0)
	s := testutil.Open(t, f)
	draft, err := s.Maps.NewTilesetDraft(0)
	if err != nil {
		t.Fatal("broken layout blocked tileset", err)
	}
	if _, err := draft.Preview(); err == nil {
		t.Fatal("broken layout preview accepted")
	}
	if _, err := draft.Tileset(); err != nil {
		t.Fatal("broken layout blocked definitions", err)
	}
	before, _ := draft.Definition(0)
	for _, operation := range []func() error{
		func() error { return draft.SetPart(-1, 0, maps.TilePart{}) },
		func() error { return draft.SetPart(1024, 0, maps.TilePart{}) },
		func() error { return draft.SetPart(0, 4, maps.TilePart{}) },
		func() error { return draft.SetPart(0, 0, maps.TilePart{Tile: 2}) },
		func() error { return draft.SetPart(0, 0, maps.TilePart{Palette: 5}) },
		func() error { return draft.SetTerrain(0, -1) },
		func() error { return draft.SetTerrain(0, 256) },
	} {
		if err := operation(); !errors.Is(err, edit.ErrValue) {
			t.Fatal("invalid input accepted", err)
		}
	}
	after, _ := draft.Definition(0)
	if before != after || draft.Modified() {
		t.Fatal("invalid input mutated draft")
	}
	if _, err := s.Maps.PlanTileset(draft); !errors.Is(err, edit.ErrValue) {
		t.Fatal("no-op plan", err)
	}
	if err := draft.SetTerrain(0, 0xE3); err != nil {
		t.Fatal(err)
	}
	plan, err := s.Maps.PlanTileset(draft)
	if err != nil {
		t.Fatal("broken layout blocked independent plan", err)
	}
	if err := s.Maps.ApplyTileset(plan); err != nil {
		t.Fatal(err)
	}
}
