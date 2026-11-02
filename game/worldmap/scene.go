package worldmap

import (
	"image"
	"image/draw"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

const MapWidth, MapHeight = 480, 320

var ErrScene = fault.Code{
	ID:   "game.invalid_worldmap_scene",
	Kind: fault.InvalidData,
}

// SceneFormat names the native daytime background and road graphics roots.
// Road words are relative to the engine's character and palette base.
type SceneFormat struct {
	Image, Palette, PaletteMap int
	RoadTiles, RoadPalette     int
}

type roadTile struct {
	index int
	word  uint16
}

// readRoad follows the native contiguous writes: the fourth header byte repeats
// the run without advancing the destination to a new tilemap row.
func readRoad(data []byte, at int) ([]roadTile, int, error) {
	if at < 0xC0 || at%2 != 0 {
		return nil, 0, ErrScene
	}
	var tiles []roadTile
	pos := at
	for pos-at < 65536 && gba.Within(data, pos, 1) {
		if data[pos] == 0xFF {
			return tiles, pos - at + 1, nil
		}
		if !gba.Within(data, pos, 4) {
			break
		}
		x, y, n, repeat := int(
			data[pos],
		), int(
			data[pos+1],
		), int(
			data[pos+2],
		), int(
			data[pos+3],
		)
		count := n * repeat
		start := y*60 + x
		if x >= 60 || y >= 40 || count > 2400-start ||
			!gba.Within(data, pos+4, count*2) ||
			len(tiles)+count > 32768 {
			break
		}
		pos += 4
		for i := range count {
			tiles = append(
				tiles,
				roadTile{start + i, gba.Uint16(data, pos+i*2)},
			)
		}
		pos += count * 2
	}
	return nil, pos - at, ErrScene
}

func fixedAsset(data []byte, site, length int) ([]byte, int, error) {
	at, ok := gba.Pointer(data, site)
	if site < 0xC0 || !ok || at < 0xC0 || at%4 != 0 ||
		!gba.Within(data, at, length) {
		return nil, 0, ErrScene
	}
	return data[at : at+length], at, nil
}

func compressedAsset(data []byte, site, limit int) ([]byte, int, int, error) {
	_, at, err := fixedAsset(data, site, 4)
	if err != nil {
		return nil, 0, 0, err
	}
	decoded, length, err := gba.DecompressLZ77Stream(data[at:], limit)
	if err != nil {
		return nil, at, 0, fault.New(
			ErrScene,
			"worldmap.decompress_scene",
			fault.Context{
				Range: &fault.Range{Offset: at, Length: 4, Size: len(data)},
			},
			err,
		)
	}
	return decoded, at, length, nil
}

func (f SceneFormat) claims(data []byte) []space.Claim {
	var claims []space.Claim
	for _, asset := range []struct{ site, length, limit int }{
		{f.Image, MapWidth * MapHeight / 2, 0},
		{f.Palette, 128, 0},
		{f.PaletteMap, 0, 1280},
		{f.RoadTiles, 0, 0x3000},
		{f.RoadPalette, 32, 0},
	} {
		if asset.site < 0xC0 || !gba.Within(data, asset.site, 4) {
			continue
		}
		owner := space.Owner{Resource: "worldmap.scene", ID: asset.site}
		claims = append(
			claims,
			space.Claim{
				Start: asset.site,
				End:   asset.site + 4,
				Owner: owner,
				Site:  true,
			},
		)
		var at, length int
		var err error
		if asset.limit != 0 {
			_, at, length, err = compressedAsset(data, asset.site, asset.limit)
		} else {
			_, at, err = fixedAsset(data, asset.site, asset.length)
			length = asset.length
		}
		if err == nil {
			claims = append(
				claims,
				space.Claim{Start: at, End: at + length, Owner: owner},
			)
		}
	}
	return claims
}

func (f SceneFormat) background(data []byte) (*image.RGBA, error) {
	pixels, _, err := fixedAsset(data, f.Image, MapWidth*MapHeight/2)
	if err != nil {
		return nil, err
	}
	colors, _, err := fixedAsset(data, f.Palette, 128)
	if err != nil {
		return nil, err
	}
	banks, _, _, err := compressedAsset(data, f.PaletteMap, 1280)
	if err != nil {
		return nil, err
	}
	if len(banks) != 1280 {
		return nil, ErrScene
	}
	palette := make([]byte, 512)
	copy(palette, colors)
	out := image.NewRGBA(image.Rect(0, 0, MapWidth, MapHeight))
	// Chunks keep character indices within the ten bits of a GBA tile word.
	for row := 0; row < 40; row += 16 {
		height := min(16, 40-row)
		tiles := make([]uint16, 60*height)
		for i := range tiles {
			x, y := i%60, row+i/60
			bank := banks[y*32+x/2] >> ((x % 2) * 4) & 15
			if bank >= 4 {
				return nil, ErrScene
			}
			tiles[i] = uint16(i) | uint16(bank)<<12
		}
		part, err := gba.Background4BPP(
			pixels[row*60*32:(row+height)*60*32],
			palette,
			tiles,
			60,
			height,
		)
		if err != nil {
			return nil, err
		}
		draw.Draw(
			out,
			image.Rect(0, row*8, MapWidth, (row+height)*8),
			part,
			image.Point{},
			draw.Src,
		)
	}
	return out, nil
}

func (f SceneFormat) road(data []byte, route Route) (*image.RGBA, error) {
	draft, err := newRoadDraft(data, f, route.Road)
	if err != nil {
		return nil, err
	}
	return draft.Preview(), nil
}

// Scene contains independent background and selected-road results. A damaged
// graphic does not prevent endpoint editing or hide node/control coordinates.
type Scene struct {
	Background, Road       *image.RGBA
	BackgroundErr, RoadErr error
}

type sceneCache struct {
	stamp      edit.Stamp
	background *image.RGBA
	err        error
	routes     map[int]Scene
}

func (e *Editor) Scene(id int) Scene {
	if !e.scenery.stamp.Current(e.context) {
		image := e.context.Document().Snapshot()
		e.scenery = sceneCache{
			stamp:  e.context.Bind(image.Revision),
			routes: make(map[int]Scene),
		}
		e.scenery.err = e.context.Validate(image.Data)
		if e.scenery.err == nil {
			e.scenery.background, e.scenery.err = e.format.Scene.background(
				image.Data,
			)
		}
	}
	if result, ok := e.scenery.routes[id]; ok {
		return result
	}
	result := Scene{
		Background:    e.scenery.background,
		BackgroundErr: e.scenery.err,
	}
	routes, err := e.Routes()
	if err == nil && id >= 0 && id < len(routes) {
		result.Road, result.RoadErr = e.format.Scene.road(
			e.context.Document().Snapshot().Data,
			routes[id],
		)
	} else {
		result.RoadErr = ErrRoutes
	}
	e.scenery.routes[id] = result
	return result
}
