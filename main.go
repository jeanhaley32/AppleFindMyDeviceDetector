package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"gopkg.in/yaml.v2"
	"tinygo.org/x/bluetooth"
)

const (
	// The UUID of an Apple findmy device.
	appleIdentifier          = byte(0x004C) // 0x004C is the company identifier for Apple.
	findMyNetworkBroadcastID = byte(0x12)   // 0x12 is the broadcast ID for the FindMy network.
	adManSpecData            = byte(0xFF)   // 0xFF is the AD type for manufacturer specific data.
	companyIdentlocation     = "company_identifiers.yaml"
	scanWait                 = time.Duration(1 * time.Second)
	scanTime                 = time.Duration(5 * time.Second)
)

// bluetooth devices manufacturer data.
type manData map[uint16][]byte

type devContent struct {
	manufacturerData map[uint16][]byte
	localName        devName
	companyIdent     devValue
}

// map to hold the devices found.
type bleDevs map[uint16]map[uint16]devContent

type devValue uint16
type devName string

// map to hold the company identifiers.
type CorpIdentMap map[devValue]devName

var (
	adapter = bluetooth.DefaultAdapter
	devices sync.Map //make(bleDevs)
	cmap    = make(CorpIdentMap)
)

func main() {
	// Ingest the company identifiers.
	cmap = ingestCorpDevices(companyIdentlocation)
	// Enable BLE interface.
	must("enable BLE stack", adapter.Enable())
	ptab := table.NewWriter()
	ptab.SetOutputMirror(os.Stdout)
	ptab.AppendHeader(table.Row{"Device ID", "Name", "Company", "FindMy"})
	wg := sync.WaitGroup{}
	for {
		// Start scanning.
		fmt.Println("Scanning for nearby Bluetooth devices...")
		t := time.Now()
		wg.Add(1)
		go func() {
			err := adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
				if time.Now().After(t.Add(scanTime)) {
					fmt.Println("Scan time exceeded")
					adapter.StopScan() // Stop scanning after 10 seconds.
				}
				devices.Store(device.Address.Get16Bit(), map[uint16]devContent{
					device.Address.Get16Bit(): {
						manufacturerData: device.ManufacturerData(),
						localName:        devName(device.LocalName()),
						companyIdent:     devValue(getCompanyIdent(device.ManufacturerData())),
					}})
			})
			must("start scan", err)
			wg.Done()
		}()
		wg.Wait()
		// Print the devices found.
		fmt.Println("Devices found:")
		devices.Range(func(k, v interface{}) bool {
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
		// clear the devices map.
		devices = sync.Map{}
		// Wait x seconds before scanning again.
		time.Sleep(scanWait)
	}

}

// must is a helper function that wraps a call to a function returning an error and logs it if the error is non-nil.
func must(action string, err error) {
	if err != nil {
		log.Fatalf("Failed to %s: %v", action, err)
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

func clearScreen() {
	cmd := exec.Command("clear") // Linux or macOS
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls") // Windows
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func getCompanyIdent(md manData) uint16 {
	if len(md) > 0 {
		for manId := range md {
			return manId
		}
	}
	return 0
}

// resolveCompanyIdent returns the name of the company from the ingested company identifiers.
func resolveCompanyIdent(c *CorpIdentMap, t devValue) devName {
	if val, ok := (*c)[t]; ok {
		return val
	}
	return "Unknown"
}

// ingestCorpDevices reads the company identifiers from a YAML file and returns a map of the company identifiers.
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
	// Loop through the company identifiers and return the name of the company.
	for _, v := range c.CompanyIdentifiers {
		cmap[v.Value] = v.Name
	}
	return cmap
}
