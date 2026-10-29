package game

import (
	"bytes"
	"encoding/hex"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

// These literal scenes follow the native catalogs independently of the schema:
// background conversation, recruitment, movement and an item reward.
func sceneFixture(
	t *testing.T,
	version string,
) (*testutil.FixtureBuilder, []byte, []byte) {
	t.Helper()
	var original, changed string
	switch version {
	case "FE8U", "FE8J":
		original = `21171000 20230000 4021060000000000 20171000 20123100
			201A0000 201B0100 201D0000 221B0000 20240000
			22340100 402F04000100050A 20300000 4005030001000000 20370100
			20143200 20160600 21140600 23171000 22171000 28020700 20020900 2137FFFF 20010000`
		changed = `21171000 20230000 4021060000000000 20171000 20123200
			201A0000 201B0100 201D0000 221B0000 20240000
			22340200 402FFFFF0100FF0A 20300000 4005030002000000 2037FDFF
			20143200 20160600 21140600 23171000 22171000 28020700 20020900 2137FFFF 20010000`
	case "FE7J":
		original = `8C001000 90000000 05000600 8D001000 76003100
			0D00000001000000 09000000 91000000
			630000000100000000000000 240000000100000005000A00 39000000
			5A0000000100000001000000
			77003200 7D000600 78000600 7A000400 8E001000 8F001000 250000000100000005000A0004000000 0A00000000000000`
		changed = `8C001000 90000000 05000600 8D001000 76003200
			0D00000001000000 09000000 91000000
			630000000200000000000000 2400000001000000FFFF0A00 39000000
			5A0000000100000002000000
			77003200 7D000600 78000600 7A000400 8E001000 8F001000 250000000100000005000A0004000000 0A00000000000000`
	case "FE7U":
		original = `8E001000 92000000 05000600 8F001000 78003100
			0D00000001000000 09000000 93000000
			650000000100000000000000 260000000100000005000A00 3B000000
			5C0000000100000001000000
			79003200 7F000600 7A000600 7C000400 90001000 91001000 270000000100000005000A0004000000 0A00000000000000`
		changed = `8E001000 92000000 05000600 8F001000 78003200
			0D00000001000000 09000000 93000000
			650000000200000000000000 2600000001000000FFFF0A00 3B000000
			5C0000000100000002000000
			79003200 7F000600 7A000600 7C000400 90001000 91001000 270000000100000005000A0004000000 0A00000000000000`
	case "FE6":
		original = `4300000010000000 47000000 0300000006000000 4400000010000000 3600000031000000
			0700000001000000 05000000 48000000
			2B0000000100000000000000 0F0000000100000005000A00 14000000
			270000000100000001000000
			3700000032000000 3C00000006000000 38000000 3900000004000000 4500000010000000 4600000010000000 0600000000000000`
		changed = `4300000010000000 47000000 0300000006000000 4400000010000000 3600000032000000
			0700000001000000 05000000 48000000
			2B0000000200000000000000 0F00000001000000FFFF0A00 14000000
			270000000100000002000000
			3700000032000000 3C00000006000000 38000000 3900000004000000 4500000010000000 4600000010000000 0600000000000000`
	default:
		t.Fatal("unsupported scene fixture version")
	}
	decode := func(source string) []byte {
		data, err := hex.DecodeString(strings.Join(strings.Fields(source), ""))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	prefix, changedPrefix, tail := cameraSceneParts(version)
	if version != "FE8J" && version != "FE8U" {
		// Native early return (1) leaves later labels searchable; END (0) does not.
		original = strings.TrimSuffix(original, "00000000") + "01000000"
		changed = strings.TrimSuffix(changed, "00000000") + "01000000"
	}
	f := testutil.NewScriptFixture(version)
	before := decode(prefix + original + tail)
	after := decode(changedPrefix + changed + tail)
	f.WriteAt(f.GetPosition("script_root"), before)
	return f, before, after
}

func sceneInstruction(t *testing.T, d *ScriptDraft, op ScriptOp) int {
	t.Helper()
	index := slices.IndexFunc(
		d.Instructions(),
		func(row ScriptInstruction) bool { return row.Op == op },
	)
	if index < 0 {
		t.Fatalf("scene has no %s", op)
	}
	return index
}

func TestScriptSceneWorkflow(t *testing.T) {
	for _, fixture := range gameFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			f, _, changed := sceneFixture(t, fixture.name)
			s := openFixture(t, f)
			site, root := f.GetPosition(
				"script_site",
			), f.GetPosition(
				"script_root",
			)
			before := s.Document.Snapshot()
			d, err := s.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			if !d.Complete() || !d.Relocatable() {
				t.Fatal("native scene must support structural editing")
			}
			set := func(op ScriptOp, field string, value int64) {
				t.Helper()
				if err := d.Set(
					sceneInstruction(t, d, op),
					field,
					value,
				); err != nil {
					t.Fatal(err)
				}
			}
			set(ScriptCameraUnit, ScriptCharacter, 2)
			switch fixture.name {
			case "FE8J", "FE8U":
				set(ScriptCameraPosition, ScriptX, -1)
				set(ScriptCursorUnit, ScriptCharacter, -2)
				set(ScriptCheckUnitExists, ScriptCharacter, 0)
				set(ScriptCheckUnitAlive, ScriptCharacter, -3)
				set(ScriptCheckUnitDeployed, ScriptCharacter, -2)
				set(ScriptCheckUnitFaction, ScriptCharacter, -1)
			default:
				x := int64(-1)
				if fixture.name != "FE6" {
					x = 0xFF
				}
				set(ScriptCameraPosition, ScriptX, x)
				set(ScriptCursorFlashUnit, ScriptCharacter, 2)
				set(ScriptBranchNoAliveAlly, ScriptCharacter, 3)
			}
			set(ScriptMusic, ScriptSong, 0x32)
			set(ScriptJoinPlayer, ScriptCharacter, 2)
			set(ScriptMoveUnit, ScriptX, -1)
			if fixture.name == "FE8U" || fixture.name == "FE8J" {
				set(ScriptMoveUnit, ScriptSpeed, -1)
				rewardSlot := slices.IndexFunc(
					d.Instructions(),
					func(row ScriptInstruction) bool {
						return row.Op == ScriptSetSlot && row.Raw[2] == 3
					},
				)
				if err := d.Set(rewardSlot, ScriptSlotValue, 2); err != nil {
					t.Fatal(err)
				}
				set(ScriptGiveItemSlot, ScriptCharacter, -3)
			} else {
				set(ScriptGiveItem, ScriptItem, 2)
			}
			plan, err := s.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Relocated() || len(plan.Uses()) < 2 ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal(
					"parameter preview must preserve ROM and describe aliases",
				)
			}
			set(ScriptMusic, ScriptSong, 0x33)
			if err := s.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			expected := bytes.Clone(before.Data)
			copy(expected[root:], changed)
			if !bytes.Equal(expected, s.Document.Snapshot().Data) {
				t.Fatal("scene operand bytes or immutable plan")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("scene undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(expected, s.Document.Snapshot().Data) {
				t.Fatal("scene redo")
			}

			d, err = s.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			moveIndex := sceneInstruction(t, d, ScriptMoveUnit)
			if value, _ := d.Value(moveIndex, ScriptX); value != -1 {
				t.Fatal("signed destination was lost")
			}
			last := len(d.Instructions()) - 1
			if err := d.Insert(last, ScriptJoinNPC); err != nil {
				t.Fatal(err)
			}
			if err := d.Set(last, ScriptCharacter, 3); err != nil {
				t.Fatal(err)
			}
			// The fixed native bytes below cover each faction encoding, separately
			// from the parameter edits above and from the decoder used after saving.
			npc := []byte{0x23, 0x34, 3, 0}
			const endLength = 4
			switch fixture.name {
			case "FE6":
				npc = []byte{0x2B, 0, 0, 0, 3, 0, 0, 0, 0x40, 0, 0, 0}
			case "FE7J":
				npc = []byte{0x63, 0, 0, 0, 3, 0, 0, 0, 0x40, 0, 0, 0}
			case "FE7U":
				npc = []byte{0x65, 0, 0, 0, 3, 0, 0, 0, 0x40, 0, 0, 0}
			}
			packed := bytes.Clone(changed[:len(changed)-endLength])
			packed = append(packed, npc...)
			packed = append(packed, changed[len(changed)-endLength:]...)
			plan, err = s.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if !plan.Relocated() {
				t.Fatal("scene insertion must append")
			}
			if err := s.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			if !bytes.Equal(
				after.Data[plan.Destination():plan.Destination()+plan.Length()],
				packed,
			) {
				t.Fatal("relocated scene bytes")
			}
			preserved := bytes.Clone(after.Data[:len(expected)])
			copy(preserved[site:site+4], expected[site:site+4])
			if !bytes.Equal(preserved, expected) {
				t.Fatal(
					"relocation changed old resources or an independent reference",
				)
			}
			alias, err := s.Script(0, f.GetPosition("script_alias_site"))
			if err != nil || alias.Entry.Address != root {
				t.Fatal("independent alias moved")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(expected, s.Document.Snapshot().Data) {
				t.Fatal("structural scene undo")
			}
			if !s.Document.Redo() ||
				!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("structural scene redo")
			}
			path := filepath.Join(t.TempDir(), "scene.gba")
			if err := s.Document.SaveAs(path, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after.Data, reopened.Document.Snapshot().Data) {
				t.Fatal("scene changed on save")
			}
			d, err = reopened.NewScriptDraft(0, site)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Remove(
				sceneInstruction(t, d, ScriptJoinNPC),
			); err != nil {
				t.Fatal(err)
			}
			continued, err := reopened.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyScriptEdit(continued); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(
				reopened.Document.Snapshot().Data[continued.Destination():continued.Destination()+continued.Length()],
				changed,
			) {
				t.Fatal("continued scene editing")
			}
		})
	}
}

func TestScriptSceneGuards(t *testing.T) {
	for _, fixture := range gameFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			f, _, _ := sceneFixture(t, fixture.name)
			s := openFixture(t, f)
			before := s.Document.Snapshot()
			d, err := s.NewScriptDraft(0, f.GetPosition("script_site"))
			if err != nil {
				t.Fatal(err)
			}
			index := sceneInstruction(t, d, ScriptMoveUnit)
			rows := d.Instructions()
			for _, value := range []int64{-0x8001, 0x8000} {
				if err := d.Set(
					index,
					ScriptX,
					value,
				); !errors.Is(
					err,
					ErrValue,
				) {
					t.Fatalf("out of range coordinate: %v", err)
				}
				if !bytes.Equal(rows[index].Raw, d.Instructions()[index].Raw) {
					t.Fatal("invalid draft edit changed bytes")
				}
			}
			fade := sceneInstruction(t, d, ScriptFadeToBlack)
			for _, speed := range []int64{0, 0x8000, 0xFFFF} {
				if err := d.Set(
					fade,
					ScriptSpeed,
					speed,
				); !errors.Is(
					err,
					ErrValue,
				) {
					t.Fatal(
						"zero or negative native fade speed must not create a stalled fade",
					)
				}
			}
			if err := d.Set(index, ScriptX, 4); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Document.ApplyEdit(
				before.Revision,
				"external edit",
				rom.Edit{
					Size:    len(before.Data),
					Changes: []rom.Change{{Offset: 0x100, Data: []byte{0x51}}},
				},
			); err != nil {
				t.Fatal(err)
			}
			conflicted := s.Document.Snapshot()
			if err := s.ApplyScriptEdit(plan); !errors.Is(err, rom.ErrStale) {
				t.Fatalf("stale scene preview: %v", err)
			}
			if !bytes.Equal(conflicted.Data, s.Document.Snapshot().Data) {
				t.Fatal("failed scene apply changed document")
			}
		})
	}
}
