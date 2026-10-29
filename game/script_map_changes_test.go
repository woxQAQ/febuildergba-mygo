package game

import (
	"bytes"
	"errors"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func mapChangesScript(t *testing.T, s *Session, address int) *ScriptDraft {
	t.Helper()
	entries, err := s.ScriptEntries(0)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Address == address {
			d, err := s.NewScriptDraft(0, entry.Pointer)
			if err != nil {
				t.Fatal(err)
			}
			return d
		}
	}
	t.Fatal("script entry not discovered")
	return nil
}

func TestMapChangeScriptWorkflow(t *testing.T) {
	for _, version := range []string{"FE6", "FE7J", "FE7U", "FE8J", "FE8U"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewMapChangesFixture(version)
			s := openFixture(t, f)
			before := s.Document.Snapshot()
			d := mapChangesScript(t, s, f.GetPosition("deploy_script"))
			links, err := s.ScriptMapChanges(d, 0)
			if err != nil || len(links) != 1 || links[0].ID != 3 ||
				links[0].Address != 0x238400 {
				t.Fatal("direct link", links, err)
			}
			position := 1
			if version == "FE8J" || version == "FE8U" {
				position = 2
			}
			links, err = s.ScriptMapChanges(d, position)
			if err != nil || len(links) != 1 || links[0].ID != 7 {
				t.Fatal("last coordinate match", links, err)
			}
			if err = d.Set(0, ScriptMapChangeID, 4); err != nil {
				t.Fatal(err)
			}
			if _, err = s.PlanScriptEdit(d); !errors.Is(err, ErrMapChanges) {
				t.Fatal("missing target accepted", err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("invalid reference changed ROM")
			}
			if err = d.Set(0, ScriptMapChangeID, 7); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			want := []byte{0x20, 0x27, 7, 0}
			switch version {
			case "FE6":
				want = []byte{0x29, 0, 0, 0, 7, 0, 0, 0}
			case "FE7J":
				want = []byte{0x5D, 0, 7, 0}
			case "FE7U":
				want = []byte{0x5F, 0, 7, 0}
			}
			after := s.Document.Snapshot()
			at := f.GetPosition("deploy_script")
			if !bytes.Equal(after.Data[at:at+len(want)], want) {
				t.Fatal("native operand encoding")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				!s.Document.Redo() ||
				!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("script history")
			}
			if err = s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			d = mapChangesScript(t, reopened, at)
			if err = d.Insert(
				len(d.Instructions())-1,
				ScriptMapChange,
			); err != nil {
				t.Fatal(err)
			}
			if err = d.Set(
				len(d.Instructions())-2,
				ScriptMapChangeID,
				3,
			); err != nil {
				t.Fatal(err)
			}
			plan, err = reopened.PlanScriptEdit(d)
			if err != nil || !plan.Relocated() {
				t.Fatal("structural script change", err)
			}
			if err = reopened.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMapChangeScriptContexts(t *testing.T) {
	t.Run("shared chapter target", func(t *testing.T) {
		f := testutil.NewMapChangesFixture("FE8U")
		f.WritePointer(0x221000+10*4, 0x238600)
		f.WriteAt(0x238600, []byte{3, 0, 0, 1, 1})
		f.WritePointer(0x238608, 0x238800)
		f.WriteU8(0x23860C, 0xFF)
		s := openFixture(t, f)
		d := mapChangesScript(t, s, f.GetPosition("deploy_script"))
		if err := d.Set(0, ScriptMapChangeID, 7); err != nil {
			t.Fatal(err)
		}
		if _, err := s.PlanScriptEdit(d); !errors.Is(err, ErrMapChanges) {
			t.Fatal("shared chapter missing target", err)
		}
	})
	t.Run("slot predecessor changes target", func(t *testing.T) {
		f := testutil.NewMapChangesFixture("FE8U")
		s := openFixture(t, f)
		d := mapChangesScript(t, s, f.GetPosition("deploy_script"))
		if err := d.Set(1, ScriptSlotValue, 0x00080005); err != nil {
			t.Fatal(err)
		}
		if _, err := s.PlanScriptEdit(d); !errors.Is(err, ErrMapChanges) {
			t.Fatal("unchanged consumer not revalidated", err)
		}
	})
	t.Run("queue provenance", func(t *testing.T) {
		f := testutil.NewMapChangesFixture("FE8U")
		at := f.GetPosition("deploy_script")
		f.WriteAt(
			at,
			[]byte{
				0x40,
				5,
				0x0D,
				0,
				0,
				0,
				0,
				0,
				0x40,
				5,
				1,
				0,
				3,
				0,
				0,
				0,
				0x21,
				7,
				0,
				0,
				0x40,
				5,
				1,
				0,
				7,
				0,
				0,
				0,
				0x21,
				7,
				0,
				0,
				0x20,
				0x27,
				0xFD,
				0xFF,
				0x20,
				1,
				0,
				0,
			},
		)
		s := openFixture(t, f)
		d := mapChangesScript(t, s, at)
		links, err := s.ScriptMapChanges(d, 5)
		if err != nil || len(links) != 2 || links[0].ID != 3 ||
			links[1].ID != 7 {
			t.Fatal(links, err)
		}
		if err = d.Set(1, ScriptSlotValue, 4); err != nil {
			t.Fatal(err)
		}
		if _, err = s.PlanScriptEdit(d); !errors.Is(err, ErrMapChanges) {
			t.Fatal("queue target not validated", err)
		}
	})
	for _, version := range []string{"FE7J", "FE7U"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewMapChangesFixture(version)
			at := f.GetPosition("deploy_script")
			first := byte(0x5D)
			if version == "FE7U" {
				first = 0x5F
			}
			f.WriteAt(
				at,
				[]byte{
					first,
					0,
					7,
					0x80,
					first + 2,
					0,
					3,
					0,
					first + 3,
					0,
					7,
					0x80,
					first + 4,
					0,
					1,
					0,
					first + 5,
					0,
					3,
					0,
					first,
					0,
					0xFF,
					0xFF,
					0,
					0,
					0,
					0,
				},
			)
			s := openFixture(t, f)
			d := mapChangesScript(t, s, at)
			ops := []ScriptOp{
				ScriptMapChange,
				ScriptMapChangeInstant,
				ScriptMapChangeQuiet,
				ScriptMapRefresh,
				ScriptMapChangeWater,
				ScriptMapChangeParameter,
				ScriptEnd,
			}
			for index, row := range d.Instructions() {
				if row.Op != ops[index] || !row.Editable {
					t.Fatal(index, row.Op)
				}
			}
			if value, err := d.Value(
				0,
				ScriptMapRemovePrevious,
			); err != nil ||
				value != 1 {
				t.Fatal("FE7 high bit", value, err)
			}
			if err := d.Set(0, ScriptMapChangeID, 3); err != nil {
				t.Fatal(err)
			}
			plan, err := s.PlanScriptEdit(d)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
			if got := s.Document.Snapshot().Data[at : at+4]; !bytes.Equal(
				got,
				[]byte{first, 0, 3, 0x80},
			) {
				t.Fatal("neighbor flag lost", got)
			}
		})
	}
}
