package testutil

// NewLocationFixture declares five-version tile conditions independently of the
// editor. Rewards and inventory pointers intentionally resemble script operands.
func NewLocationFixture(version string) *FixtureBuilder {
	f := NewObjectiveFixture(version)
	const table, script, inventory, random = 0x250000, 0x251000, 0x252000, 0x252800
	visit, seize, chest, door, shop, center, gold := byte(
		0x0E,
	), byte(
		0x0F,
	), byte(
		0x12,
	), byte(
		0x10,
	), byte(
		0x13,
	), byte(
		0x1D,
	), byte(
		0x76,
	)
	ignored := byte(0xA5)
	switch version {
	case versionFE8U, versionFE8J:
		visit, seize, chest, door, shop, center, gold, ignored = 0x10, 0x11, 0x14, 0x12, 0x16, 0x20, 0x77, 0
	case versionFE6:
		gold = 0x6F
	}
	raw := f.Bytes()
	original := f.GetPosition("talk_script")
	f.WriteAt(script, raw[original:original+32])
	f.WriteAt(inventory, []byte{1, 0xA1, 2, 0xB2, 0, 0xC3})
	f.WriteAt(random, []byte{1, 40, 2, 60, 0, 0xD4})
	rows := [][]byte{
		{5, 0, 0x21, 0, 0, 0, 0, 0, 1, 0, visit, ignored},
		{6, 0, 0x22, 0, 0, 0, 0, 0, 1, 0, visit, ignored},
		{7, 0, 0x23, 0, 2, 0, 0xEF, 0xBE, 1, 0, chest, ignored},
		{7, 0, 0x24, 0, gold, 0, 0xF4, 1, 1, 0, chest, ignored},
		{8, 0, 0x25, 0, 1, 0, 0, 0, 1, 0, door, 0xA5},
		{5, 0, 3, 0, 0, 0, 0, 0, 1, 0, seize, ignored},
		{10, 0, 0, 0, 0, 0, 0, 0, 1, 0, shop, ignored},
		{5, 0, 0x26, 0, 0, 0, 0, 0, 1, 0, chest, ignored},
		{0x7F, 0, 0x77, 0x88, 0, 0, 0, 0, 0xFE, 0xFD, 0xFC, 0xFB},
		{5, 0, 0, 0, 0, 0, 0, 0, 1, 0, center, ignored},
		{5, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0xFE, 0xDA},
	}
	for i, row := range rows {
		f.WriteAt(table+i*12, row)
	}
	for _, i := range []int{0, 1, 7, 8, 9, 10} {
		f.WritePointer(table+i*12+4, script)
	}
	f.WritePointer(table+5*12+4, f.GetPosition("objective_ending"))
	f.WritePointer(table+6*12+4, inventory)
	if ignored == 0 {
		f.WritePointer(table+7*12+4, random)
	}
	f.WriteU32(table+len(rows)*12, 0)
	f.WritePointer(f.GetPosition("deploy_header")+8, table)
	for name, at := range map[string]int{
		"location_table": table, "location_script": script,
		"location_inventory": inventory, "location_random": random,
		"location_terminator": table + len(rows)*12,
	} {
		f.MarkPosition(name, at)
	}
	return f
}
