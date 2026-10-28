package testutil

// NewPathFixture gives two FE8 units a shared three-step REDA resource, with
// synchronization, signed-delay edge bits and nonzero reserved configuration bits.
func NewPathFixture(version string) *FixtureBuilder {
	f := NewDeploymentFixture(version)
	if version != versionFE8U && version != versionFE8J {
		panic("FE8 paths required")
	}
	units, path := f.GetPosition("deploy_units"), f.GetPosition("deploy_path")
	f.WriteAt(path, []byte{
		0xC2, 0x50, 0xAC, 4, 1, 0, 12, 0,
		0x44, 0xA1, 0xD6, 0, 0xFF, 0xFF, 0, 0x80,
		0xC6, 0x01, 0xBD, 1, 0, 0, 60, 0,
	})
	f.WriteU8(units+7, 3)
	f.WriteU8(units+20+7, 3)
	f.WritePointer(units+20+8, path)
	return f
}
