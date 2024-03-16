package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"gopkg.in/yaml.v2"
)

const (
	companyIdentlocation     = "company_identifiers.yaml"
	adManSpecData            = byte(0xFF)   // 0xFF is the AD type for manufacturer specific data.
	appleIdentifier          = byte(0x004C) // 0x004C is the company identifier for Apple.
	findMyNetworkBroadcastID = byte(0x12)   // 0x12 is the broadcast ID for the FindMy network.
)

type devValue uint16
type devName string

// map to hold the company identifiers.
type CorpIdentMap map[devValue]devName

var (
	cmap = make(CorpIdentMap)
)

func main() {
	// Load the company identifiers into a map.
	cmap = ingestCorpDevices(companyIdentlocation)
	// create ingestion path
	ingp := make(ingestPath)
	// create a new table writer.
	ptab := table.NewWriter()
	// Set the output to stdout.
	ptab.SetOutputMirror(os.Stdout)
	// Set the table headers.
	ptab.AppendHeader(table.Row{"Device ID", "Name", "Company", "FindMy"})
	// Create a wait group to wait for the scanner to finish. we're not actually doing that in this iteration
	wg := sync.WaitGroup{}
	// start the scanner in a go routine.
	go startBleScanner(&wg, &ingp)
	// listen for devices on the ingestion path.
	for devices := range ingp {
		fmt.Println("Devices found:")
		devices.Range(func(k, v interface{}) bool {
			// iterate over the devices and add them to the table.
			for k, v := range v.(map[uint16]devContent) {
				companyName := resolveCompanyIdent(&cmap, v.companyIdent)
				localName := v.localName
				findMyDevice := isFindMyDevice(v.manufacturerData)
				ptab.AppendRow(table.Row{
					fmt.Sprintf("%x", k),
					localName,
					companyName,
					findMyDevice,
				})
			}
			return true
		})
		// Set the table style.
		ptab.SetStyle(table.StyleRounded)
		// clears the screen.
		clearScreen()
		// Render the table.
		ptab.Render()
		// Reset the rows in the table.
		ptab.ResetRows()
		// Wait x seconds before scanning again.
		time.Sleep(scanWait)
	}
}

// checks if byte 0 is the FindMy network broadcast ID.
func isFindMyDevice(b map[uint16][]byte) bool {
	if len(b) == 0 {
		return false
	}
	for _, v := range b {
		if v[0] == findMyNetworkBroadcastID {
			return true
		}
	}
	return false
}

func getCompanyIdent(md manData) uint16 {
	if len(md) > 0 {
		for manId := range md {
			return manId
		}
	}
	return 0
}

// resolve coporate identity into a string
func resolveCompanyIdent(c *CorpIdentMap, t devValue) devName {
	if val, ok := (*c)[t]; ok {
		return val
	}
	return "Unknown"
}

// converts YAML list into a hashmap of Corporate identifiers
func ingestCorpDevices(loc string) CorpIdentMap {
	cmap = make(CorpIdentMap)
	// define a map to hold individual company identifiers.
	type CompanyIdentifier struct {
		Value devValue `yaml:"value"`
		Name  devName  `yaml:"name"`
	}
	// define a struct to the top level company identifiers list.
	type CompanyIdentifiers struct {
		CompanyIdentifiers []CompanyIdentifier `yaml:"company_identifiers"`
	}

	// Open the file and read the contents.
	file, err := os.Open(loc)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	// Create a new YAML decoder.
	d := yaml.NewDecoder(file)
	// Create a new struct to hold the unmarshaled data.
	var c CompanyIdentifiers

	// Decode the file into the struct.
	err = d.Decode(&c)
	if err != nil {
		log.Fatal(err)
	}
	// Convert YAML struct into a hashmap
	for _, v := range c.CompanyIdentifiers {
		cmap[v.Value] = v.Name
	}
	return cmap
}
