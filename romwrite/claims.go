package romwrite

import "github.com/woxQAQ/febuildergba-mygo/fault"

var ErrClaim = fault.Code{
	ID:   "space.claim_conflict",
	Kind: fault.Conflict,
}

// Owner identifies the resource a claim belongs to. romwrite does not interpret
// resource meaning; an Owner is only a comparable identity.
type Owner struct {
	Resource string
	ID, Slot int
}

// Claim declares a confirmed byte range and its owner. A Site claim marks a
// writable pointer position; Host names the record containing that position,
// and is zero for standalone sites and for payload extents.
type Claim struct {
	Range

	Owner Owner
	Host  Owner
	Site  bool
}

// CheckPointerWrite verifies that owner may rewrite the pointer site [site, site+4)
// under claims. An overlapping claim is accepted only when it is the owner's
// own site declaration at that position, or a claim owned by the declared
// host and containing the site. Anything else conflicts, including two owners
// declaring the same site.
func CheckPointerWrite(claims []Claim, owner Owner, site int) error {
	siteRange := Range{Start: site, End: site + 4}
	host := Owner{}
	declared := false
	for _, claim := range claims {
		if claim.Site && claim.Range == siteRange && claim.Owner == owner {
			declared = true
			host = claim.Host
		}
	}
	for _, claim := range claims {
		if !claim.overlaps(siteRange) {
			continue
		}
		if claim.Site && claim.Range == siteRange && claim.Owner == owner {
			continue
		}
		if declared && host != (Owner{}) && claim.Owner == host &&
			claim.Start <= site && site+4 <= claim.End {
			continue
		}
		return fault.Wrap(ErrClaim, "space.check_write", fault.Context{
			Resource: claim.Owner.Resource,
			Range:    &fault.Range{Offset: site, Length: 4},
		})
	}
	return nil
}

// CheckRangeWrite checks an in-place payload write against every overlapping
// claim. The owner may rewrite its payload and pointer sites hosted by it;
// owning a pointer does not authorize overwriting that pointer's host.
// Unclaimed bytes are allowed; callers establish the write's bounds and format.
func CheckRangeWrite(claims []Claim, owner Owner, region Range) error {
	for _, claim := range claims {
		if !claim.overlaps(region) {
			continue
		}
		if owner != (Owner{}) && (claim.Owner == owner ||
			(claim.Site && claim.Host == owner)) {
			continue
		}
		return fault.Wrap(ErrClaim, "space.check_range_write", fault.Context{
			Resource: claim.Owner.Resource,
			Range: &fault.Range{
				Offset: region.Start,
				Length: region.End - region.Start,
			},
		})
	}
	return nil
}

// ClaimRanges flattens claims into plain ranges for consumers that reserve
// confirmed resources without rewriting a pointer, such as table moves.
func ClaimRanges(claims []Claim) []Range {
	result := make([]Range, len(claims))
	for i, claim := range claims {
		result[i] = claim.Range
	}
	return result
}
