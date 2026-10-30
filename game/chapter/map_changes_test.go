package chapter_test

import (
	"bytes"
	"errors"
	"image"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestMapChangesNativeTables(t *testing.T) {
	for _, version := range []string{"FE6", "FE7J", "FE7U", "FE8J", "FE8U"} {
		t.Run(version, func(t *testing.T) {
			for _, terminal := range []byte{0x80, 0xFE, 0xFF} {
				f := testutil.NewMapChangesFixture(version)
				f.WriteU8(0x238418, terminal)
				s := testutil.Open(t, f)
				d, err := s.Maps.NewChangesDraft(0)
				if err != nil {
					t.Fatal(err)
				}
				if len(d.Entries()) != 2 {
					t.Fatal("signed end marker")
				}
				if err = d.Stroke(
					0,
					[]image.Point{{X: 1, Y: 2}},
					4,
				); err != nil {
					t.Fatal(err)
				}
				plan, err := s.Maps.PlanChanges(d)
				if err != nil {
					t.Fatal(err)
				}
				if err = s.Maps.ApplyChanges(plan); err != nil {
					t.Fatal(err)
				}
				if s.Document.Snapshot().Data[plan.Destination()+24] != terminal {
					t.Fatal("terminal bytes not preserved")
				}
			}
			f := testutil.NewMapChangesFixture(version)
			f.WriteU8(0x23840C, 3)
			s := testutil.Open(t, f)
			d, err := s.Maps.NewChangesDraft(0)
			if err != nil {
				t.Fatal(err)
			}
			script := mapChangesScript(t, s, f.GetPosition("deploy_script"))
			position := 1
			if version == "FE8J" || version == "FE8U" {
				position = 2
			}
			links, err := s.Events.MapChanges(script, position)
			if err != nil || len(links) != 1 || links[0].Address != 0x238400 {
				t.Fatal(
					"last rectangle must resolve ID to first record",
					links,
					err,
				)
			}
			if err = d.Delete(0); err != nil {
				t.Fatal(err)
			}
			plan, err := s.Maps.PlanChanges(d)
			if err != nil || !plan.Applicable() {
				t.Fatal(err)
			}
			for _, ref := range plan.References().References {
				if !ref.Retargeted || ref.ID != 3 || ref.AfterID != 3 {
					t.Fatal("duplicate-ID retarget omitted", ref)
				}
			}
			if err = s.Maps.ApplyChanges(plan); err != nil {
				t.Fatal(err)
			}
			if got := s.Document.Snapshot().Data[plan.Destination() : plan.Destination()+12]; !bytes.Equal(
				got,
				f.Bytes()[0x23840C:0x238418],
			) {
				t.Fatal("remaining record rewritten")
			}
		})
	}
	t.Run("short terminal", func(t *testing.T) {
		f := testutil.NewMapChangesFixture("FE8U")
		end := len(f.Bytes()) - 4
		f.WritePointer(0x221000+9*4, end)
		f.WriteAt(end, []byte{0x80, 0x5A, 0xA5, 0xEF})
		s := testutil.Open(t, f)
		d, err := s.Maps.NewChangesDraft(0)
		if err != nil {
			t.Fatal(err)
		}
		if err = d.Add(3, image.Rect(0, 0, 1, 1)); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Maps.PlanChanges(d)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Maps.ApplyChanges(plan); err != nil {
			t.Fatal(err)
		}
		if got := s.Document.Snapshot().Data[plan.Destination()+12 : plan.Destination()+16]; !bytes.Equal(
			got,
			[]byte{0x80, 0x5A, 0xA5, 0xEF},
		) {
			t.Fatal("short terminal changed")
		}
	})
	t.Run("zero resource ID", func(t *testing.T) {
		f := testutil.NewMapChangesFixture("FE8U")
		f.WriteU8(f.GetPosition("chapters")+11, 0)
		f.WritePointer(0x221000, 0x238400)
		s := testutil.Open(t, f)
		d, err := s.Maps.NewChangesDraft(0)
		if err != nil || len(d.Entries()) != 2 {
			t.Fatal(err)
		}
		if err = d.Stroke(0, []image.Point{{X: 1, Y: 2}}, 4); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Maps.PlanChanges(d)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Pointer() != 0x221000 {
			t.Fatal("wrong zero-ID entry")
		}
		if err = s.Maps.ApplyChanges(plan); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("unreadable payload", func(t *testing.T) {
		f := testutil.NewMapChangesFixture("FE8U")
		f.WriteU32(0x238408, 0x08000000)
		s := testutil.Open(t, f)
		before := s.Document.Snapshot()
		if _, err := s.Maps.NewChangesDraft(
			0,
		); !errors.Is(
			err,
			maps.ErrMapChanges,
		) {
			t.Fatal(err)
		}
		if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
			t.Fatal("bad payload changed ROM")
		}
	})
}

func TestMapChangesRuntimeContexts(t *testing.T) {
	for _, version := range []string{"FE6", "FE7J", "FE7U"} {
		t.Run(version, func(t *testing.T) {
			f := testutil.NewMapChangesFixture(version)
			s := testutil.Open(t, f)
			d := mapChangesScript(t, s, f.GetPosition("deploy_script"))
			if err := d.Set(1, event.ScriptX, 0); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Events.MapChanges(
				d,
				1,
			); !errors.Is(
				err,
				event.ErrMapChangeContext,
			) {
				t.Fatal("coordinate fallback falsely resolved", err)
			}
			plan, err := s.Events.PlanScriptEdit(d)
			if err != nil {
				t.Fatal("native runtime parameter fallback rejected", err)
			}
			if err = s.Events.ApplyScriptEdit(plan); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("halfword queue IDs", func(t *testing.T) {
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
				0x12,
				0x34,
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
		s := testutil.Open(t, f)
		d := mapChangesScript(t, s, at)
		links, err := s.Events.MapChanges(d, 3)
		if err != nil || len(links) != 1 || links[0].ID != 3 {
			t.Fatal(links, err)
		}
		if err = d.Set(1, event.ScriptSlotValue, 0x12340007); err != nil {
			t.Fatal(err)
		}
		plan, err := s.Events.PlanScriptEdit(d)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Events.ApplyScriptEdit(plan); err != nil {
			t.Fatal(err)
		}
	})
	for _, restore := range []bool{false, true} {
		t.Run(
			map[bool]string{false: "call barrier", true: "explicit map load"}[restore],
			func(t *testing.T) {
				f := testutil.NewMapChangesFixture("FE8U")
				at := f.GetPosition("deploy_script")
				raw := []byte{0x40, 0x0A, 0, 0, 0, 0, 0, 0}
				position := 1
				if restore {
					raw = append(raw, 0x20, 0x25, 1, 0)
					position = 2
				}
				raw = append(raw, 0x20, 0x27, 3, 0, 0x20, 1, 0, 0)
				f.WriteAt(at, raw)
				f.WritePointer(at+4, at+0x100)
				f.WriteAt(at+0x100, []byte{0x20, 1, 0, 0})
				s := testutil.Open(t, f)
				d := mapChangesScript(t, s, at)
				links, err := s.Events.MapChanges(d, position)
				if restore {
					if err != nil || len(links) != 1 || links[0].Chapter != 1 {
						t.Fatal(links, err)
					}
				} else if !errors.Is(err, event.ErrMapChangeContext) {
					t.Fatal("call retained chapter assumption", links, err)
				}
			},
		)
	}
}
