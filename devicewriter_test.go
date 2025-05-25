package main

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"tinygo.org/x/bluetooth"
)

// Mock bluetooth.Address for testing (can be shared if in same package or defined per test file)
type mockDWAddress struct {
	addr string
}

func (m mockDWAddress) String() string     { return m.addr }
func (m mockDWAddress) IsRandom() bool     { return false }
func (m mockDWAddress) SetRandom(b bool) {}

// Mock bluetooth.ScanResult for testing
type mockDWScanResult struct {
	addr             bluetooth.Address
	manufacturerData map[uint16][]byte
	rssi             int16
	localName        string
}

func (m mockDWScanResult) Address() bluetooth.Address             { return m.addr }
func (m mockDWScanResult) ManufacturerData() map[uint16][]byte    { return m.manufacturerData }
func (m mockDWScanResult) RSSI() int16                            { return m.rssi }
func (m mockDWScanResult) LocalName() string                      { return m.localName }
func (m mockDWScanResult) ServiceUUIDs() []bluetooth.UUID         { return nil }
func (m mockDWScanResult) Bytes() []byte                          { return nil }
func (m mockDWScanResult) AdvertisementPayload() bluetooth.AdvertisementPayload { return nil }


// Mock device methods needed for prepareTableRows
// We need to attach these to the 'device' struct used in screenWriter.
// This is a bit tricky as 'device' is defined in blescan.go.
// For a true unit test of devicewriter.go, 'device' itself could be an interface,
// or we provide a test version of 'device' here.
// Given the current structure, we'll assume the 'device' struct from blescan.go
// and make sure our mockDWScanResult can satisfy its 'd' field.
// The methods isAppleAirTag and isRegistered are on the device struct.
// We will create device instances directly for testing prepareTableRows.

func (d *device) dwIsAppleAirTag(isAirTag bool) {
	// This is a helper to control the outcome for testing
	// In a real scenario, this logic is in blescan.go
	// For testing prepareTableRows, we care about the output based on this.
	// This approach is a bit of a hack. A better way would be an interface for device.
	// For now, we'll rely on constructing the ManufacturerData to match.
}

func (d *device) dwIsRegistered(isRegistered bool) {
    // Similar to dwIsAppleAirTag
}


func TestScreenWriter_PrepareTableRows(t *testing.T) {
	sampleCorpMap := CorpIdentMap{
		76: "Apple, Inc.",
		0:  "Unknown", // For devices with no company ID
	}

	writer := &screenWriter{
		corpMap: sampleCorpMap,
		// ptab, header, etc., are not directly used by prepareTableRows
	}

	// Define common times for easier assertion
	now := time.Now()
	firstSeenTime := now.Add(-10 * time.Minute)
	lastSeenTime := now.Add(-1 * time.Minute)
	detectedFor := lastSeenTime.Sub(firstSeenTime)

	tests := []struct {
		name             string
		deviceList       deviceList
		termHeight       int
		expectedNumRows  int
		expectedRowParts []table.Row // Check specific parts of rows
	}{
		{
			name:            "Empty device list",
			deviceList:      deviceList{devices: []device{}, scanCount: 0},
			termHeight:      20,
			expectedNumRows: 0,
		},
		{
			name: "One item, AirTag, Registered, No ManuData",
			deviceList: deviceList{
				devices: []device{
					{
						d: mockDWScanResult{
							addr:             mockDWAddress{addr: "DEV1"},
							manufacturerData: map[uint16][]byte{76: {findMyNetworkBroadcastID, AirTagPayloadLength, 0x01}}, // Registered AirTag
						},
						firstSeen: firstSeenTime,
						lastSeen:  lastSeenTime,
						timesSeen: 5,
					},
				},
				scanCount: 10,
			},
			termHeight:      20,
			expectedNumRows: 1,
			expectedRowParts: []table.Row{
				{"DEV1", "Apple, Inc.", "[1]: 1", "*", "*", fmt.Sprintf("%v:%v:%v", firstSeenTime.Round(time.Second), lastSeenTime.Round(time.Second), detectedFor.Round(time.Second)), 5, "50%"},
			},
		},
		{
			name: "One item, Not AirTag, Not Registered, With ManuData",
			deviceList: deviceList{
				devices: []device{
					{
						d: mockDWScanResult{
							addr:             mockDWAddress{addr: "DEV2"},
							manufacturerData: map[uint16][]byte{76: {0x00, 0x00, 0xDE, 0xAD}}, // Not an AirTag payload
						},
						firstSeen: firstSeenTime,
						lastSeen:  lastSeenTime,
						timesSeen: 2,
					},
				},
				scanCount: 8,
			},
			termHeight:      20,
			expectedNumRows: 1,
			expectedRowParts: []table.Row{
				{"DEV2", "Apple, Inc.", "[DE AD]: 2", "", "", fmt.Sprintf("%v:%v:%v", firstSeenTime.Round(time.Second), lastSeenTime.Round(time.Second), detectedFor.Round(time.Second)), 2, "25%"},
			},
		},
		{
			name: "One item, AirTag, Unregistered",
			deviceList: deviceList{
				devices: []device{
					{
						d: mockDWScanResult{
							addr:             mockDWAddress{addr: "DEV3"},
							manufacturerData: map[uint16][]byte{76: {unregisteredFindMyDevice, AirTagPayloadLength, 0xBE, 0xEF}}, // Unregistered AirTag
						},
						firstSeen: firstSeenTime,
						lastSeen:  lastSeenTime,
						timesSeen: 10,
					},
				},
				scanCount: 10,
			},
			termHeight:      20,
			expectedNumRows: 1,
			expectedRowParts: []table.Row{
				{"DEV3", "Apple, Inc.", "[BE EF]: 2", "*", "", fmt.Sprintf("%v:%v:%v", firstSeenTime.Round(time.Second), lastSeenTime.Round(time.Second), detectedFor.Round(time.Second)), 10, "100%"},
			},
		},
		{
			name: "Manufacturer data formatting - empty data for a company ID",
			deviceList: deviceList{
				devices: []device{
					{
						d: mockDWScanResult{
							addr:             mockDWAddress{addr: "DEV4"},
							manufacturerData: map[uint16][]byte{76: {}}, // Apple, but empty data
						},
						// other fields as needed
					},
				},
				scanCount: 1,
			},
			termHeight:      20,
			expectedNumRows: 1,
			expectedRowParts: []table.Row{
				{"DEV4", "Apple, Inc.", "[]: 0", "", "", "0s:0s:0s", 0, "0%"}, // Times will be zero if not set
			},
		},
		{
			name: "Manufacturer data formatting - no manufacturer data at all for device",
			deviceList: deviceList{
				devices: []device{
					{
						d: mockDWScanResult{
							addr:             mockDWAddress{addr: "DEV5"},
							manufacturerData: map[uint16][]byte{}, // No manufacturer data entries
						},
						// other fields as needed
					},
				},
				scanCount: 1,
			},
			termHeight:      20,
			expectedNumRows: 1,
			expectedRowParts: []table.Row{
				{"DEV5", "Unknown", "None", "", "", "0s:0s:0s", 0, "0%"},
			},
		},
		{
            name: "Device list shorter than termHeight-rowBuff",
            deviceList: deviceList{
                devices: []device{
                    {d: mockDWScanResult{addr: mockDWAddress{addr: "Short1"}}},
                    {d: mockDWScanResult{addr: mockDWAddress{addr: "Short2"}}},
                },
                scanCount: 2,
            },
            termHeight:      10, // rowBuff is 5, so 10-5=5. List length 2.
            expectedNumRows: 2,
        },
        {
            name: "Device list longer than termHeight-rowBuff",
            deviceList: deviceList{
                devices: []device{
                    {d: mockDWScanResult{addr: mockDWAddress{addr: "Long1"}}},
                    {d: mockDWScanResult{addr: mockDWAddress{addr: "Long2"}}},
                    {d: mockDWScanResult{addr: mockDWAddress{addr: "Long3"}}},
                },
                scanCount: 3,
            },
            termHeight:      7, // rowBuff is 5, so 7-5=2. List length 3, expect 2 rows.
            expectedNumRows: 2,
        },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For these tests, we need to ensure the device methods isAppleAirTag and isRegistered
			// (which are defined in blescan.go) behave as expected for the test case.
			// The ManufacturerData in mockDWScanResult is constructed to achieve this.
			// This is a bit of an integration test for that aspect.

			rows := writer.prepareTableRows(tt.deviceList, tt.termHeight)
			if len(rows) != tt.expectedNumRows {
				t.Errorf("prepareTableRows() returned %d rows, want %d", len(rows), tt.expectedNumRows)
			}

			if tt.expectedRowParts != nil {
				for i, expectedParts := range tt.expectedRowParts {
					if i >= len(rows) {
						t.Errorf("Expected row part check for row index %d, but only got %d rows", i, len(rows))
						continue
					}
					currentRow := rows[i]
					// Compare relevant parts. Assuming order: DevID, Manu, ManuData, AirTag, Reg, Times, Seen, Percent
					if len(currentRow) < 8 || len(expectedParts) < 8 {
						t.Errorf("Row %d or expectedParts has too few elements for comparison", i)
						continue
					}
					// Dev ID
					if currentRow[0] != expectedParts[0] {
						t.Errorf("Row %d, Dev ID: got %v, want %v", i, currentRow[0], expectedParts[0])
					}
					// Manufacturer
					if currentRow[1] != expectedParts[1] {
						t.Errorf("Row %d, Manufacturer: got %v, want %v", i, currentRow[1], expectedParts[1])
					}
					// ManuData
					if currentRow[2] != expectedParts[2] {
						t.Errorf("Row %d, ManuData: got %v, want %v", i, currentRow[2], expectedParts[2])
					}
					// AirTag
					if currentRow[3] != expectedParts[3] {
						t.Errorf("Row %d, AirTag: got %v, want %v", i, currentRow[3], expectedParts[3])
					}
					// Registered
					if currentRow[4] != expectedParts[4] {
						t.Errorf("Row %d, Registered: got %v, want %v", i, currentRow[4], expectedParts[4])
					}
					// Times (First:Last:Delta) - this can be fragile if exact time func is not perfectly mimicked
					// For simplicity, we are checking the string representation passed in expected parts
					if currentRow[5] != expectedParts[5] {
						t.Errorf("Row %d, Times: got %v, want %v", i, currentRow[5], expectedParts[5])
					}
					// Times Seen
					if !reflect.DeepEqual(currentRow[6],expectedParts[6]) { // Use DeepEqual for int vs interface{}
						t.Errorf("Row %d, TimesSeen: got %v (type %T), want %v (type %T)", i, currentRow[6], currentRow[6], expectedParts[6], expectedParts[6])
					}
					// Percent Seen
					if currentRow[7] != expectedParts[7] {
						t.Errorf("Row %d, PercentSeen: got %v, want %v", i, currentRow[7], expectedParts[7])
					}
				}
			}
		})
	}
}

// Globals from blescan.go needed for device methods if we were to call them directly.
// However, for prepareTableRows, we are checking the *output* based on how
// ManufacturerData is set up.
var (
	findMy map[string][]byte = map[string][]byte{
		"payloadType":   {unregisteredFindMyDevice, findMyNetworkBroadcastID},
		"payloadLength": {AirTagPayloadLength},
	}
)

// Constants from blescan.go
const (
	appleIdentifier          = byte(0x004C)
	findMyNetworkBroadcastID = byte(0x12)
	unregisteredFindMyDevice = byte(0x07)
	AirTagPayloadLength      = byte(0x19)
)
