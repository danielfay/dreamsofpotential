package game

import "testing"

func TestResourceFamilyRegistryConsistency(t *testing.T) {
	for i := range resourceFamilies {
		fam := &resourceFamilies[i]
		if fam.Stockpile == nil || fam.LocalStockpile == nil || fam.AbstractRate == nil ||
			fam.ProjectedRate == nil || fam.AwakenReq == nil || fam.AwakenFill == nil ||
			fam.Estimate == nil {
			t.Fatalf("resourceFamilies[%d] has nil accessors", i)
		}
		if got := familyForResource(fam.Resource); got != fam {
			t.Fatalf("familyForResource(%v) did not round-trip to row %d", fam.Resource, i)
		}
		if got := familyForPotential(fam.Potential); got != fam {
			t.Fatalf("familyForPotential(%v) did not round-trip to row %d", fam.Potential, i)
		}
	}
}
