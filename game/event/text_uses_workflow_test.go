package event_test

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func scriptTextUses(uses []text.Use) []text.Use {
	return slices.DeleteFunc(
		uses,
		func(use text.Use) bool { return use.Kind != event.TextUse },
	)
}

func TestScriptTextUsageWorkflow(t *testing.T) {
	for _, version := range testutil.Games() {
		t.Run(version.Name, func(t *testing.T) {
			f := testutil.NewScriptTextFixture(version.Name)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			var want []text.Use
			secondField := event.ScriptTextID
			if version.Name == "FE7J" || version.Name == "FE7U" {
				secondField = event.ScriptTextPrimary
			}
			for chapter := range 2 {
				for _, entry := range []struct {
					site   int
					source string
				}{{f.GetPosition("text_entry"), "script_ending"}, {f.GetPosition("text_alias"), "deploy_objective_event"}} {
					want = append(
						want,
						text.Use{
							Kind:    event.TextUse,
							ID:      chapter,
							Field:   event.ScriptTextID,
							Pointer: entry.site,
							Address: f.GetPosition("text_first"),
							Source:  entry.source,
						},
						text.Use{
							Kind:    event.TextUse,
							ID:      chapter,
							Field:   secondField,
							Pointer: entry.site,
							Address: f.GetPosition("text_second"),
							Source:  entry.source,
						},
					)
				}
			}
			uses, complete := s.Texts.Uses(1)
			if !complete || !slices.Equal(scriptTextUses(uses), want) {
				t.Fatalf(
					"direct operands and shared entry contexts: complete=%v, uses=%+v, want=%+v",
					complete,
					uses,
					want,
				)
			}
			for _, use := range want {
				program, err := s.Events.Script(use.ID, use.Pointer)
				if err != nil ||
					!slices.ContainsFunc(
						program.Instructions,
						func(row event.ScriptInstruction) bool { return row.Address == use.Address },
					) {
					t.Fatal(
						"usage does not navigate to its instruction",
						use,
						err,
					)
				}
			}
			uses[0].Address = 0
			fresh, _ := s.Texts.Uses(1)
			if !slices.Equal(scriptTextUses(fresh), want) ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("usage query changed its cache or ROM")
			}
			childUses, complete := s.Events.TextUses(2)
			child := text.Use{
				Kind:    event.TextUse,
				ID:      0,
				Field:   event.ScriptTextID,
				Pointer: f.GetPosition("text_call") + 4,
				Address: f.GetPosition("script_child"),
				Source:  "script_call_entry",
			}
			if !complete || !slices.Contains(childUses, child) {
				t.Fatal("static call text missing", childUses)
			}
			if secondField == event.ScriptTextPrimary {
				other := want[1]
				other.Field = event.ScriptTextHector
				if !slices.Contains(childUses, other) {
					t.Fatal(
						"FE7 route-specific text operand missing",
						childUses,
					)
				}
			}

			file, err := s.Texts.File()
			if err != nil {
				t.Fatal(err)
			}
			imported, err := file.Import([]byte("[0001]\r\nAA\r\n"))
			if err != nil {
				t.Fatal(err)
			}
			filePlan, err := s.Texts.PlanImport(imported)
			if err != nil || filePlan.Entries()[0].UsesIncomplete ||
				!slices.Equal(
					scriptTextUses(filePlan.Entries()[0].Uses),
					want,
				) {
				t.Fatal("file preview lost script usage", err)
			}
			plan, err := s.Texts.PlanID(1, "AA")
			if err != nil || !plan.UsesComplete() ||
				!slices.Equal(scriptTextUses(plan.Uses()), want) {
				t.Fatal("text preview lost confirmed script uses", err)
			}
			copy := plan.Uses()
			copy[len(copy)-1].Address = 0
			if !slices.Equal(scriptTextUses(plan.Uses()), want) {
				t.Fatal("text preview aliases its caller")
			}
			if err := s.Texts.Apply(plan); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(
				before.Data[f.GetPosition("script_root"):f.GetPosition("script_root")+64],
				s.Document.Snapshot().Data[f.GetPosition("script_root"):f.GetPosition("script_root")+64],
			) {
				t.Fatal("text replacement rewrote the referencing script")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("text replacement did not undo atomically")
			}
			draft, err := s.Events.NewScriptDraft(
				0,
				f.GetPosition("text_entry"),
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := draft.Set(0, event.ScriptTextID, 2); err != nil {
				t.Fatal(err)
			}
			edit, err := s.Events.PlanScriptEdit(draft)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyScriptEdit(edit); err != nil {
				t.Fatal(err)
			}
			changed := s.Document.Snapshot()
			remaining, _ := s.Texts.Uses(1)
			wantRemaining := slices.DeleteFunc(
				slices.Clone(want),
				func(use text.Use) bool { return use.Address == f.GetPosition("text_first") },
			)
			if !slices.Equal(scriptTextUses(remaining), wantRemaining) ||
				plan.Current(s.Texts) {
				t.Fatal("script edit did not invalidate text usage and preview")
			}
			if err := s.Texts.Apply(
				plan,
			); !errors.Is(err, rom.ErrStale) ||
				!bytes.Equal(changed.Data, s.Document.Snapshot().Data) {
				t.Fatal("stale preview wrote partial bytes", err)
			}
			if !s.Document.Undo() {
				t.Fatal("script undo")
			}
			restored, _ := s.Texts.Uses(1)
			if !slices.Equal(scriptTextUses(restored), want) ||
				!s.Document.Redo() {
				t.Fatal("undo query or redo")
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			saved, _ := reopened.Texts.Uses(1)
			if !slices.Equal(scriptTextUses(saved), wantRemaining) ||
				!bytes.Equal(changed.Data, reopened.Document.Snapshot().Data) {
				t.Fatal("save and reopen lost text usage")
			}
		})
	}
}

func TestScriptTextUsageLayoutAndDocumentIsolation(t *testing.T) {
	for _, version := range testutil.Games() {
		t.Run(version.Name, func(t *testing.T) {
			f := testutil.NewScriptTextFixture(version.Name)
			s := testutil.Open(t, f)
			table := s.Records.Table(records.Chapters)
			const manual, header = 0x24F000, 0x24F400
			f.WriteAt(
				manual,
				f.Bytes()[table.Address:table.Address+table.Stride],
			)
			eventOffset, ending := 116, 76
			switch version.Name {
			case "FE7U":
				eventOffset, ending = 120, 60
			case "FE7J":
				ending = 60
			case "FE6":
				eventOffset, ending = 58, 24
			}
			f.WriteU8(manual+eventOffset, 3)
			f.WritePointer(f.GetPosition("deploy_list")+12, header)
			f.WriteAt(header, make([]byte, 80))
			f.WritePointer(header+ending, f.GetPosition("script_child"))
			s = testutil.Open(t, f)
			original, _ := s.Texts.Uses(1)
			plan, err := s.Texts.PlanID(1, "AA")
			if err != nil {
				t.Fatal(err)
			}
			other := testutil.Open(t, f)
			if err := other.Texts.Apply(plan); !errors.Is(err, rom.ErrStale) {
				t.Fatal("foreign preview", err)
			}
			before := s.Document.Snapshot()
			if err := s.Records.UseLayout(
				records.Chapters,
				records.Layout{Address: manual, Stride: table.Stride, Count: 1},
			); err != nil {
				t.Fatal(err)
			}
			uses, complete := s.Texts.Uses(1)
			if !complete || len(scriptTextUses(uses)) != 0 ||
				plan.Current(s.Texts) {
				t.Fatal("manual layout ignored", uses, complete)
			}
			if err := s.Texts.Apply(
				plan,
			); !errors.Is(err, rom.ErrStale) ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("layout preview changed ROM", err)
			}
			s.Records.UseAutomaticLayout(records.Chapters)
			restored, _ := s.Texts.Uses(1)
			if !slices.Equal(original, restored) {
				t.Fatal("automatic layout did not restore text usage")
			}
		})
	}
}

func TestScriptTextUsagePartialDiscovery(t *testing.T) {
	for _, scenario := range []string{"dynamic text", "unknown boundary", "opaque ASM", "dynamic call", "damaged conditions", "cycle", "instruction limit"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.NewScriptTextFixture("FE8U")
			root := f.GetPosition("script_root")
			switch scenario {
			case "dynamic text":
				f.WriteU16(root+2, 0x8000)
			case "unknown boundary":
				f.WriteAt(
					f.GetPosition("text_call"),
					[]byte{0xFF, 0xFF, 0xFF, 0xFF},
				)
			case "opaque ASM":
				f.WriteAt(
					f.GetPosition("text_call"),
					[]byte{0x40, 0x0D, 0, 0, 0, 0, 0, 2},
				)
			case "dynamic call":
				f.WriteU32(f.GetPosition("text_call")+4, 0xFFFFFFFF)
			case "damaged conditions":
				at := len(f.Bytes()) - 12
				f.WriteAt(
					at,
					f.Bytes()[f.GetPosition("objective_table"):f.GetPosition("objective_table")+12],
				)
				f.WritePointer(f.GetPosition("deploy_header")+12, at)
			case "cycle":
				f.WritePointer(f.GetPosition("text_call")+4, root)
			case "instruction limit":
				f.WriteAt(root, bytes.Repeat([]byte{0x20, 0x1B, 1, 0}, 8192))
			}
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			uses, complete := s.Events.TextUses(1)
			if complete != (scenario == "cycle") || len(uses) == 0 {
				t.Fatal(
					"partial scope or surviving direct operands",
					complete,
					len(uses),
				)
			}
			if scenario == "dynamic text" &&
				slices.ContainsFunc(
					uses,
					func(use text.Use) bool { return use.Address == root },
				) {
				t.Fatal("dynamic text was mistaken for a literal")
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("discovery mutated ROM")
			}
			draft, err := s.Events.NewScriptDraft(
				0,
				f.GetPosition("text_entry"),
			)
			if err != nil {
				t.Fatal(err)
			}
			operands := draft.TextOperands(1)
			if len(operands) != 1 || operands[0].ID != 1 {
				t.Fatal("partial script lost readable text operand", operands)
			}
			if !draft.Complete() {
				if len(draft.Fields(1)) != 0 ||
					draft.Set(1, event.ScriptTextID, 2) == nil {
					t.Fatal("readable reference enabled partial script writes")
				}
			}
			if scenario == "dynamic text" {
				plan, err := s.Texts.PlanID(1, "AA")
				if err != nil || plan.UsesComplete() {
					t.Fatal("partial usage blocked editing or lost scope", err)
				}
				if err := s.Texts.Apply(plan); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestWorldMapScriptTextUsage(t *testing.T) {
	for _, version := range testutil.Games() {
		t.Run(version.Name, func(t *testing.T) {
			var f *testutil.FixtureBuilder
			id, address := uint16(1), 0
			switch version.Name {
			case "FE6":
				f = testutil.NewFE6WorldMapFixture(t)
				address = f.GetPosition("fe6_wm_text")
			case "FE7J", "FE7U":
				f = testutil.NewFE7WorldMapFixture(t, version.Name)
				address = f.GetPosition("fe7_wm_text")
			default:
				f = testutil.NewWorldMapSceneFixture(t, version.Name)
				id, address = 3, f.GetPosition("wm_scene")+56
				f.WritePointer(
					f.GetPosition("wm_intro_root"),
					f.GetPosition("wm_beginning"),
				)
			}
			s := testutil.Open(t, f)
			uses, _ := s.Texts.Uses(id)
			var found []text.Use
			for _, use := range uses {
				if use.Kind == event.TextUse &&
					use.ID == event.WorldMapScripts &&
					use.Address == address {
					found = append(found, use)
				}
				if use.Kind == event.TextUse && use.Address == address &&
					use.ID != event.WorldMapScripts {
					t.Fatal(
						"world-map usage duplicated under battle-map context",
						use,
					)
				}
			}
			if len(found) < 2 {
				t.Fatal(
					"world-map aliases or compound text ID missing",
					id,
					address,
					uses,
				)
			}
			if version.Name == "FE8U" || version.Name == "FE8J" {
				first := found[0]
				if !slices.ContainsFunc(
					found,
					func(use text.Use) bool { return use.Pointer == first.Pointer && use.Source != first.Source },
				) {
					t.Fatal(
						"shared physical directory lost its distinct roles",
						found,
					)
				}
			}
		})
	}
}
