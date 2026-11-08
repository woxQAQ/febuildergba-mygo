// Package icons holds the MyGo icons generated from @iconify-json/* sets.
// Regenerate with just icons and call icons.<Set>.<Icon>(). Each method parses
// its SVG on first use and safely shares it across callers. Unused methods,
// caches and SVG strings can be linked out.
package icons

import (
	"sync"

	"github.com/egoist/mygo/ui"
)

type cachedIcon struct {
	once sync.Once
	svg  *ui.SVG
}
