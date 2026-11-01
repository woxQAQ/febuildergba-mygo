package game_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// The same bytes can decode as a script and another resource. A successful
// script decode must not authorize overwriting that other resource's claim.
func TestScriptParametersRespectSessionClaims(t *testing.T) {
	for _, tc := range []struct {
		name    string
		address int
		protect func(*testutil.FixtureBuilder, int)
	}{
		{"quote", 0, func(f *testutil.FixtureBuilder, at int) {
			f.WritePointer(0x846AC, at)
			f.WriteU16(at+16, 0xFFFF)
		}},
		{"support", 0, func(f *testutil.FixtureBuilder, at int) {
			f.WritePointer(f.GetPosition("units")+44, at)
		}},
		{"support_talk", 0, func(f *testutil.FixtureBuilder, at int) {
			f.WritePointer(0x84784, at)
			f.WritePointer(0x847FC, at)
			f.WriteU16(at+16, 0xFFFF)
		}},
		{"terrain_partial_overlap", 0, func(f *testutil.FixtureBuilder, at int) {
			f.WritePointer(f.GetPosition("classes")+84+56, at+3)
		}},
		{"terrain_unchanged_instruction", 0, func(f *testutil.FixtureBuilder, at int) {
			// The END instruction is unchanged, but the entire script range
			// must be free of conflicting ownership before any row is written.
			f.WritePointer(f.GetPosition("classes")+84+56, at+4)
		}},
		{"default_shop", 0, func(f *testutil.FixtureBuilder, at int) {
			f.WritePointer(0xB42B0, at)
		}},
		{"worldmap_node", 0, func(f *testutil.FixtureBuilder, at int) {
			f.WritePointer(0xBE84, at)
		}},
		{"itemicon_root", 0x36B4, nil},
		{"itemicon_palette_root", 0x35D0, nil},
		{"portrait_root", 0x5524, nil},
		{"condition_table", 0, func(f *testutil.FixtureBuilder, at int) {
			f.WritePointer(f.GetPosition("deploy_header"), at)
		}},
		{"deployment_records", 0, func(f *testutil.FixtureBuilder, at int) {
			f.WritePointer(f.GetPosition("deploy_first"), at)
		}},
		{"deployment_path", 0, func(f *testutil.FixtureBuilder, at int) {
			f.WritePointer(f.GetPosition("deploy_units")+8, at)
		}},
		{"chapter_inventory", 0, func(f *testutil.FixtureBuilder, at int) {
			const locations = 0x254000
			f.WritePointer(f.GetPosition("deploy_header")+8, locations)
			f.WriteAt(locations, []byte{10, 0, 0, 0, 0, 0, 0, 0, 1, 1, 0x16, 0})
			f.WritePointer(locations+4, at)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := testutil.NewScriptFixture("FE8U")
			at := tc.address
			if at == 0 {
				at = f.GetPosition("script_root")
			}
			f.WriteAt(at, make([]byte, 32))
			f.WriteAt(at, []byte{0x20, 0x0E, 5, 0, 0x20, 1, 0, 0})
			f.WritePointer(f.GetPosition("script_site"), at)
			if tc.protect != nil {
				tc.protect(f, at)
			}
			s := testutil.Open(t, f)
			before, state := s.Document.Snapshot(), s.Document.State()
			draft, err := s.Events.NewScriptDraft(
				0,
				f.GetPosition("script_site"),
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := draft.Set(0, event.ScriptFrames, 9); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanScriptEdit(draft)
			if plan != nil || !errors.Is(err, edit.ErrLayout) ||
				!errors.Is(err, space.ErrClaim) {
				t.Fatalf(
					"overlapping resource allowed a script write: plan=%v err=%v",
					plan,
					err,
				)
			}
			if state != s.Document.State() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal(
					"rejected script write changed bytes or document history",
				)
			}
		})
	}
}

func TestScriptCodeClaimsProtectOpeningAndAllowAdjacentPayload(t *testing.T) {
	f := testutil.NewDeploymentFixture("FE8U")
	at := f.GetPosition("deploy_script")
	// This opening has no deployment pointer and no condition-table alias.
	f.WriteAt(at, []byte{0x20, 0x0E, 5, 0, 0x20, 1, 0, 0})
	s := testutil.Open(t, f)
	before, state := s.Document.Snapshot(), s.Document.State()
	if _, err := s.Records.PlanTableMove(records.Items, []space.FreeRange{{
		Range:  space.Range{Start: at, End: at + 0x100},
		Source: "declared interval overlapping an opening script",
	}}); !errors.Is(err, space.ErrRange) {
		t.Fatalf("table move could overwrite opening instructions: %v", err)
	}
	if state != s.Document.State() ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
		t.Fatal("rejected move changed bytes or document history")
	}
	f.WritePointer(f.GetPosition("classes")+84+56, at+8)
	s = testutil.Open(t, f)
	before = s.Document.Snapshot()
	draft, err := s.Events.NewScriptDraft(0, f.GetPosition("deploy_header")+72)
	if err != nil {
		t.Fatal(err)
	}
	if err := draft.Set(0, event.ScriptFrames, 9); err != nil {
		t.Fatal(err)
	}
	plan, err := s.Events.PlanScriptEdit(draft)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Events.ApplyScriptEdit(plan); err != nil {
		t.Fatal(err)
	}
	expected := bytes.Clone(before.Data)
	expected[at+2] = 9
	if !bytes.Equal(expected, s.Document.Snapshot().Data) {
		t.Fatal(
			"opening parameter edit touched adjacent terrain payload or unknown bytes",
		)
	}
	if !s.Document.Undo() ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
		t.Fatal("opening edit undo")
	}
}

// The instruction bytes host operand sites. Repointing a called script or a
// deployment must remain possible without authorizing writes to other hosts.
func TestScriptOperandHostsAllowResourceEdits(t *testing.T) {
	for _, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.NewScriptFixture(fixture.Name)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			call := f.GetPosition("script_call") + 4
			draft, err := s.Events.NewScriptDraft(0, call)
			if err != nil {
				t.Fatal(err)
			}
			if err := draft.Insert(0, event.ScriptWait); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Events.PlanScriptEdit(draft)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			if target, ok := gba.Pointer(
				after.Data,
				call,
			); !ok ||
				target != plan.Destination() {
				t.Fatal("called script pointer was not updated")
			}
			expected := bytes.Clone(before.Data)
			copy(expected[call:call+4], after.Data[call:call+4])
			if !bytes.Equal(expected, after.Data[:len(expected)]) {
				t.Fatal(
					"called script relocation changed other instructions or old resources",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("called script relocation undo")
			}

			load := f.GetPosition("script_load") + 4
			units, err := s.Deployments.NewDraft(0, load)
			if err != nil {
				t.Fatal(err)
			}
			if err := units.Set(0, "deploy_level", 7); err != nil {
				t.Fatal(err)
			}
			deploymentPlan, err := s.Deployments.Plan(units)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Deployments.Apply(deploymentPlan); err != nil {
				t.Fatal(err)
			}
			after = s.Document.Snapshot()
			if target, ok := gba.Pointer(
				after.Data,
				load,
			); !ok ||
				target != deploymentPlan.Destination() {
				t.Fatal("hosted deployment pointer was not updated")
			}
			expected = bytes.Clone(before.Data)
			copy(expected[load:load+4], after.Data[load:load+4])
			if !bytes.Equal(expected, after.Data[:len(expected)]) {
				t.Fatal(
					"deployment relocation changed other instructions or old resources",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("hosted deployment undo")
			}
		})
	}
}
