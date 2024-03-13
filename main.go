// Test
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
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
	scannerRunTime           = 10           // The number of seconds to run the scanner.
	companyIdentlocation     = "company_identifiers.yaml"
	scanWait                 = time.Duration(1 * time.Second)
)

type manufacturerData map[uint16][]byte

type devContent struct {
	manufacturerData map[uint16][]byte
	localName        string
	companyIdent     uint16
}

type trackingDevices map[uint16]map[uint16]devContent

var (
	adapter = bluetooth.DefaultAdapter
	devices = make(trackingDevices)
)

func main() {
	// Enable BLE interface.
	ptab := table.NewWriter()
	ptab.SetOutputMirror(os.Stdout)
	ptab.AppendHeader(table.Row{"Device ID", "Name", "Company", "FindMy"})
	must("enable BLE stack", adapter.Enable())
	for {
		// Start scanning.
		fmt.Println("Scanning for nearby Bluetooth devices...")
		t := time.Now()
		err := adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
			if time.Now().After(t.Add(scannerRunTime * time.Second)) {
				fmt.Println("Scan time exceeded")
				adapter.StopScan() // Stop scanning after 10 seconds.
			}
			devices[device.Address.Get16Bit()] = map[uint16]devContent{
				device.Address.Get16Bit(): {
					manufacturerData: device.ManufacturerData(),
					localName:        device.LocalName(),
					companyIdent:     getCompanyIdent(device.ManufacturerData()),
				}}
		})
		must("start scan", err)
		// Print the devices found.
		fmt.Println("Devices found:")
		for k, v := range devices {
			companyName := resolveCompanyIdent(v[k].companyIdent)
			localName := v[k].localName
			findMyDevice := isFindMyDevice(v[k].manufacturerData)
			ptab.AppendRow(table.Row{
				fmt.Sprintf("%x", k),
				localName,
				companyName,
				findMyDevice,
			})
		}
		ptab.SetStyle(table.StyleRounded)
		// clears the screen.
		clearScreen()
		// Render the table.
		ptab.Render()
		// Reset the rows in the table.
		ptab.ResetRows()
		// clear the devices map.
		devices = make(trackingDevices)
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

func getCompanyIdent(md manufacturerData) uint16 {
	if len(md) > 0 {
		for manId := range md {
			return manId
		}
	}
	return 0
}

func resolveCompanyIdent(companyIdent uint16) string {
	// define a map to hold the company identifiers.
	type CompanyIdentifier struct {
		Value uint16 `yaml:"value"`
		Name  string `yaml:"name"`
	}
	type CompanyIdentifiers struct {
		CompanyIdentifiers []CompanyIdentifier `yaml:"company_identifiers"`
	}

	// Open the file and read the contents.
	file, err := os.Open(companyIdentlocation)
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
		// test line to print the company identifiers against the yaml file.
		if v.Value == companyIdent {
			return v.Name
		}
	}

	return "unkown"

}
