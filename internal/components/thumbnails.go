package components

import (
	"image"

	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/sprite"
)

// Thumbnails caches decoded images, never build-scoped UI elements. Resource
// stamps invalidate both successful and failed reads after document/layout edits.
type Thumbnails struct {
	session           *game.Session
	portraits         map[int]portraitThumbnail
	standing, weapons map[int]spriteThumbnail
}
type portraitThumbnail struct {
	selection records.PortraitSelection
	thumbnail Thumbnail
}
type spriteThumbnail struct {
	source    sprite.Preview
	thumbnail Thumbnail
}
type Thumbnail struct {
	bitmap        *ui.Bitmap
	width, height float32
	err           error
}

func thumbnail(bitmap *image.RGBA, err error) Thumbnail {
	result := Thumbnail{err: err}
	if bitmap != nil {
		result.bitmap = ui.NewBitmap(bitmap)
		result.bitmap.Pixelated()
		result.width = float32(bitmap.Bounds().Dx())
		result.height = float32(bitmap.Bounds().Dy())
	}
	return result
}

func (t *Thumbnails) prepare(session *game.Session) {
	if t.session != session {
		*t = Thumbnails{
			session:   session,
			portraits: make(map[int]portraitThumbnail),
			standing:  make(map[int]spriteThumbnail),
			weapons:   make(map[int]spriteThumbnail),
		}
	}
}

func (t *Thumbnails) Portrait(session *game.Session, id int) Thumbnail {
	t.prepare(session)
	if cached, ok := t.portraits[id]; ok &&
		cached.selection.Current(session.Records) {
		return cached.thumbnail
	}
	selection, err := session.Records.Portrait(records.Units, id)
	var bitmap *image.RGBA
	if err == nil && !selection.Inherited && selection.Ref.ID != 0 {
		preview, previewErr := session.Portraits.Preview(selection)
		err = previewErr
		if err == nil {
			bitmap, err = preview.Mini.Image, preview.MiniErr
			if bitmap == nil {
				bitmap, err = preview.Main.Image, preview.MainErr
			}
		}
	}
	result := thumbnail(bitmap, err)
	t.portraits[id] = portraitThumbnail{selection, result}
	return result
}

func (t *Thumbnails) Standing(session *game.Session, id int) Thumbnail {
	t.prepare(session)
	if cached, ok := t.standing[id]; ok &&
		cached.source.Current(session.Sprites) {
		return cached.thumbnail
	}
	value, err := session.Records.Value(records.Classes, id, "map_sprite")
	source := session.Sprites.Standing(int(value))
	if err != nil {
		source.Image = nil
		source.Err = err
	}
	result := thumbnail(visiblePixels(source.Image), source.Err)
	t.standing[id] = spriteThumbnail{source, result}
	return result
}

func (t *Thumbnails) Weapon(session *game.Session, id int) Thumbnail {
	t.prepare(session)
	if cached, ok := t.weapons[id]; ok &&
		cached.source.Current(session.Sprites) {
		return cached.thumbnail
	}
	source := session.Sprites.Weapon(id)
	result := thumbnail(source.Image, source.Err)
	t.weapons[id] = spriteThumbnail{source, result}
	return result
}

func (t Thumbnail) View(c *ui.Context, size float32, label string) {
	ui.Row(c).
		Size(size, size).
		Shrink(0).
		Justify(ui.Center).
		AlignItems(ui.Center).
		Children(func() {
			if t.bitmap != nil {
				scale := min(float32(1), size/t.width, size/t.height)
				ui.Image(c, t.bitmap).
					Size(t.width*scale, t.height*scale).
					Label(label)
			} else {
				help := label
				if t.err != nil {
					help += "\n" + ErrorText(t.err)
				}
				ui.Text(c, "—").
					TextColor(c.Theme().TextMuted).
					Label(label).
					Tooltip(help)
			}
		})
}

// visiblePixels removes the transparent canvas around a standing frame.
func visiblePixels(bitmap *image.RGBA) *image.RGBA {
	if bitmap == nil {
		return nil
	}
	var bounds image.Rectangle
	for y := bitmap.Bounds().Min.Y; y < bitmap.Bounds().Max.Y; y++ {
		for x := bitmap.Bounds().Min.X; x < bitmap.Bounds().Max.X; x++ {
			if bitmap.RGBAAt(x, y).A != 0 {
				bounds = bounds.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if bounds.Empty() {
		return nil
	}
	return bitmap.SubImage(bounds).(*image.RGBA)
}

// Inline uses only the image's visible width and at most one text row of height.
func (t Thumbnail) Inline(c *ui.Context, height float32, label string) {
	if !t.Available() {
		return
	}
	scale := min(float32(1), height/t.height)
	ui.Image(c, t.bitmap).
		Size(t.width*scale, t.height*scale).
		Shrink(0).
		Label(label)
}

func (t Thumbnail) Available() bool { return t.bitmap != nil }
