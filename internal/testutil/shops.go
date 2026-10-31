package testutil

// NewShopFixture includes an independent event header and two locations sharing
// an inventory. Bytes beyond the terminator are unrelated, live ROM content.
func NewShopFixture(version string) *FixtureBuilder {
	f := NewLocationFixture(version)
	const alias = 0x253000
	header, table := f.GetPosition(
		"deploy_header",
	), f.GetPosition(
		"location_table",
	)
	raw := f.Bytes()
	f.WriteAt(alias, raw[header:header+80])
	f.WritePointer(f.GetPosition("deploy_list")+8, alias)
	f.WriteAt(table+11*12, raw[table+6*12:table+7*12])
	f.WriteU32(table+12*12, 0)
	f.WriteAt(
		f.GetPosition("location_inventory")+6,
		[]byte{0xDE, 0xAD, 0xBE, 0xEF},
	)
	f.MarkPosition("shop_alias", alias)
	return f
}
