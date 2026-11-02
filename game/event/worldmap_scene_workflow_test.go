package event_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func TestWorldMapSceneParametersAndHistory(t *testing.T) {
	for _, version := range []string{"FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewWorldMapSceneFixture(t, version)
			s := testutil.Open(t, f)
			d := worldMapScript(t, s, f.GetPosition("wm_site"))
			wantOps := []event.ScriptOp{
				event.ScriptWMInitScene,
				event.ScriptWMDarken,
				event.ScriptWMWaitDarken,
				event.ScriptWMShowTextBox,
				event.ScriptWMWaitTextBox,
				event.ScriptWMRevealScreen,
				event.ScriptWMWaitRevealScreen,
				event.ScriptWMTextBottom,
				event.ScriptWMShowPortrait,
				event.ScriptWMText,
				event.ScriptTextWait,
				event.ScriptWMClearPortrait,
				event.ScriptWMTextHide,
				event.ScriptWMSkipScene,
				event.ScriptWMNoFade,
				event.ScriptWMFadeBlack,
				event.ScriptWMEndScene,
				event.ScriptWait,
				event.ScriptEnd,
			}
			var ops []event.ScriptOp
			for _, row := range d.Instructions() {
				ops = append(ops, row.Op)
			}
			if !d.Complete() || !d.Relocatable() ||
				!slices.Equal(ops, wantOps) {
				t.Fatal("scene decoding or premature world-map end", ops)
			}
			for _, value := range []struct {
				row   int
				field string
				want  int64
			}{
				{1, event.ScriptWMFadeFrames, -2}, {3, event.ScriptWMTextBoxCurve, -1}, {5, event.ScriptWMRevealStyle, 2}, {8, event.ScriptWMFaceSlot, 0}, {8, event.ScriptWMPortrait, 1}, {8, event.ScriptWMFaceX, 52}, {8, event.ScriptWMFaceSlide, 1}, {8, event.ScriptWMFaceFlip, 1}, {8, event.ScriptWMFaceLayer, 1}, {8, event.ScriptWMFaceDelay, -2}, {9, event.ScriptTextID, 3}, {11, event.ScriptWMFaceSlot, 1}, {11, event.ScriptWMFaceSlide, 1}, {11, event.ScriptWMFaceDelay, 65535},
			} {
				got, err := d.Value(value.row, value.field)
				if err != nil || got != value.want {
					t.Fatal("native operand", value, got, err)
				}
			}
			if err := d.Set(
				9,
				event.ScriptTextID,
				3,
			); err != nil ||
				d.Modified() {
				t.Fatal("unchanged text normalized its split encoding", err)
			}
			for _, change := range []struct {
				row   int
				field string
				value int64
			}{
				{1, event.ScriptWMFadeFrames, 5}, {3, event.ScriptWMTextBoxFrames, 90}, {3, event.ScriptWMTextBoxCurve, 2}, {5, event.ScriptWMRevealStyle, 0}, {8, event.ScriptWMPortrait, 2}, {8, event.ScriptWMFaceX, 188}, {8, event.ScriptWMFaceSlide, 2}, {8, event.ScriptWMFaceFlip, 0}, {8, event.ScriptWMFaceLayer, 3}, {8, event.ScriptWMFaceDelay, 3}, {9, event.ScriptTextID, 2}, {11, event.ScriptWMFaceSlot, 0}, {11, event.ScriptWMFaceSlide, 2}, {11, event.ScriptWMFaceDelay, 1},
			} {
				if err := d.Set(
					change.row,
					change.field,
					change.value,
				); err != nil {
					t.Fatal(change, err)
				}
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			plan, err := s.Events.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Relocated() || len(plan.Uses()) < 3 ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("shared parameter preview")
			}
			if err = d.Set(8, event.ScriptWMPortrait, 1); err != nil {
				t.Fatal(err)
			}
			if err = s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			want := bytes.Clone(before.Data)
			copy(want[f.GetPosition("wm_scene"):], []byte{
				0x20, 0xC0, 0x11, 0x22,
				0x40, 0xAD, 0x33, 0x44, 5, 0, 0, 0,
				0x20, 0xAE, 0x55, 0x66,
				0x40, 0xAF, 0x77, 0x88, 90, 0, 2, 0,
				0x20, 0xB1, 0x99, 0xAA,
				0x40, 0xB2, 0xBB, 0xCC, 0, 0xAA, 0xBB, 0xCC,
				0x20, 0xB3, 0xDD, 0xEE,
				0x20, 0xC7, 0x12, 0x34,
				0x60, 0xC4, 0x56, 0x78, 0, 0, 2, 0, 188, 0xE2, 3, 0,
				0x40, 0xC6, 0x9A, 0xBC, 0, 0, 2, 0,
				0x20, 0x1D, 0, 0,
				0x40, 0xC5, 0, 0, 0x5B, 0xD2, 1, 0,
				0x20, 0xC9, 0xDE, 0xF0,
				0x20, 0xC1, 0x11, 0x22,
				0x20, 0xC2, 0x33, 0x44,
				0x20, 0x81, 0x55, 0x66,
				0x20, 0x82, 0x77, 0x88,
				0x20, 0x0E, 1, 0,
				0x20, 1, 0, 0,
			})
			if !bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal(
					"scene edit changed unrelated bits, resources, aliases or frozen values",
				)
			}
			alias := worldMapScript(t, s, f.GetPosition("wm_alias_site"))
			if id, _ := alias.Value(9, event.ScriptTextID); id != 2 {
				t.Fatal("shared text not updated", id)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("atomic scene undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal("atomic scene redo")
			}
			if err = s.Events.ApplyScriptEdit(
				plan,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatal("stale scene replay", err)
			}
		})
	}
}

func TestWorldMapSceneRelocationKeepsAliases(t *testing.T) {
	for _, version := range []string{"FE8U", "FE8J"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewWorldMapSceneFixture(t, version)
			s := testutil.Open(t, f)
			site, at := f.GetPosition("wm_site"), f.GetPosition("wm_scene")
			d := worldMapScript(t, s, site)
			if err := d.Set(8, event.ScriptWMPortrait, 2); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(9, event.ScriptTextID, 2); err != nil {
				t.Fatal(err)
			}
			if err := d.Insert(9, event.ScriptWMOpenRoute); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(9, event.ScriptWMRoute, 7); err != nil {
				t.Fatal(err)
			}
			if err := d.Move(9, 7); err != nil {
				t.Fatal(err)
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			plan, err := s.Events.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if !plan.Relocated() || plan.Length() != 112 ||
				state != s.Document.State() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("scene relocation preview")
			}
			for _, use := range plan.Uses() {
				if use.Pointer != site {
					t.Fatal("independent pointer included in redirection", use)
				}
			}
			if err = d.Remove(7); err != nil {
				t.Fatal(err)
			}
			if err = s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			body := bytes.Clone(before.Data[at : at+104])
			body[50], body[60] = 2, 0
			want := bytes.Clone(before.Data)
			binary.LittleEndian.PutUint32(
				want[site:],
				uint32(len(want))+0x08000000,
			)
			want = append(want, body[:40]...)
			want = append(want, 0x40, 0x91, 0, 0, 7, 0, 0, 0)
			want = append(want, body[40:]...)
			if !bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal(
					"relocation changed old code, independent aliases or resources",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("undo relocated scene")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal("redo relocated scene")
			}
			if err = s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			fresh := worldMapScript(t, reopened, site)
			if !fresh.Relocatable() || len(fresh.Instructions()) != 20 {
				t.Fatal("reopened scene")
			}
			if id, _ := fresh.Value(9, event.ScriptWMPortrait); id != 2 {
				t.Fatal("reopened portrait", id)
			}
			if id, _ := fresh.Value(10, event.ScriptTextID); id != 2 {
				t.Fatal("reopened narration", id)
			}
			alias := worldMapScript(t, reopened, f.GetPosition("wm_alias_site"))
			if id, _ := alias.Value(9, event.ScriptTextID); id != 3 {
				t.Fatal("independent alias redirected", id)
			}
		})
	}
}

func TestWorldMapSceneCreation(t *testing.T) {
	f := testutil.NewWorldMapSceneFixture(t, "FE8U")
	s := testutil.Open(t, f)
	site := f.GetPosition("wm_intro") + 9*4
	d := worldMapScript(t, s, site)
	for index, op := range []event.ScriptOp{event.ScriptWMInitScene, event.ScriptWMTextTop, event.ScriptWMShowPortrait, event.ScriptWMText, event.ScriptTextWait, event.ScriptWMClearPortrait, event.ScriptWMTextHide, event.ScriptWMFadeBlack, event.ScriptWMEndScene, event.ScriptEnd} {
		if err := d.Insert(index, op); err != nil {
			t.Fatal(op, err)
		}
	}
	before := s.Document.Snapshot()
	plan, err := s.Events.PlanScriptEdit(d)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Events.ApplyScriptEdit(plan); err != nil {
		t.Fatal(err)
	}
	want := bytes.Clone(before.Data)
	binary.LittleEndian.PutUint32(want[site:], uint32(len(want))+0x08000000)
	want = append(want, []byte{
		0x20, 0xC0, 0, 0,
		0x20, 0xC8, 0, 0,
		0x60, 0xC4, 0, 0, 0, 0, 1, 0, 52, 5, 0, 0,
		0x40, 0xC6, 0, 0, 1, 0, 0, 0,
		0x20, 0x1D, 0, 0,
		0x40, 0xC5, 0, 0, 0, 1, 0, 0,
		0x20, 0xC9, 0, 0,
		0x20, 0x81, 0, 0,
		0x20, 0x82, 0, 0,
		0x20, 1, 0, 0,
	}...)
	if !bytes.Equal(want, s.Document.Snapshot().Data) {
		t.Fatal("new scene defaults or unrelated data")
	}
	if !s.Document.Undo() ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
		t.Fatal("new scene undo")
	}
}

func TestWorldMapSceneReferencesAndOpaqueBytes(t *testing.T) {
	for _, scenario := range []string{"missing-portrait", "class-card", "damaged-portrait", "missing-text", "high-direct-text", "unreadable-unchanged-portrait", "opaque-instruction", "split-duplicate-bits"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewWorldMapSceneFixture(t, "FE8U")
			at := f.GetPosition("wm_scene")
			switch scenario {
			case "damaged-portrait":
				f.WriteU8(testutil.ImageFace, 0xFF)
			case "unreadable-unchanged-portrait":
				f.WriteU32(f.GetPosition("wm_portrait_root"), 0)
			case "opaque-instruction":
				f.WriteAt(at+24, []byte{0x20, 0xD0, 0x99, 0xAA})
			case "split-duplicate-bits":
				f.WriteAt(at+60, []byte{3, 0, 3, 0})
			}
			s := testutil.Open(t, f)
			d := worldMapScript(t, s, f.GetPosition("wm_site"))
			row, field, value := 8, event.ScriptWMPortrait, int64(2)
			accepted := false
			switch scenario {
			case "missing-portrait":
				value = 99
			case "class-card":
				value = 3
			case "missing-text":
				row, field, value = 9, event.ScriptTextID, 4
			case "high-direct-text":
				row, field, value = 9, event.ScriptTextID, 0x8001
			case "unreadable-unchanged-portrait":
				field, value, accepted = event.ScriptWMFaceX, 200, true
			case "opaque-instruction":
				accepted = true
				if d.Relocatable() {
					t.Fatal("opaque command became relocatable")
				}
				if err := d.Insert(
					0,
					event.ScriptWMTextTop,
				); !errors.Is(
					err,
					event.ErrScriptStructure,
				) {
					t.Fatal("opaque insertion", err)
				}
			case "split-duplicate-bits":
				accepted = true
				if err := d.Set(
					9,
					event.ScriptTextID,
					3,
				); err != nil ||
					d.Modified() {
					t.Fatal("duplicate halves normalized", err)
				}
				row, field, value = 9, event.ScriptTextID, 2
			}
			if err := d.Set(row, field, value); err != nil {
				t.Fatal(err)
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			plan, err := s.Events.PlanScriptEdit(d)
			if state != s.Document.State() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("preview changed bytes/history")
			}
			if !accepted {
				if err == nil || plan != nil {
					t.Fatal("invalid reference accepted", scenario)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			want := bytes.Clone(before.Data)
			switch scenario {
			case "unreadable-unchanged-portrait":
				want[at+52] = 200
			case "opaque-instruction":
				want[at+50] = 2
			case "split-duplicate-bits":
				want[at+60], want[at+62] = 2, 2
			}
			if !bytes.Equal(want, s.Document.Snapshot().Data) {
				t.Fatal("unchanged encoding or unrelated field rewritten")
			}
		})
	}
	// Invalid native slot and bitfield values are rejected without changing the draft.
	f := testutil.NewWorldMapSceneFixture(t, "FE8J")
	s := testutil.Open(t, f)
	d := worldMapScript(t, s, f.GetPosition("wm_site"))
	for _, value := range []struct {
		row   int
		field string
		value int64
	}{
		{8, event.ScriptWMFaceSlot, 2}, {11, event.ScriptWMFaceSlot, 2}, {8, event.ScriptWMFaceX, 256}, {8, event.ScriptWMFaceSlide, 4}, {8, event.ScriptWMFaceFlip, 2}, {8, event.ScriptWMFaceLayer, 4}, {8, event.ScriptWMFaceDelay, 32768}, {11, event.ScriptWMFaceDelay, -1}, {3, event.ScriptWMTextBoxFrames, 65536}, {3, event.ScriptWMTextBoxCurve, 32768}, {5, event.ScriptWMRevealStyle, 256},
	} {
		if err := d.Set(
			value.row,
			value.field,
			value.value,
		); !errors.Is(err, edit.ErrValue) ||
			d.Modified() {
			t.Fatal("invalid scene operand", value, err)
		}
	}
}
