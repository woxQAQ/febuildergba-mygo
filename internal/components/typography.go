package components

// Font sizes are in logical pixels. Roles keep their offsets from FontBody so
// changing the base size adjusts the whole text hierarchy.
const (
	FontBody float32 = 13

	FontAnnotation   = FontBody - 2 // Dense list metadata.
	FontCaption      = FontBody - 1 // Supporting labels, hints and status text.
	FontWindowTitle  = FontBody + 1 // The document name in window chrome.
	FontPanelTitle   = FontBody + 3 // A preview or review within a page.
	FontSectionTitle = FontBody + 5 // The selected resource's section heading.
	FontTitle        = FontBody + 7 // Page and record titles.
)
