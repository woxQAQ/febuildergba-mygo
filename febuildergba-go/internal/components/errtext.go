package components

import (
	"errors"
	"strings"

	"github.com/egoist/mygo/i18n"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/locales"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// ErrorText maps domain errors to catalog messages while retaining unknown details.
func ErrorText(err error) string {
	if err == nil {
		return ""
	}
	if mapError, ok := errors.AsType[*game.MapError](err); ok {
		return locales.MapResourceError(locales.Label(mapError.Resource))
	}
	key := strings.ReplaceAll(err.Error(), ".", "_")
	if _, _, ok := locales.Catalog.Message(i18n.Locale(), key); ok {
		return locales.Catalog.Format(i18n.Locale(), key, nil)
	}
	for _, target := range []error{game.ErrVersion, game.ErrLayout, game.ErrValue, game.ErrField, game.ErrImage, game.ErrMap, game.ErrText, game.ErrNameEncoding, game.ErrTextControl, game.ErrTextArgument, game.ErrTextEncoding, space.ErrRange, space.ErrCapacity, space.ErrReference, rom.ErrRange, rom.ErrStale, rom.ErrExternalChange, rom.ErrExists, rom.ErrReadOnly, rom.ErrSize} {
		if errors.Is(err, target) {
			return locales.Catalog.Format(i18n.Locale(), strings.ReplaceAll(target.Error(), ".", "_"), nil)
		}
	}
	return locales.Error() + " " + err.Error()
}
