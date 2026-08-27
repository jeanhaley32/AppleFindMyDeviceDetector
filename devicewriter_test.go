package main

import (
	"sort"
	"strings"
	"testing"
)

// A device broadcasting more than one manufacturer-data entry must have each row
// labeled with THAT entry's company, not the device-wide (Apple-preferred)
// CompanyID. This is the regression guard for the mislabeled-company bug.
func TestManufacturerCells_LabelsEachEntryWithItsOwnCompany(t *testing.T) {
	names := CompanyIdentifiers{
		appleCompanyID: "Apple, Inc.",
		0x0006:         "Microsoft",
	}
	md := map[uint16][]byte{
		appleCompanyID: {0x12, 0x19},
		0x0006:         {0x01, 0x02, 0x03},
	}

	got := manufacturerCells(md, &names)
	if len(got) != 2 {
		t.Fatalf("manufacturerCells returned %d cells, want 2", len(got))
	}

	// Find each entry by its data payload and check the paired company.
	byData := map[string]string{}
	for _, c := range got {
		byData[c.data] = c.company
	}
	if company := byData["[12 19]: 2"]; company != "Apple, Inc." {
		t.Errorf("Apple payload paired with %q, want %q", company, "Apple, Inc.")
	}
	if company := byData["[1 2 3]: 3"]; company != "Microsoft" {
		t.Errorf("Microsoft payload paired with %q, want %q", company, "Microsoft")
	}
}

// An empty payload must still produce a full data cell so the caller emits a
// full-width row, not a single-column "None" row that misaligns under the header.
func TestManufacturerCells_EmptyPayloadIsFullCell(t *testing.T) {
	names := CompanyIdentifiers{0x0006: "Microsoft"}
	md := map[uint16][]byte{0x0006: {}}

	got := manufacturerCells(md, &names)
	if len(got) != 1 {
		t.Fatalf("manufacturerCells returned %d cells, want 1", len(got))
	}
	if got[0].data != "(none)" {
		t.Errorf("empty payload data cell = %q, want %q", got[0].data, "(none)")
	}
	if got[0].company != "Microsoft" {
		t.Errorf("empty payload company = %q, want %q", got[0].company, "Microsoft")
	}
}

// Unknown company IDs fall through to the getCompanyName default rather than
// dropping the entry.
func TestManufacturerCells_UnknownCompany(t *testing.T) {
	names := CompanyIdentifiers{}
	md := map[uint16][]byte{0x9999: {0xAB}}

	got := manufacturerCells(md, &names)
	if len(got) != 1 {
		t.Fatalf("manufacturerCells returned %d cells, want 1", len(got))
	}
	if got[0].company != "Unknown" {
		t.Errorf("unknown company = %q, want %q", got[0].company, "Unknown")
	}
	if !strings.Contains(got[0].data, "AB") {
		t.Errorf("data cell %q missing hex payload", got[0].data)
	}
}

// Guard the invariant directly: every entry's rendered hex must belong to the
// company it is paired with, across many randomized map-iteration orders.
func TestManufacturerCells_NoCrossPairing(t *testing.T) {
	names := CompanyIdentifiers{appleCompanyID: "Apple, Inc.", 0x0006: "Microsoft"}
	want := map[string]string{"Apple, Inc.": "[12]: 1", "Microsoft": "[34]: 1"}
	md := map[uint16][]byte{appleCompanyID: {0x12}, 0x0006: {0x34}}

	for i := 0; i < 50; i++ {
		got := manufacturerCells(md, &names)
		pairs := make([]string, 0, len(got))
		for _, c := range got {
			if want[c.company] != c.data {
				t.Fatalf("company %q paired with %q, want %q", c.company, c.data, want[c.company])
			}
			pairs = append(pairs, c.company)
		}
		sort.Strings(pairs)
		if strings.Join(pairs, ",") != "Apple, Inc.,Microsoft" {
			t.Fatalf("missing a company in output: %v", pairs)
		}
	}
}
