package corp

import (
	"os"

	"gopkg.in/yaml.v2"
)

// CorpIdentMap holds the company identifiers.
type CorpIdentMap map[uint16]string

// Resolve returns the company name for a given identifier.
func (c CorpIdentMap) Resolve(t uint16) string {
	if val, ok := c[t]; ok {
		return val
	}
	return "Unknown"
}

// IngestCorpDevices converts a YAML list into a hashmap of Corporate identifiers.
func IngestCorpDevices(loc string) (CorpIdentMap, error) {
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
		return nil, err
	}
	defer file.Close()
	// Create a new YAML decoder.
	d := yaml.NewDecoder(file)
	// Create a new struct to hold the unmarshaled data.
	var c CompanyIdentifiers

	// Decode the file into the struct.
	err = d.Decode(&c)
	if err != nil {
		return nil, err
	}
	// Convert YAML struct into a hashmap
	for _, v := range c.CompanyIdentifiers {
		cmap[v.Value] = v.Name
	}
	return cmap, nil
}
