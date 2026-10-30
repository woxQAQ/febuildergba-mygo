package chapter_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestMapResizeNativeChangeTables(t *testing.T) {
	for _, version := range []string{"FE6", "FE7J", "FE7U", "FE8J", "FE8U"} {
		t.Run(version, func(t *testing.T) {
			for _, mode := range []string{"80 terminator", "FE terminator", "FF terminator", "short terminator", "null entry", "entry zero", "entry zero conflict"} {
				t.Run(mode, func(t *testing.T) {
					f := testutil.NewMapResizeFixture(version)
					base, stride := f.GetPosition("chapters"), 148
					switch version {
					case "FE6":
						stride = 68
					case "FE7U":
						stride = 152
					}
					want := 2
					switch mode {
					case "80 terminator", "FE terminator", "FF terminator":
						marker := byte(0x80)
						switch mode {
						case "FE terminator":
							marker = 0xFE
						case "FF terminator":
							marker = 0xFF
						}
						f.WriteU8(f.GetPosition("resize_changes")+12, marker)
						f.WriteAt(
							f.GetPosition("resize_changes")+24,
							[]byte{3, 7, 12, 4, 4},
						)
						want = 1
					case "short terminator":
						at := len(f.Bytes()) - 4
						f.WritePointer(0x221000+9*4, at)
						f.WriteAt(at, []byte{0x80, 5, 8, 1})
						want = 0
					case "null entry":
						f.WriteU32(0x221000+9*4, 0)
						want = 0
					case "entry zero":
						f.WriteU8(base+11, 0)
						f.WriteU8(base+stride+11, 0)
						f.WritePointer(
							0x221000,
							f.GetPosition("resize_changes"),
						)
					case "entry zero conflict":
						f.WriteU8(base+8, 0)
						f.WriteU8(base+11, 0)
						f.WritePointer(
							f.GetPosition("map_layout_list"),
							f.GetPosition("map_layout"),
						)
						f.WritePointer(
							f.GetPosition("resize_changes_root"),
							f.GetPosition("map_layout_list"),
						)
					}
					s := testutil.Open(t, f)
					before := s.Document.Snapshot()
					d, err := s.Maps.NewDraft(0)
					if err != nil {
						t.Fatal(err)
					}
					if err = d.Resize(3, 4, maps.MapTopLeft, 0); err != nil {
						t.Fatal(err)
					}
					plan, err := s.Maps.Plan(d)
					if mode == "entry zero conflict" {
						if !errors.Is(err, edit.ErrLayout) ||
							!bytes.Equal(
								before.Data,
								s.Document.Snapshot().Data,
							) {
							t.Fatal(
								"cross-resource zero entry was writable",
								err,
							)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					count := 0
					for _, item := range plan.Impacts() {
						if item.Kind == maps.ChangeArea &&
							item.Chapter == 0 {
							count++
							if item.Index > 2 {
								t.Fatal(
									"data after native terminator was inspected",
									item,
								)
							}
						}
					}
					if count != want {
						t.Fatal("map-change discovery", count, want)
					}
					for _, warning := range plan.ImpactWarnings() {
						if warning.Kind == maps.ChangeArea {
							t.Fatal("valid table marked incomplete", warning)
						}
					}
				})
			}
		})
	}
}

func TestMapResizeScriptContext(t *testing.T) {
	for _, version := range []string{"FE6", "FE7J", "FE7U", "FE8J", "FE8U"} {
		t.Run(version, func(t *testing.T) {
			for _, mode := range []string{"call", "branch", "ASM", "unknown", "other map"} {
				t.Run(mode, func(t *testing.T) {
					f := testutil.NewMapResizeFixture(version)
					script := f.GetPosition("deploy_script")
					var barrier, move, end []byte
					at := script + 20
					if version == "FE8J" || version == "FE8U" {
						at = script + 16
						move = []byte{0x40, 0x2F, 0, 0, 1, 0, 5, 9}
						end = []byte{0x20, 1, 0, 0}
						switch mode {
						case "call":
							barrier = []byte{0x40, 0x0A, 0, 0, 0, 0x58, 0x24, 8}
						case "branch":
							barrier = []byte{0x20, 9, 1, 0}
						case "ASM":
							barrier = []byte{0x40, 0x0D, 0, 0, 0, 0x58, 0x24, 8}
						case "unknown":
							barrier = []byte{0x20, 0xFE, 0, 0}
						case "other map":
							barrier = []byte{0x20, 0x25, 1, 0}
						}
					} else {
						code, call, branch, asm, mapCode := byte(
							0x24,
						), byte(
							0xCC,
						), byte(
							0x43,
						), byte(
							0x3C,
						), byte(
							0x1B,
						)
						switch version {
						case "FE6":
							code, call, branch, asm, mapCode = 0x0F, 0x25, 0x1C, 0x17, 0x3F
						case "FE7U":
							code, call, branch, asm, mapCode = 0x26, 0xCB, 0x45, 0x3E, 0x1D
						}
						move = []byte{code, 0, 0, 0, 1, 0, 0, 0, 5, 0, 9, 0}
						end = make([]byte, 4)
						switch mode {
						case "call":
							barrier = []byte{call, 0, 0, 0, 0, 0x58, 0x24, 8}
						case "branch":
							barrier = []byte{branch, 0, 0, 0, 1, 0, 0, 0}
						case "ASM":
							barrier = []byte{asm, 0, 0, 0, 0, 0x58, 0x24, 8}
						case "unknown":
							barrier = []byte{0xFE, 0xFF, 0, 0}
						case "other map":
							barrier = []byte{mapCode, 0, 0, 0, 1, 0, 0, 0}
						}
					}
					f.WriteAt(at, barrier)
					f.WriteAt(at+len(barrier), move)
					f.WriteAt(at+len(barrier)+len(move), end)
					f.WriteAt(0x245800, move)
					f.WriteAt(0x245800+len(move), end)
					s := testutil.Open(t, f)
					d, err := s.Maps.NewDraft(0)
					if err != nil {
						t.Fatal(err)
					}
					if err = d.Resize(3, 4, maps.MapTopLeft, 0); err != nil {
						t.Fatal(err)
					}
					plan, err := s.Maps.Plan(d)
					if err != nil {
						t.Fatal(err)
					}
					known := false
					for _, item := range plan.Impacts() {
						if item.Kind != maps.ScriptPosition ||
							item.Chapter != 0 {
							continue
						}
						if item.Bounds.Min.X == 5 && item.Bounds.Min.Y == 9 {
							t.Fatal(
								"unconfirmed script context treated as this map",
								item,
							)
						}
						known = known ||
							(item.Address == script+8 && item.LostCells == 1)
					}
					if !known {
						t.Fatal("confirmed prefix was lost")
					}
					if mode != "other map" {
						warning := false
						for _, item := range plan.ImpactWarnings() {
							warning = warning ||
								(item.Chapter == 0 && item.Kind == maps.ScriptPosition && errors.Is(item.Err, event.ErrMapImpactContext))
						}
						if !warning {
							t.Fatal("unknown script context was not reported")
						}
					}
				})
			}
		})
	}
}
