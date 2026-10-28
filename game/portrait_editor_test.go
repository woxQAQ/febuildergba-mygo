package game

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func TestPortraitEditingWorkflow(t *testing.T) {
	for index, fixture := range gameFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			f := assetFixture(t, index)
			g := []gameFormat{fe8uFormat(), fe8jFormat(), fe7uFormat(), fe7jFormat(), fe6Format()}[index]
			raw := bytes.Repeat([]byte{0x11}, 4096)
			if index == 4 {
				raw = append(raw, bytes.Repeat([]byte{0x11}, 256)...)
			}
			copy(raw[768:800], bytes.Repeat([]byte{0x22}, 32))
			copy(raw[2816:2848], bytes.Repeat([]byte{0x33}, 32))
			compressed, err := gba.CompressLZ77(raw)
			if err != nil {
				t.Fatal(err)
			}
			f.WriteAt(assetFace, compressed)
			at := assetTable + g.portraits.stride
			f.WriteAt(at+g.portraits.stride-2, []byte{0xA5, 0x5A})
			s := openFixture(t, f)
			baseline := s.Document.Snapshot()
			p, err := s.Portrait(1)
			if err != nil || p.Main == nil || p.Mini == nil || p.Sheet == nil {
				t.Fatalf("portrait parts: %v %+v", err, p)
			}
			if got := p.Mini.RGBAAt(0, 0); got != (color.RGBA{G: 255, A: 255}) {
				t.Fatalf("independent mini tile: %v", got)
			}
			frame, err := p.Frame(0, 1)
			want := color.RGBA{B: 255, A: 255}
			if index == 4 {
				want = color.RGBA{G: 255, A: 255}
			}
			if err != nil || frame.RGBAAt(0, 0) != want {
				t.Fatalf("mouth frame origin: %v %v", err, frame.RGBAAt(0, 0))
			}
			if index != 4 {
				frame, err = p.Frame(2, 0)
				if err != nil ||
					frame.RGBAAt(0, 0) != (color.RGBA{B: 255, A: 255}) {
					t.Fatal("closed eye native tile")
				}
				if !p.HasEyes || p.MouthFrames() != 7 {
					t.Fatal("FE7/8 frame capabilities")
				}
			} else if p.HasEyes || p.MouthFrames() != 5 {
				t.Fatal("FE6 frame capabilities")
			}
			if s.Document.State().Dirty {
				t.Fatal("preview wrote ROM")
			}
			fields := map[string]int64{
				"portrait_mouth_x": 2,
				"portrait_mouth_y": 3,
			}
			if p.HasEyes {
				fields["portrait_eye_x"] = 1
				fields["portrait_eye_y"] = 4
				fields["portrait_blink"] = 6
			}
			// Public view metadata cannot redirect the writer to an unrelated byte.
			p.Fields[0].Offset = 0
			if err := s.SetPortrait(p, fields); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			offset := 20
			if index == 4 {
				offset = 12
			}
			wantData := bytes.Clone(baseline.Data)
			wantData[at+offset], wantData[at+offset+1] = 2, 3
			if p.HasEyes {
				wantData[at+22], wantData[at+23], wantData[at+24] = 1, 4, 6
			}
			if !bytes.Equal(after.Data, wantData) {
				t.Fatal(
					"position edit changed pointers, reserved bytes or another record",
				)
			}
			if err := s.SetPortrait(p, fields); !errors.Is(err, rom.ErrStale) {
				t.Fatalf("stale resource accepted: %v", err)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(s.Document.Snapshot().Data, baseline.Data) {
				t.Fatal("atomic coordinate undo")
			}
			if !s.Document.Redo() {
				t.Fatal("coordinate redo")
			}
			p, err = s.Portrait(1)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.SetPortrait(
				p,
				map[string]int64{"portrait_mouth_x": 256},
			); !errors.Is(
				err,
				ErrValue,
			) {
				t.Fatal("invalid coordinate accepted")
			}
			if err := s.SetPortrait(
				p,
				map[string]int64{"portrait_mouth_x": 7, "unknown": 0},
			); !errors.Is(
				err,
				ErrField,
			) {
				t.Fatal("unknown field accepted")
			}
			if s.Document.State().Revision != p.Revision() {
				t.Fatal("invalid multi-field edit partially committed")
			}
			card, err := s.Portrait(3)
			if err != nil || len(card.Fields) != 0 || card.Main == nil {
				t.Fatal("class card capabilities")
			}
			if err := s.SetPortrait(
				card,
				map[string]int64{"portrait_mouth_x": 1},
			); !errors.Is(
				err,
				ErrField,
			) {
				t.Fatal("class card got mouth fields")
			}
			other := openFixture(t, f)
			if err := other.SetPortrait(
				p,
				fields,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatal("foreign document accepted")
			}
			portraitSource, err := s.RecordPortrait(Units, 1)
			if err != nil || !portraitSource.Current(s) || !p.Current(s) {
				t.Fatal("current portrait snapshot", err)
			}
			layout := s.Table(Units).Layout
			manual := layout
			manual.Count--
			if err := s.UseLayout(Units, manual); err != nil {
				t.Fatal(err)
			}
			if p.Current(s) || portraitSource.Current(s) {
				t.Fatal("layout change kept portrait view current")
			}
			if err := s.SetPortrait(p, fields); !errors.Is(err, rom.ErrStale) {
				t.Fatal("layout change accepted old resource")
			}
			if err := s.UseLayout(Units, layout); err != nil {
				t.Fatal(err)
			}
			p, err = s.Portrait(1)
			if err != nil {
				t.Fatal(err)
			}
			beforeMove := s.Document.Snapshot()
			const movedTable = 0x35C000
			body := bytes.Clone(
				beforeMove.Data[assetTable : assetTable+4*g.portraits.stride],
			)
			if err := s.Document.Apply(
				beforeMove.Revision,
				"move portrait table",
				[]rom.Change{
					{Offset: movedTable, Data: body},
					{
						Offset: g.portraits.pointer,
						Data: binary.LittleEndian.AppendUint32(
							nil,
							0x08000000+movedTable,
						),
					},
				},
			); err != nil {
				t.Fatal(err)
			}
			if err := s.SetPortrait(p, fields); !errors.Is(err, rom.ErrStale) {
				t.Fatal("table repoint accepted old metadata")
			}
			p, err = s.Portrait(1)
			if err != nil || p.Entry.Address != movedTable+g.portraits.stride {
				t.Fatal("metadata ignored live portrait pointer", err)
			}
			bitmap := portraitCrop(p.Sheet, p.Sheet.Bounds())
			bitmap.SetRGBA(16, 0, color.RGBA{G: 255, A: 255})
			pngData := imagePNG(t, bitmap)
			plan, err := s.PlanAssetEdit(
				AssetRef{Kind: PortraitAsset, ID: 1},
				pngData,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.SetPortrait(
				p,
				map[string]int64{"portrait_mouth_x": 4},
			); err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyAssetEdit(plan); !errors.Is(err, rom.ErrStale) {
				t.Fatal("position change did not invalidate PNG preview")
			}
			if !s.Document.Undo() {
				t.Fatal("position before PNG undo")
			}
			plan, err = s.PlanAssetEdit(
				AssetRef{Kind: PortraitAsset, ID: 1},
				pngData,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyAssetEdit(plan); err != nil {
				t.Fatal(err)
			}
			p, err = s.Portrait(1)
			if err != nil || p.Mouth != image.Pt(2, 3) ||
				p.Main.RGBAAt(16, 0) != (color.RGBA{G: 255, A: 255}) {
				t.Fatal("PNG replacement lost coordinates or image")
			}
			output := filepath.Join(t.TempDir(), "portrait-position.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(output)
			if err != nil {
				t.Fatal(err)
			}
			p, err = reopened.Portrait(1)
			if err != nil || p.Mouth != image.Pt(2, 3) {
				t.Fatal("position save/reopen")
			}
			if err := reopened.SetPortrait(
				p,
				map[string]int64{"portrait_mouth_y": 4},
			); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPortraitPartsAndReferences(t *testing.T) {
	for index, fixture := range gameFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			f := assetFixture(t, index)
			g := []gameFormat{fe8uFormat(), fe8jFormat(), fe7uFormat(), fe7jFormat(), fe6Format()}[index]
			generic := g.genericPortraitFormat()
			const genericTable = 0x358000
			f.WritePointer(generic.pointer, genericTable)
			for id := 0; id < generic.count; id++ {
				f.WritePointer(genericTable+id*4, assetMini+0x400)
				f.WritePointer(genericTable+32+id*4, assetPalette)
			}
			f.WriteAt(assetMini+0x400, bytes.Repeat([]byte{0x33}, 512))
			s := openFixture(t, f)
			r, err := s.RecordPortrait(Units, 1)
			if err != nil || r.Ref.ID != 1 || r.Inherited ||
				r.Mini.Image == nil {
				t.Fatal("stored portrait reference")
			}
			if err := s.Set(Units, []int{1}, "portrait_id", 0); err != nil {
				t.Fatal(err)
			}
			r, err = s.RecordPortrait(Units, 1)
			if err != nil || !r.Inherited || r.Ref.ID != 1 {
				t.Fatalf("class portrait fallback: %+v %v", r, err)
			}
			if err := s.Set(Units, []int{1}, "mini_portrait", 1); err != nil {
				t.Fatal(err)
			}
			r, err = s.RecordPortrait(Units, 1)
			if err != nil || r.GenericMini != 1 || r.MiniErr != nil ||
				r.Mini.Image.RGBAAt(0, 0) != (color.RGBA{B: 255, A: 255}) ||
				len(r.MiniResources) != 2 {
				t.Fatalf("generic mini source: %+v %v", r, err)
			}
			if err := s.Set(Units, []int{1}, "mini_portrait", 255); err != nil {
				t.Fatal(err)
			}
			r, err = s.RecordPortrait(Units, 1)
			if err != nil || r.Main.Image == nil || r.MiniErr == nil {
				t.Fatal("invalid generic miniature hid main portrait")
			}
			at := assetTable + g.portraits.stride
			before := s.Document.Snapshot()
			if err := s.Document.Apply(
				before.Revision,
				"broken mini",
				[]rom.Change{
					{Offset: at + 4, Data: []byte{0xFF, 0xFF, 0xFF, 0xFF}},
				},
			); err != nil {
				t.Fatal(err)
			}
			p, err := s.Portrait(1)
			if err != nil || p.Main == nil || p.MiniErr == nil ||
				p.SheetErr == nil {
				t.Fatalf("partial resource recovery: %v %+v", err, p)
			}
			if err := s.SetPortrait(
				p,
				map[string]int64{"portrait_mouth_x": 1},
			); err != nil {
				t.Fatal("valid metadata blocked by damaged mini", err)
			}
			if !s.Document.Undo() {
				t.Fatal("position undo")
			}
			if !s.Document.Undo() {
				t.Fatal("partial resource undo")
			}
			if index != 4 {
				before = s.Document.Snapshot()
				if err := s.Document.Apply(
					before.Revision,
					"missing mouth",
					[]rom.Change{{Offset: at + 12, Data: []byte{0, 0, 0, 0}}},
				); err != nil {
					t.Fatal(err)
				}
				p, err = s.Portrait(1)
				if err != nil || p.Main == nil || p.Mini == nil {
					t.Fatal("missing mouth hid main or mini")
				}
				if _, err := p.Frame(0, 1); err == nil {
					t.Fatal("missing mouth frame accepted")
				}
				if _, err := p.Frame(0, 7); err != nil {
					t.Fatal("embedded mouth frame unavailable")
				}
			}
		})
	}
}
