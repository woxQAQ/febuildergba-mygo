// Package locales contains MyGo-generated catalog accessors and domain label adapters.
package locales

import (
	"errors"
	"strings"

	"github.com/egoist/mygo/i18n"

	"github.com/woxQAQ/febuildergba-mygo/internal/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/rom"
	"github.com/woxQAQ/febuildergba-mygo/internal/space"
)

// Label resolves schema keys; stat IDs such as growth.hp share their hp label.
func Label(key string) string {
	if _, tail, ok := strings.Cut(key, "."); ok {
		key = tail
	}
	return Catalog.Format(i18n.Locale(), key, nil)
}

// ErrorText maps domain errors to catalog messages while retaining unknown details.
func ErrorText(err error) string {
	if err == nil {
		return ""
	}
	key := strings.ReplaceAll(err.Error(), ".", "_")
	if _, _, ok := Catalog.Message(i18n.Locale(), key); ok {
		return Catalog.Format(i18n.Locale(), key, nil)
	}
	for _, target := range []error{game.ErrVersion, game.ErrLayout, game.ErrValue, game.ErrField, game.ErrImage, space.ErrRange, space.ErrCapacity, space.ErrReference, rom.ErrRange, rom.ErrStale, rom.ErrExternalChange, rom.ErrExists, rom.ErrReadOnly, rom.ErrSize} {
		if errors.Is(err, target) {
			return Catalog.Format(i18n.Locale(), strings.ReplaceAll(target.Error(), ".", "_"), nil)
		}
	}
	return Error() + " " + err.Error()
}
