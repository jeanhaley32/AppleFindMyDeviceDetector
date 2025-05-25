package main

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

// map to hold the company identifiers.
type CorpIdentMap map[uint16]string

const (
	companyIdentlocation = "company_identifiers.yaml"
)

// resolve coporate identity into a string
func resolveCompanyIdent(c *CorpIdentMap, t uint16) string {
	if val, ok := (*c)[t]; ok {
		return val
	}
	return "Unknown"
}

// converts YAML list into a hashmap of Corporate identifiers
func ingestCorpDevices(loc string) (CorpIdentMap, error) {
	cmap := make(CorpIdentMap)
	// define a map to hold individual company identifiers.
	type CompanyIdentifier struct {
		Value uint16 `yaml:"value"`
		Name  string `yaml:"name"`
	}
	// define a struct to the top level company identifiers list.
	type CompanyIdentifiers struct {
		CompanyIdentifiers []CompanyIdentifier `yaml:"company_identifiers"`
	}

	// Open the file and read the contents.
	file, err := os.Open(loc)
	if err != nil {
		return nil, fmt.Errorf("failed to open company identifiers file %s: %w", loc, err)
	}
	defer file.Close()
	// Create a new YAML decoder.
	d := yaml.NewDecoder(file)
	// Create a new struct to hold the unmarshaled data.
	var c CompanyIdentifiers

	// Decode the file into the struct.
	err = d.Decode(&c)
	if err != nil {
		return nil, fmt.Errorf("failed to decode company identifiers file %s: %w", loc, err)
	}
	// Convert YAML struct into a hashmap
	for _, v := range c.CompanyIdentifiers {
		cmap[v.Value] = v.Name
	}
	return cmap, nil
}

func getCompanyIdent(md manData) uint16 {
	if len(md) > 0 {
		for manId := range md {
			return manId
		}
	}
	return 0
}
