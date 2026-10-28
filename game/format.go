package game

const (
	fe6ID  = "FE6"
	fe7UID = "FE7U"
	fe7JID = "FE7J"
	fe8UID = "FE8U"
	fe8JID = "FE8J"
)

// gameFormat groups the locations and encodings consumed by the shared editors.
type gameFormat struct {
	id, header  string
	minimum     int
	tables      []tableRule
	chapters    chapterFormat
	deployments deploymentFormat
	maps        mapFormat
	text        textFormat
	portraits   portraitFormat
	icons       iconFormat
	features    func([]byte) (map[byte]string, []string)
}

func (g gameFormat) ID() string { return g.id }

func (g gameFormat) Match(data []byte) bool {
	return len(data) >= g.minimum && string(data[0xAC:0xB2]) == g.header
}

func (g gameFormat) Inspect(data []byte, layouts map[Kind]Layout) Inspection {
	var aliases map[byte]string
	var result Inspection
	if g.features != nil {
		aliases, result.Patches = g.features(data)
	}
	nameOf := func(id uint16) (string, error) { return g.text.readName(data, id, aliases) }
	for _, rule := range g.tableRules(data) {
		layout, manual := layouts[rule.kind]
		result.Tables = append(
			result.Tables,
			rule.read(data, layout, manual, nameOf),
		)
	}
	return result
}

func (g gameFormat) Preview(data []byte, ref AssetRef) (Preview, error) {
	switch ref.Kind {
	case PortraitAsset:
		return g.portraits.read(data, ref.ID)
	case ItemIconAsset:
		return g.icons.read(data, ref.ID)
	default:
		return Preview{}, ErrImage
	}
}
