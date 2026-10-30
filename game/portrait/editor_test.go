package portrait_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/portrait"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func TestPortraitEditingWorkflow(t *testing.T) {
	for index, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.ImageFixture(t, index)
			g := testutil.ImageLayouts[index]
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
			f.WriteAt(testutil.ImageFace, compressed)
			at := testutil.ImageTable + g.PortraitStride
			f.WriteAt(at+g.PortraitStride-2, []byte{0xA5, 0x5A})
			s := testutil.Open(t, f)
			baseline := s.Document.Snapshot()
			p, err := s.Portraits.Read(1)
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
			if err := s.Portraits.Set(p, fields); err != nil {
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
			if err := s.Portraits.Set(
				p,
				fields,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatalf("stale resource accepted: %v", err)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(s.Document.Snapshot().Data, baseline.Data) {
				t.Fatal("atomic coordinate undo")
			}
			if !s.Document.Redo() {
				t.Fatal("coordinate redo")
			}
			p, err = s.Portraits.Read(1)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Portraits.Set(
				p,
				map[string]int64{"portrait_mouth_x": 256},
			); !errors.Is(
				err,
				edit.ErrValue,
			) {
				t.Fatal("invalid coordinate accepted")
			}
			if err := s.Portraits.Set(
				p,
				map[string]int64{"portrait_mouth_x": 7, "unknown": 0},
			); !errors.Is(
				err,
				edit.ErrField,
			) {
				t.Fatal("unknown field accepted")
			}
			if s.Document.State().Revision != p.Revision() {
				t.Fatal("invalid multi-field edit partially committed")
			}
			card, err := s.Portraits.Read(3)
			if err != nil || len(card.Fields) != 0 || card.Main == nil {
				t.Fatal("class card capabilities")
			}
			if err := s.Portraits.Set(
				card,
				map[string]int64{"portrait_mouth_x": 1},
			); !errors.Is(
				err,
				edit.ErrField,
			) {
				t.Fatal("class card got mouth fields")
			}
			other := testutil.Open(t, f)
			if err := other.Portraits.Set(
				p,
				fields,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatal("foreign document accepted")
			}
			portraitSource, err := recordPortrait(s, records.Units, 1)
			if err != nil || !portraitSource.Current(s.Portraits) ||
				!p.Current(s.Portraits) {
				t.Fatal("current portrait snapshot", err)
			}
			layout := s.Records.Table(records.Units).Layout
			manual := layout
			manual.Count--
			if err := s.Records.UseLayout(records.Units, manual); err != nil {
				t.Fatal(err)
			}
			if p.Current(s.Portraits) || portraitSource.Current(s.Portraits) {
				t.Fatal("layout change kept portrait view current")
			}
			if err := s.Portraits.Set(
				p,
				fields,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatal("layout change accepted old resource")
			}
			if err := s.Records.UseLayout(records.Units, layout); err != nil {
				t.Fatal(err)
			}
			p, err = s.Portraits.Read(1)
			if err != nil {
				t.Fatal(err)
			}
			beforeMove := s.Document.Snapshot()
			const movedTable = 0x35C000
			body := bytes.Clone(
				beforeMove.Data[testutil.ImageTable : testutil.ImageTable+4*g.PortraitStride],
			)
			if err := s.Document.Apply(
				beforeMove.Revision,
				"move portrait table",
				[]rom.Change{
					{Offset: movedTable, Data: body},
					{
						Offset: g.PortraitPointer,
						Data: binary.LittleEndian.AppendUint32(
							nil,
							0x08000000+movedTable,
						),
					},
				},
			); err != nil {
				t.Fatal(err)
			}
			if err := s.Portraits.Set(
				p,
				fields,
			); !errors.Is(
				err,
				rom.ErrStale,
			) {
				t.Fatal("table repoint accepted old metadata")
			}
			p, err = s.Portraits.Read(1)
			if err != nil || p.Entry.Address != movedTable+g.PortraitStride {
				t.Fatal("metadata ignored live portrait pointer", err)
			}
			bitmap := image.NewRGBA(p.Sheet.Bounds())
			copy(bitmap.Pix, p.Sheet.Pix)
			bitmap.SetRGBA(16, 0, color.RGBA{G: 255, A: 255})
			pngData := testutil.ImagePNG(t, bitmap)
			plan, err := s.Images.Plan(
				graphics.Ref{Kind: graphics.Portrait, ID: 1},
				pngData,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Portraits.Set(
				p,
				map[string]int64{"portrait_mouth_x": 4},
			); err != nil {
				t.Fatal(err)
			}
			if err := s.Images.Apply(plan); !errors.Is(err, rom.ErrStale) {
				t.Fatal("position change did not invalidate PNG preview")
			}
			if !s.Document.Undo() {
				t.Fatal("position before PNG undo")
			}
			plan, err = s.Images.Plan(
				graphics.Ref{Kind: graphics.Portrait, ID: 1},
				pngData,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Images.Apply(plan); err != nil {
				t.Fatal(err)
			}
			p, err = s.Portraits.Read(1)
			if err != nil || p.Mouth != image.Pt(2, 3) ||
				p.Main.RGBAAt(16, 0) != (color.RGBA{G: 255, A: 255}) {
				t.Fatal("PNG replacement lost coordinates or image")
			}
			output := filepath.Join(t.TempDir(), "portrait-position.gba")
			if err := s.Document.SaveAs(output, false); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			p, err = reopened.Portraits.Read(1)
			if err != nil || p.Mouth != image.Pt(2, 3) {
				t.Fatal("position save/reopen")
			}
			if err := reopened.Portraits.Set(
				p,
				map[string]int64{"portrait_mouth_y": 4},
			); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPortraitPartsAndReferences(t *testing.T) {
	for index, fixture := range testutil.Games() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := testutil.ImageFixture(t, index)
			g := testutil.ImageLayouts[index]
			const genericTable = 0x358000
			f.WritePointer(g.GenericPointer, genericTable)
			for id := 0; id < g.GenericCount; id++ {
				f.WritePointer(genericTable+id*4, testutil.ImageMini+0x400)
				f.WritePointer(genericTable+32+id*4, testutil.ImagePalette)
			}
			f.WriteAt(testutil.ImageMini+0x400, bytes.Repeat([]byte{0x33}, 512))
			s := testutil.Open(t, f)
			r, err := recordPortrait(s, records.Units, 1)
			if err != nil || r.Ref.ID != 1 || r.Inherited ||
				r.Mini.Image == nil {
				t.Fatal("stored portrait reference")
			}
			if err := s.Records.Set(
				records.Units,
				[]int{1},
				"portrait_id",
				0,
			); err != nil {
				t.Fatal(err)
			}
			r, err = recordPortrait(s, records.Units, 1)
			if err != nil || !r.Inherited || r.Ref.ID != 1 {
				t.Fatalf("class portrait fallback: %+v %v", r, err)
			}
			if err := s.Records.Set(
				records.Units,
				[]int{1},
				"mini_portrait",
				1,
			); err != nil {
				t.Fatal(err)
			}
			r, err = recordPortrait(s, records.Units, 1)
			if err != nil || r.GenericMini != 1 || r.MiniErr != nil ||
				r.Mini.Image.RGBAAt(0, 0) != (color.RGBA{B: 255, A: 255}) ||
				len(r.MiniResources) != 2 {
				t.Fatalf("generic mini source: %+v %v", r, err)
			}
			if err := s.Records.Set(
				records.Units,
				[]int{1},
				"mini_portrait",
				255,
			); err != nil {
				t.Fatal(err)
			}
			r, err = recordPortrait(s, records.Units, 1)
			if err != nil || r.Main.Image == nil || r.MiniErr == nil {
				t.Fatal("invalid generic miniature hid main portrait")
			}
			at := testutil.ImageTable + g.PortraitStride
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
			p, err := s.Portraits.Read(1)
			if err != nil || p.Main == nil || p.MiniErr == nil ||
				p.SheetErr == nil {
				t.Fatalf("partial resource recovery: %v %+v", err, p)
			}
			if err := s.Portraits.Set(
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
				p, err = s.Portraits.Read(1)
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

func recordPortrait(
	s *game.Session,
	kind records.Kind,
	id int,
) (portrait.RecordPreview, error) {
	selection, err := s.Records.Portrait(kind, id)
	if err != nil {
		return portrait.RecordPreview{}, err
	}
	return s.Portraits.Preview(selection)
}
