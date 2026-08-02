package main

import (
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

// CompanyIdentifiers maps Bluetooth company IDs to company names.
type CompanyIdentifiers map[uint16]string

const (
	companyIdentifiersFile = "company_identifiers.yaml"
)

// getCompanyName looks up a company name by its Bluetooth company ID.
func getCompanyName(identifiers *CompanyIdentifiers, companyID uint16) string {
	if name, ok := (*identifiers)[companyID]; ok {
		return name
	}
	return "Unknown"
}

// loadCompanyIdentifiers reads company identifiers from a YAML file into a map.
func loadCompanyIdentifiers(filePath string) CompanyIdentifiers {
	identifiers := make(CompanyIdentifiers)

	type companyEntry struct {
		Value uint16 `yaml:"value"`
		Name  string `yaml:"name"`
	}
	type companyFile struct {
		CompanyIdentifiers []companyEntry `yaml:"company_identifiers"`
	}

	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	var data companyFile
	if err := decoder.Decode(&data); err != nil {
		log.Fatal(err)
	}

	for _, company := range data.CompanyIdentifiers {
		identifiers[company.Value] = company.Name
	}
	return identifiers
}

const appleCompanyID uint16 = 0x004C

// extractCompanyID returns a deterministic company ID from manufacturer data:
// Apple's ID if present (this tool is Apple-device focused), otherwise the
// lowest ID. Map iteration order is randomized, so picking "the first" key
// by ranging (the old behavior) flickered between entries on every render
// for devices broadcasting more than one manufacturer-data entry.
func extractCompanyID(mfrData ManufacturerData) uint16 {
	if _, ok := mfrData[appleCompanyID]; ok {
		return appleCompanyID
	}

	first := true
	var lowest uint16
	for companyID := range mfrData {
		if first || companyID < lowest {
			lowest = companyID
			first = false
		}
	}
	return lowest
}
