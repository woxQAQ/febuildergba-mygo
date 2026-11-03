package components

import (
	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

// ErrorText renders the primary failure. Diagnostic strings are never treated
// as catalog keys, and secondary causes do not override the outer decision.
func ErrorText(err error) string {
	if err == nil {
		return ""
	}
	code := fault.CodeOf(err)
	if code == maps.ErrMap {
		for frame := range fault.Frames(err) {
			if frame.Code == code && frame.Context.Resource != "" {
				return locales.MapResourceError(
					locales.Label(frame.Context.Resource),
				)
			}
		}
	}
	if message, ok := errorMessages[code.ID]; ok {
		return message()
	}
	return locales.Error() + " " + err.Error()
}

// Catalog ownership stays at the presentation boundary. Functions resolve the
// current locale each time, including errors kept while switching languages.
var errorMessages = map[string]func() string{
	"game.invalid_arena":                   locales.GameInvalidArena,
	"game.arena_reference_conflict":        locales.GameArenaReferenceConflict,
	"game.arena_class_pool":                locales.GameArenaClassPool,
	"game.arena_weapon_chain":              locales.GameArenaWeaponChain,
	"game.invalid_terrain":                 locales.GameInvalidTerrain,
	"game.terrain_reference_conflict":      locales.GameTerrainReferenceConflict,
	"game.invalid_support":                 locales.GameInvalidSupport,
	"game.support_reference_conflict":      locales.GameSupportReferenceConflict,
	"game.invalid_script":                  locales.GameInvalidScript,
	"game.unsupported_script_structure":    locales.GameUnsupportedScriptStructure,
	"game.invalid_shop_inventory":          locales.GameInvalidShopInventory,
	"game.invalid_quote_table":             locales.GameInvalidQuoteTable,
	"game.quote_reference_conflict":        locales.GameQuoteReferenceConflict,
	"game.invalid_worldmap_events":         locales.GameInvalidWorldmapEvents,
	"game.invalid_worldmap_routes":         locales.GameInvalidWorldmapRoutes,
	"game.invalid_worldmap_movement":       locales.GameInvalidWorldmapMovement,
	"game.worldmap_connections":            locales.GameWorldmapConnections,
	"game.invalid_worldmap_scene":          locales.GameInvalidWorldmapScene,
	"game.worldmap_dark_background_pixels": locales.GameWorldmapDarkBackgroundPixels,
	"game.worldmap_road_tileset_png":       locales.GameWorldmapRoadTilesetPng,
	"game.worldmap_road_tileset_size":      locales.GameWorldmapRoadTilesetSize,
	"game.worldmap_road_tileset_colors":    locales.GameWorldmapRoadTilesetColors,
	"game.worldmap_road_tileset_padding":   locales.GameWorldmapRoadTilesetPadding,
	"game.worldmap_background_png":         locales.GameWorldmapBackgroundPng,
	"game.worldmap_background_size":        locales.GameWorldmapBackgroundSize,
	"game.worldmap_background_colors":      locales.GameWorldmapBackgroundColors,
	"game.invalid_worldmap_nodes":          locales.GameInvalidWorldmapNodes,
	"game.worldmap_reference_conflict":     locales.GameWorldmapReferenceConflict,
	"game.invalid_event_conditions":        locales.GameInvalidEventConditions,
	"game.invalid_deployment":              locales.GameInvalidDeployment,
	"game.incomplete_deployment_script":    locales.GameIncompleteDeploymentScript,
	"game.unsupported_version":             locales.GameUnsupportedVersion,
	"game.invalid_layout":                  locales.GameInvalidLayout,
	"game.invalid_field":                   locales.GameInvalidField,
	"game.invalid_value":                   locales.GameInvalidValue,
	"game.invalid_text":                    locales.GameInvalidText,
	"game.invalid_image":                   locales.GameInvalidImage,
	"game.invalid_name_encoding":           locales.GameInvalidNameEncoding,
	"game.invalid_text_encoding":           locales.GameInvalidTextEncoding,
	"game.unsupported_text_control":        locales.GameUnsupportedTextControl,
	"game.invalid_text_argument":           locales.GameInvalidTextArgument,
	"game.invalid_map":                     locales.GameInvalidMap,
	"game.map_graphics_absent":             locales.GameMapGraphicsAbsent,
	"game.map_graphics_png":                locales.GameMapGraphicsPng,
	"game.map_graphics_size":               locales.GameMapGraphicsSize,
	"game.map_graphics_colors":             locales.GameMapGraphicsColors,
	"game.map_graphics_padding":            locales.GameMapGraphicsPadding,
	"game.map_graphics_reference":          locales.GameMapGraphicsReference,
	"game.map_palette_png":                 locales.GameMapPalettePng,
	"game.map_palette_size":                locales.GameMapPaletteSize,
	"game.map_palette_pixels":              locales.GameMapPalettePixels,
	"game.map_palette_colors":              locales.GameMapPaletteColors,
	"game.map_tileset_reference":           locales.GameMapTilesetReference,
	"game.map_tileset_config_size":         locales.GameMapTilesetConfigSize,
	"game.map_tileset_config_data":         locales.GameMapTilesetConfigData,
	"app.tileset_export_path":              locales.AppTilesetExportPath,
	"app.tileset_export_rom":               locales.AppTilesetExportRom,
	"game.map_impact_limit":                locales.GameMapImpactLimit,
	"game.map_impact_context":              locales.GameMapImpactContext,
	"game.invalid_map_changes":             locales.GameInvalidMapChanges,
	"game.map_change_reference":            locales.GameMapChangeReference,
	"game.map_change_context":              locales.GameMapChangeContext,
	"game.map_change_limit":                locales.GameMapChangeLimit,
	"game.unsupported_asset_format":        locales.GameUnsupportedAssetFormat,
	"game.invalid_asset_size":              locales.GameInvalidAssetSize,
	"game.invalid_asset_colors":            locales.GameInvalidAssetColors,
	"game.unmapped_asset_pixels":           locales.GameUnmappedAssetPixels,
	"gba.invalid_graphics":                 locales.GameInvalidImage,
	"space.invalid_range":                  locales.SpaceInvalidRange,
	"space.no_capacity":                    locales.SpaceNoCapacity,
	"space.invalid_reference":              locales.SpaceInvalidReference,
	"rom.range":                            locales.RomRange,
	"rom.stale_edit":                       locales.RomStaleEdit,
	"rom.external_change":                  locales.RomExternalChange,
	"rom.target_exists":                    locales.RomTargetExists,
	"rom.read_only":                        locales.RomReadOnly,
	"rom.invalid_size":                     locales.RomInvalidSize,
	"input.invalid_address":                locales.InvalidAddress,
	"input.invalid_hex":                    locales.InvalidHex,
	"input.raw_length":                     locales.RawLength,
	"input.raw_read_first":                 locales.RawReadFirst,
	"app.png_export_path":                  locales.PngExportPath,
	"app.png_export_rom":                   locales.PngExportRom,
	"app.no_emulator":                      locales.NoEmulator,
	"app.run_saved":                        locales.RunSaved,
	"fault.not_found":                      locales.FaultNotFound,
	"fault.permission":                     locales.FaultPermission,
	"fault.exists":                         locales.FaultExists,
	"fault.io":                             locales.FaultIo,
	"fault.canceled":                       locales.FaultCanceled,
}
