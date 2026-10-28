package recordform

import (
	"github.com/egoist/mygo/ui"
)

func (a *Form) assetEditor(c *ui.Context, preview *assetPreview) {
	if preview == nil || preview.bitmap == nil {
		return
	}
	a.imageEditor.Render(c, a.state, preview.ref, a.assets)
}
