package worldmap

import (
	"image"

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
	// Reload and fade are confirmed consumers of the daytime palette. The
	// dark palette is replaced independently and never follows daytime writes.
	PaletteReload, PaletteFade, DarkPalette int
	// The second image consumer reloads roads when returning to the map.
	RoadTiles, RoadTilesReload, RoadPalette int
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
		{f.PaletteReload, 128, 0},
		{f.PaletteFade, 128, 0},
		{f.DarkPalette, 128, 0},
		{f.PaletteMap, 0, 1280},
		{f.RoadTiles, 0, 0x3000},
		{f.RoadTilesReload, 0, 0x3000},
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

func (f SceneFormat) background(
	data []byte,
	tone BackgroundTone,
) (*image.RGBA, error) {
	background, err := f.readBackground(data, tone)
	if err != nil {
		return nil, err
	}
	return background.image(), nil
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
	Background, DarkBackground, Road          *image.RGBA
	BackgroundErr, DarkBackgroundErr, RoadErr error
}

type sceneCache struct {
	stamp            edit.Stamp
	background, dark *image.RGBA
	err, darkErr     error
	routes           map[int]Scene
}

// BackgroundImage returns a cached background independently of the route table.
func (e *Editor) BackgroundImage(tone BackgroundTone) (*image.RGBA, error) {
	e.refreshScene()
	switch tone {
	case Daytime:
		return e.scenery.background, e.scenery.err
	case Darkened:
		return e.scenery.dark, e.scenery.darkErr
	default:
		return nil, ErrScene
	}
}

func (e *Editor) refreshScene() {
	if !e.scenery.stamp.Current(e.context) {
		image := e.context.Capture()
		e.scenery = sceneCache{
			stamp:  image.Stamp(),
			routes: make(map[int]Scene),
		}
		e.scenery.err = e.context.Validate(image.Data)
		if e.scenery.err == nil {
			e.scenery.background, e.scenery.err = e.format.Scene.background(
				image.Data,
				Daytime,
			)
			e.scenery.dark, e.scenery.darkErr = e.format.Scene.background(
				image.Data,
				Darkened,
			)
		} else {
			e.scenery.darkErr = e.scenery.err
		}
	}
}

func (e *Editor) Scene(id int) Scene {
	e.refreshScene()
	if result, ok := e.scenery.routes[id]; ok {
		return result
	}
	result := Scene{
		Background:        e.scenery.background,
		BackgroundErr:     e.scenery.err,
		DarkBackground:    e.scenery.dark,
		DarkBackgroundErr: e.scenery.darkErr,
	}
	routes, err := e.Routes()
	if err == nil && id >= 0 && id < len(routes) {
		result.Road, result.RoadErr = e.format.Scene.road(
			e.context.Capture().Data,
			routes[id],
		)
	} else {
		result.RoadErr = ErrRoutes
	}
	e.scenery.routes[id] = result
	return result
}
