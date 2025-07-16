package main

import (
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
	log.Printf("[DEBUG] Resolving company identifier: 0x%04X", t)
	if val, ok := (*c)[t]; ok {
		log.Printf("[DEBUG] Company identifier 0x%04X resolved to: %s", t, val)
		return val
	}
	log.Printf("[DEBUG] Company identifier 0x%04X not found, returning 'Unknown'", t)
	return "Unknown"
}

// converts YAML list into a hashmap of Corporate identifiers
func ingestCorpDevices(loc string) CorpIdentMap {
	log.Printf("[DEBUG] Starting company identifier ingestion from: %s", loc)
	cmap = make(CorpIdentMap)
	// define a map to hold individual company identifiers.
	type CompanyIdentifier struct {
		Value uint16 `yaml:"value"`
		Name  string `yaml:"name"`
	}
	// define a struct to the top level company identifiers list.
	type CompanyIdentifiers struct {
		CompanyIdentifiers []CompanyIdentifier `yaml:"company_identifiers"`
	}

	log.Printf("[DEBUG] Attempting to open company identifiers file: %s", loc)
	// Open the file and read the contents.
	file, err := os.Open(loc)
	if err != nil {
		log.Printf("[DEBUG] Failed to open company identifiers file: %v", err)
		log.Fatal(err)
	}
	defer file.Close()
	log.Printf("[DEBUG] Company identifiers file opened successfully")

	// Create a new YAML decoder.
	d := yaml.NewDecoder(file)
	// Create a new struct to hold the unmarshaled data.
	var c CompanyIdentifiers

	log.Printf("[DEBUG] Decoding YAML content")
	// Decode the file into the struct.
	err = d.Decode(&c)
	if err != nil {
		log.Printf("[DEBUG] Failed to decode YAML: %v", err)
		log.Fatal(err)
	}
	log.Printf("[DEBUG] YAML decoded successfully, found %d company identifiers", len(c.CompanyIdentifiers))

	// Convert YAML struct into a hashmap
	for i, v := range c.CompanyIdentifiers {
		cmap[v.Value] = v.Name
		if i < 5 { // Log first 5 entries for debugging
			log.Printf("[DEBUG] Loaded company identifier: 0x%04X -> %s", v.Value, v.Name)
		}
	}
	log.Printf("[DEBUG] Company identifier ingestion completed - %d identifiers loaded into map", len(cmap))
	return cmap
}

func getCompanyIdent(md manData) uint16 {
	log.Printf("[DEBUG] Extracting company identifier from manufacturer data (entries: %d)", len(md))
	if len(md) > 0 {
		for manId := range md {
			log.Printf("[DEBUG] Found company identifier: 0x%04X", manId)
			return manId
		}
	}
	log.Printf("[DEBUG] No manufacturer data found, returning 0")
	return 0
}
