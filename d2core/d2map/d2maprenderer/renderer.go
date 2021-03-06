package d2maprenderer

import (
	"errors"
	"image/color"
	"log"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dat"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2term"
)

// The map renderer, used to render the map
type MapRenderer struct {
	mapEngine     *d2mapengine.MapEngine // The map engine that is being rendered
	palette       *d2dat.DATPalette      // The palette used for this map
	viewport      *Viewport              // The viewport for the map renderer (used for rendering offsets)
	camera        Camera                 // The camera for this map renderer (used to determine where on the map we are rendering)
	debugVisLevel int                    // Debug visibility index (0=none, 1=tiles, 2=sub-tiles)
	lastFrameTime float64                // The last time the map was rendered
	currentFrame  int                    // The current render frame (for animations)
}

// Creates an instance of the map renderer
func CreateMapRenderer(mapEngine *d2mapengine.MapEngine) *MapRenderer {
	result := &MapRenderer{
		mapEngine: mapEngine,
		viewport:  NewViewport(0, 0, 800, 600),
	}

	result.viewport.SetCamera(&result.camera)

	d2term.BindAction("mapdebugvis", "set map debug visualization level", func(level int) {
		result.debugVisLevel = level
	})

	if mapEngine.LevelType().Id != 0 {
		result.generateTileCache()
	}

	return result
}

func (mr *MapRenderer) RegenerateTileCache() {
	mr.generateTileCache()
}

func (mr *MapRenderer) SetMapEngine(mapEngine *d2mapengine.MapEngine) {
	mr.mapEngine = mapEngine
	mr.generateTileCache()
}

func (mr *MapRenderer) Render(target d2render.Surface) {
	mr.renderPass1(mr.viewport, target)
	if mr.debugVisLevel > 0 {
		mr.renderDebug(mr.debugVisLevel, mr.viewport, target)
	}
	mr.renderPass2(mr.viewport, target)
	mr.renderPass3(mr.viewport, target)
}

func (mr *MapRenderer) MoveCameraTo(x, y float64) {
	mr.camera.MoveTo(x, y)
}

func (mr *MapRenderer) MoveCameraBy(x, y float64) {
	mr.camera.MoveBy(x, y)
}

func (mr *MapRenderer) ScreenToWorld(x, y int) (float64, float64) {
	return mr.viewport.ScreenToWorld(x, y)
}

func (mr *MapRenderer) ScreenToOrtho(x, y int) (float64, float64) {
	return mr.viewport.ScreenToOrtho(x, y)
}

func (mr *MapRenderer) WorldToOrtho(x, y float64) (float64, float64) {
	return mr.viewport.WorldToOrtho(x, y)
}

func (mr *MapRenderer) renderPass1(viewport *Viewport, target d2render.Surface) {
	mapSize := mr.mapEngine.Size()
	// TODO: Render based on visible area
	for tileY := 0; tileY < mapSize.Height; tileY++ {
		for tileX := 0; tileX < mapSize.Width; tileX++ {
			tile := mr.mapEngine.TileAt(tileX, tileY)
			if viewport.IsTileVisible(float64(tileX), float64(tileY)) {
				viewport.PushTranslationWorld(float64(tileX), float64(tileY))
				mr.renderTilePass1(tile, target)
				viewport.PopTranslation()
			}
		}
	}
}

func (mr *MapRenderer) renderPass2(viewport *Viewport, target d2render.Surface) {
	mapSize := mr.mapEngine.Size()

	// TODO: Render based on visible area
	for tileY := 0; tileY < mapSize.Height; tileY++ {
		for tileX := 0; tileX < mapSize.Width; tileX++ {
			tile := mr.mapEngine.TileAt(tileX, tileY)
			if viewport.IsTileVisible(float64(tileX), float64(tileY)) {
				viewport.PushTranslationWorld(float64(tileX), float64(tileY))
				mr.renderTilePass2(tile, target)

				// TODO: Do not loop over every entity every frame
				for _, mapEntity := range *mr.mapEngine.Entities() {
					entityX, entityY := mapEntity.GetPosition()
					if (int(entityX) != tileX) || (int(entityY) != tileY) {
						continue
					}
					target.PushTranslation(viewport.GetTranslationScreen())
					mapEntity.Render(target)
					target.Pop()
				}
				viewport.PopTranslation()
			}
		}
	}
}

func (mr *MapRenderer) renderPass3(viewport *Viewport, target d2render.Surface) {
	mapSize := mr.mapEngine.Size()
	// TODO: Render based on visible area
	for tileY := 0; tileY < mapSize.Height; tileY++ {
		for tileX := 0; tileX < mapSize.Width; tileX++ {
			tile := mr.mapEngine.TileAt(tileX, tileY)
			if viewport.IsTileVisible(float64(tileX), float64(tileY)) {
				viewport.PushTranslationWorld(float64(tileX), float64(tileY))
				mr.renderTilePass3(tile, target)
				viewport.PopTranslation()
			}
		}
	}

}

func (mr *MapRenderer) renderTilePass1(tile *d2ds1.TileRecord, target d2render.Surface) {
	for _, wall := range tile.Walls {
		if !wall.Hidden && wall.Prop1 != 0 && wall.Type.LowerWall() {
			mr.renderWall(wall, mr.viewport, target)
		}
	}

	for _, floor := range tile.Floors {
		if !floor.Hidden && floor.Prop1 != 0 {
			mr.renderFloor(floor, target)
		}
	}

	for _, shadow := range tile.Shadows {
		if !shadow.Hidden && shadow.Prop1 != 0 {
			mr.renderShadow(shadow, target)
		}
	}
}

func (mr *MapRenderer) renderTilePass2(tile *d2ds1.TileRecord, target d2render.Surface) {
	for _, wall := range tile.Walls {
		if !wall.Hidden && wall.Type.UpperWall() {
			mr.renderWall(wall, mr.viewport, target)
		}
	}
}

func (mr *MapRenderer) renderTilePass3(tile *d2ds1.TileRecord, target d2render.Surface) {
	for _, wall := range tile.Walls {
		if wall.Type == d2enum.Roof {
			mr.renderWall(wall, mr.viewport, target)
		}
	}
}

func (mr *MapRenderer) renderFloor(tile d2ds1.FloorShadowRecord, target d2render.Surface) {
	var img d2render.Surface
	if !tile.Animated {
		img = mr.getImageCacheRecord(tile.Style, tile.Sequence, 0, tile.RandomIndex)
	} else {
		img = mr.getImageCacheRecord(tile.Style, tile.Sequence, 0, byte(mr.currentFrame))
	}
	if img == nil {
		log.Printf("Render called on uncached floor {%v,%v}", tile.Style, tile.Sequence)
		return
	}

	mr.viewport.PushTranslationOrtho(-80, float64(tile.YAdjust))
	defer mr.viewport.PopTranslation()

	target.PushTranslation(mr.viewport.GetTranslationScreen())
	defer target.Pop()

	target.Render(img)
}

func (mr *MapRenderer) renderWall(tile d2ds1.WallRecord, viewport *Viewport, target d2render.Surface) {
	img := mr.getImageCacheRecord(tile.Style, tile.Sequence, tile.Type, tile.RandomIndex)
	if img == nil {
		log.Printf("Render called on uncached wall {%v,%v,%v}", tile.Style, tile.Sequence, tile.Type)
		return
	}

	viewport.PushTranslationOrtho(-80, float64(tile.YAdjust)-8)
	defer viewport.PopTranslation()

	target.PushTranslation(viewport.GetTranslationScreen())
	defer target.Pop()

	target.Render(img)
}

func (mr *MapRenderer) renderShadow(tile d2ds1.FloorShadowRecord, target d2render.Surface) {
	img := mr.getImageCacheRecord(tile.Style, tile.Sequence, 13, tile.RandomIndex)
	if img == nil {
		log.Printf("Render called on uncached shadow {%v,%v}", tile.Style, tile.Sequence)
		return
	}

	defer mr.viewport.PushTranslationOrtho(-80, float64(tile.YAdjust)).PopTranslation()

	target.PushTranslation(mr.viewport.GetTranslationScreen())
	target.PushColor(color.RGBA{R: 255, G: 255, B: 255, A: 160})
	defer target.PopN(2)

	target.Render(img)
}

func (mr *MapRenderer) renderDebug(debugVisLevel int, viewport *Viewport, target d2render.Surface) {
	mapSize := mr.mapEngine.Size()
	// TODO: Render based on visible area
	for tileY := 0; tileY < mapSize.Height; tileY++ {
		for tileX := 0; tileX < mapSize.Width; tileX++ {
			if viewport.IsTileVisible(float64(tileX), float64(tileY)) {
				viewport.PushTranslationWorld(float64(tileX), float64(tileY))
				mr.renderTileDebug(tileX, tileY, debugVisLevel, target)
				viewport.PopTranslation()
			}
		}
	}
}

func (mr *MapRenderer) renderTileDebug(ax, ay int, debugVisLevel int, target d2render.Surface) {
	subTileColor := color.RGBA{R: 80, G: 80, B: 255, A: 50}
	tileColor := color.RGBA{R: 255, G: 255, B: 255, A: 100}
	tileCollisionColor := color.RGBA{R: 128, G: 0, B: 0, A: 100}

	screenX1, screenY1 := mr.viewport.WorldToScreen(float64(ax), float64(ay))
	screenX2, screenY2 := mr.viewport.WorldToScreen(float64(ax+1), float64(ay))
	screenX3, screenY3 := mr.viewport.WorldToScreen(float64(ax), float64(ay+1))

	target.PushTranslation(screenX1, screenY1)
	defer target.Pop()

	target.DrawLine(screenX2-screenX1, screenY2-screenY1, tileColor)
	target.DrawLine(screenX3-screenX1, screenY3-screenY1, tileColor)
	target.PushTranslation(-10, 10)
	target.DrawText("%v, %v", ax, ay)
	target.Pop()

	if debugVisLevel > 1 {
		for i := 1; i <= 4; i++ {
			x2 := i * 16
			y2 := i * 8

			target.PushTranslation(-x2, y2)
			target.DrawLine(80, 40, subTileColor)
			target.Pop()

			target.PushTranslation(x2, y2)
			target.DrawLine(-80, 40, subTileColor)
			target.Pop()
		}

		tile := mr.mapEngine.TileAt(ax, ay)

		//for i, floor := range tile.Floors {
		//	target.PushTranslation(-20, 10+(i+1)*14)
		//	target.DrawText("f: %v-%v", floor.Style, floor.Sequence)
		//	target.Pop()
		//}

		for i, wall := range tile.Walls {
			if wall.Type.Special() {
				target.PushTranslation(-20, 10+(i+1)*14)
				target.DrawText("s: %v-%v", wall.Style, wall.Sequence)
				target.Pop()
			}
		}

		for yy := 0; yy < 5; yy++ {
			for xx := 0; xx < 5; xx++ {
				isoX := (xx - yy) * 16
				isoY := (xx + yy) * 8
				var walkableArea = (*mr.mapEngine.WalkMesh())[((yy+(ay*5))*mr.mapEngine.Size().Width*5)+xx+(ax*5)]
				if !walkableArea.Walkable {
					target.PushTranslation(isoX-3, isoY+4)
					target.DrawRect(5, 5, tileCollisionColor)
					target.Pop()
				}
			}
		}
	}
}

func (mr *MapRenderer) Advance(elapsed float64) {
	frameLength := 0.1

	mr.lastFrameTime += elapsed
	framesAdvanced := int(mr.lastFrameTime / frameLength)
	mr.lastFrameTime -= float64(framesAdvanced) * frameLength

	mr.currentFrame += framesAdvanced
	if mr.currentFrame > 9 {
		mr.currentFrame = 0
	}
}

func loadPaletteForAct(levelType d2enum.RegionIdType) (*d2dat.DATPalette, error) {
	var palettePath string
	switch levelType {
	case d2enum.RegionAct1Town, d2enum.RegionAct1Wilderness, d2enum.RegionAct1Cave, d2enum.RegionAct1Crypt,
		d2enum.RegionAct1Monestary, d2enum.RegionAct1Courtyard, d2enum.RegionAct1Barracks,
		d2enum.RegionAct1Jail, d2enum.RegionAct1Cathedral, d2enum.RegionAct1Catacombs, d2enum.RegionAct1Tristram:
		palettePath = d2resource.PaletteAct1
		break
	case d2enum.RegionAct2Town, d2enum.RegionAct2Sewer, d2enum.RegionAct2Harem, d2enum.RegionAct2Basement,
		d2enum.RegionAct2Desert, d2enum.RegionAct2Tomb, d2enum.RegionAct2Lair, d2enum.RegionAct2Arcane:
		palettePath = d2resource.PaletteAct2
		break
	case d2enum.RegionAct3Town, d2enum.RegionAct3Jungle, d2enum.RegionAct3Kurast, d2enum.RegionAct3Spider,
		d2enum.RegionAct3Dungeon, d2enum.RegionAct3Sewer:
		palettePath = d2resource.PaletteAct3
		break
	case d2enum.RegionAct4Town, d2enum.RegionAct4Mesa, d2enum.RegionAct4Lava, d2enum.RegionAct5Lava:
		palettePath = d2resource.PaletteAct4
		break
	case d2enum.RegonAct5Town, d2enum.RegionAct5Siege, d2enum.RegionAct5Barricade, d2enum.RegionAct5Temple,
		d2enum.RegionAct5IceCaves, d2enum.RegionAct5Baal:
		palettePath = d2resource.PaletteAct5
		break
	default:
		return nil, errors.New("failed to find palette for region")
	}

	return d2asset.LoadPalette(palettePath)
}

func (mr *MapRenderer) ViewportToLeft() {
	mr.viewport.toLeft()
}

func (mr *MapRenderer) ViewportToRight() {
	mr.viewport.toRight()
}

func (mr *MapRenderer) ViewportDefault() {
	mr.viewport.resetAlign()
}
