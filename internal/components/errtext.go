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
	"game.invalid_script":               locales.GameInvalidScript,
	"game.unsupported_script_structure": locales.GameUnsupportedScriptStructure,
	"game.invalid_shop_inventory":       locales.GameInvalidShopInventory,
	"game.invalid_worldmap_nodes":       locales.GameInvalidWorldmapNodes,
	"game.invalid_event_conditions":     locales.GameInvalidEventConditions,
	"game.invalid_deployment":           locales.GameInvalidDeployment,
	"game.incomplete_deployment_script": locales.GameIncompleteDeploymentScript,
	"game.unsupported_version":          locales.GameUnsupportedVersion,
	"game.invalid_layout":               locales.GameInvalidLayout,
	"game.invalid_field":                locales.GameInvalidField,
	"game.invalid_value":                locales.GameInvalidValue,
	"game.invalid_text":                 locales.GameInvalidText,
	"game.invalid_image":                locales.GameInvalidImage,
	"game.invalid_name_encoding":        locales.GameInvalidNameEncoding,
	"game.invalid_text_encoding":        locales.GameInvalidTextEncoding,
	"game.unsupported_text_control":     locales.GameUnsupportedTextControl,
	"game.invalid_text_argument":        locales.GameInvalidTextArgument,
	"game.invalid_map":                  locales.GameInvalidMap,
	"game.map_impact_limit":             locales.GameMapImpactLimit,
	"game.map_impact_context":           locales.GameMapImpactContext,
	"game.invalid_map_changes":          locales.GameInvalidMapChanges,
	"game.map_change_reference":         locales.GameMapChangeReference,
	"game.map_change_context":           locales.GameMapChangeContext,
	"game.map_change_limit":             locales.GameMapChangeLimit,
	"game.unsupported_asset_format":     locales.GameUnsupportedAssetFormat,
	"game.invalid_asset_size":           locales.GameInvalidAssetSize,
	"game.invalid_asset_colors":         locales.GameInvalidAssetColors,
	"game.unmapped_asset_pixels":        locales.GameUnmappedAssetPixels,
	"gba.invalid_graphics":              locales.GameInvalidImage,
	"space.invalid_range":               locales.SpaceInvalidRange,
	"space.no_capacity":                 locales.SpaceNoCapacity,
	"space.invalid_reference":           locales.SpaceInvalidReference,
	"rom.range":                         locales.RomRange,
	"rom.stale_edit":                    locales.RomStaleEdit,
	"rom.external_change":               locales.RomExternalChange,
	"rom.target_exists":                 locales.RomTargetExists,
	"rom.read_only":                     locales.RomReadOnly,
	"rom.invalid_size":                  locales.RomInvalidSize,
	"input.invalid_address":             locales.InvalidAddress,
	"input.invalid_hex":                 locales.InvalidHex,
	"input.raw_length":                  locales.RawLength,
	"input.raw_read_first":              locales.RawReadFirst,
	"app.png_export_path":               locales.PngExportPath,
	"app.png_export_rom":                locales.PngExportRom,
	"app.no_emulator":                   locales.NoEmulator,
	"app.run_saved":                     locales.RunSaved,
	"fault.not_found":                   locales.FaultNotFound,
	"fault.permission":                  locales.FaultPermission,
	"fault.exists":                      locales.FaultExists,
	"fault.io":                          locales.FaultIo,
	"fault.canceled":                    locales.FaultCanceled,
}
