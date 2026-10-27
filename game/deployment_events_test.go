package game

import (
	"bytes"
	"slices"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestDeploymentEventDiscoveryWorkflow(t *testing.T) {
	f := testutil.NewDeploymentFixture("FE8U")
	const script = 0x242000
	units := f.GetPosition("deploy_units")
	// An unused pointer-valued SVAL is not a deployment. A native LOAD helper
	// consumes the independently stored slot 2/3 pointer operands that follow.
	f.WriteAt(script, []byte{0x40, 5, 1, 0})
	f.WritePointer(script+4, units)
	f.WriteAt(script+8, []byte{0x40, 5, 2, 0})
	f.WritePointer(script+12, units)
	f.WriteAt(script+16, []byte{0x40, 5, 3, 0})
	f.WritePointer(script+20, units)
	f.WriteAt(script+24, []byte{0x40, 0x0A, 0, 0})
	f.WritePointer(script+28, 0x9EE248)
	// Loads after switching to another map are omitted until this chapter is
	// selected again. The values are chapter IDs, not resource IDs.
	f.WriteAt(script+32, []byte{0x20, 0x25, 1, 0, 0x40, 0x2C, 1, 0})
	f.WritePointer(script+40, units)
	f.WriteAt(script+44, []byte{0x20, 0x25, 0, 0, 0x40, 0x2C, 1, 0})
	f.WritePointer(script+52, units)
	// An early return has a forward conditional target. Continue decoding to
	// its label without treating dead bytes as a new event resource.
	f.WriteAt(
		script+56,
		[]byte{
			0x40,
			0x0C,
			7,
			0,
			0,
			0,
			0,
			0,
			0x20,
			1,
			0,
			0,
			0x20,
			8,
			7,
			0,
			0x40,
			0x2C,
			1,
			0,
		},
	)
	f.WritePointer(script+76, units)
	f.WriteAt(script+80, []byte{0x20, 1, 0, 0})
	s := openFixture(t, f)
	result, err := s.ChapterDeployments(0)
	if err != nil || len(result.Warnings) != 0 {
		t.Fatalf("typed opening discovery: %+v, %v", result, err)
	}
	var sites []int
	for _, group := range result.Groups {
		if group.Label == "deploy_opening" {
			sites = append(sites, group.Pointer)
		}
	}
	if !slices.Equal(
		sites,
		[]int{script + 12, script + 20, script + 52, script + 76},
	) {
		t.Fatalf("incorrect typed/static event references: %X", sites)
	}
	before := s.Document.Snapshot()
	draft, err := s.NewDeploymentDraft(0, script+12)
	if err != nil {
		t.Fatal(err)
	}
	if err := draft.Set(0, "deploy_level", 6); err != nil {
		t.Fatal(err)
	}
	plan, err := s.PlanDeploymentEdit(draft)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyDeploymentEdit(plan); err != nil {
		t.Fatal(err)
	}
	result, err = s.ChapterDeployments(0)
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range result.Groups {
		if group.Pointer == script+12 {
			if group.Units[0].Level != 6 ||
				group.Address != plan.Destination() {
				t.Fatal("SVAL pointer site did not update")
			}
		} else if len(group.Units) != 0 && group.Units[0].Level != 13 {
			t.Fatal(
				"unselected script operands changed with their original alias",
			)
		}
	}
	if !s.Document.Undo() ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
		t.Fatal("event-loaded deployment did not undo atomically")
	}
}

func TestDeploymentEventFailureIsolation(t *testing.T) {
	for _, failure := range []string{"unknown opcode", "recursive call", "dynamic load"} {
		t.Run(failure, func(t *testing.T) {
			f := testutil.NewDeploymentFixture("FE8U")
			script := f.GetPosition("deploy_script")
			switch failure {
			case "unknown opcode":
				f.WriteAt(script+8, []byte{0xFF, 0xFF, 0xFF, 0xFF})
			case "recursive call":
				f.WriteAt(script+8, []byte{0x40, 0x0A, 0, 0})
				f.WritePointer(script+12, script)
				f.WriteAt(script+16, []byte{0x20, 1, 0, 0})
			case "dynamic load":
				f.WriteAt(
					script+8,
					[]byte{
						0x40,
						0x2C,
						1,
						0,
						0xFF,
						0xFF,
						0xFF,
						0xFF,
						0x20,
						1,
						0,
						0,
					},
				)
			}
			s := openFixture(t, f)
			result, err := s.ChapterDeployments(0)
			if err != nil || len(result.Warnings) == 0 ||
				result.Groups[0].Err != nil ||
				!slices.ContainsFunc(
					result.Groups,
					func(group DeploymentGroup) bool {
						return group.Pointer == script+4 && group.Err == nil
					},
				) {
				t.Fatalf(
					"partial event discovery lost independent groups: %+v, %v",
					result,
					err,
				)
			}
			draft, err := s.NewDeploymentDraft(0, result.Groups[0].Pointer)
			if err != nil {
				t.Fatal(err)
			}
			if err := draft.Set(0, "deploy_level", 4); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanDeploymentEdit(draft)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyDeploymentEdit(plan); err != nil {
				t.Fatal(err)
			}
		})
	}
}
