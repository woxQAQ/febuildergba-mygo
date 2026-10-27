// Package locales contains MyGo-generated catalog accessors and schema label
// resolution. It stays free of domain imports so every layer can depend on it.
package locales

import (
	"strings"

	"github.com/egoist/mygo/i18n"
)

// Label resolves schema keys; stat IDs such as growth.hp share their hp label.
func Label(key string) string {
	if _, tail, ok := strings.Cut(key, "."); ok {
		key = tail
	}
	return Catalog.Format(i18n.Locale(), key, nil)
}
