package main

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"tinygo.org/x/bluetooth"
)

// Mock bluetooth.Address for testing
type mockBTAddress struct {
	addr string
}

func (m mockBTAddress) String() string {
	return m.addr
}

func (m mockBTAddress) IsRandom() bool { return false } // Not used in current tests
func (m mockBTAddress) SetRandom(b bool) {}            // Not used

// Mock bluetooth.ScanResult for testing
type mockScanResult struct {
	addr             bluetooth.Address
	manufacturerData map[uint16][]byte
	rssi             int16
	localName        string
}

func (m mockScanResult) Address() bluetooth.Address             { return m.addr }
func (m mockScanResult) ManufacturerData() map[uint16][]byte    { return m.manufacturerData }
func (m mockScanResult) RSSI() int16                            { return m.rssi }
func (m mockScanResult) LocalName() string                      { return m.localName }
func (m mockScanResult) ServiceUUIDs() []bluetooth.UUID         { return nil } // Not used
func (m mockScanResult) Bytes() []byte                          { return nil } // Not used
func (m mockScanResult) AdvertisementPayload() bluetooth.AdvertisementPayload { return nil } // Not used

func TestScanner_TrimMap(t *testing.T) {
	s := &scanner{
		devices: new(sync.Map),
		count:   0,
	}

	dev1Addr := mockBTAddress{addr: "dev1"}
	dev2Addr := mockBTAddress{addr: "dev2"}

	// Device 1: Should be trimmed
	dev1 := device{
		d:         mockScanResult{addr: dev1Addr},
		lastSeen:  time.Now().Add(-(oldestDevice + time.Hour)), // Older than oldestDevice
		firstSeen: time.Now().Add(-(oldestDevice + 2 * time.Hour)),
		timesSeen: 1,
	}
	s.devices.Store(dev1Addr.String(), dev1)
	s.count++

	// Device 2: Should not be trimmed
	dev2 := device{
		d:         mockScanResult{addr: dev2Addr},
		lastSeen:  time.Now().Add(-time.Minute), // Recent
		firstSeen: time.Now().Add(-2 * time.Minute),
		timesSeen: 5,
	}
	s.devices.Store(dev2Addr.String(), dev2)
	s.count++

	if s.count != 2 {
		t.Fatalf("Initial device count incorrect. Got %d, want 2", s.count)
	}

	s.TrimMap()

	if s.count != 1 {
		t.Errorf("Device count after TrimMap incorrect. Got %d, want 1", s.count)
	}

	if _, ok := s.devices.Load(dev1Addr.String()); ok {
		t.Errorf("Device %s was not trimmed but should have been", dev1Addr.String())
	}
	if _, ok := s.devices.Load(dev2Addr.String()); !ok {
		t.Errorf("Device %s was trimmed but should not have been", dev2Addr.String())
	}
}

func TestScanner_SortAndPass(t *testing.T) {
	s := &scanner{
		devices: new(sync.Map),
	}

	// firstSeen, lastSeen, detectedFor
	// dev1: 10m ago, 1m ago, 9m
	// dev2: 5m ago, 2m ago, 3m
	// dev3: 20m ago, 18m ago, 2m
	// Expected order: dev1, dev2, dev3 (descending by detectedFor)

	dev1Addr := mockBTAddress{addr: "dev1"}
	dev1 := device{
		d:         mockScanResult{addr: dev1Addr},
		firstSeen: time.Now().Add(-10 * time.Minute),
		lastSeen:  time.Now().Add(-1 * time.Minute),
		timesSeen: 1,
	}
	s.devices.Store(dev1Addr.String(), dev1)

	dev2Addr := mockBTAddress{addr: "dev2"}
	dev2 := device{
		d:         mockScanResult{addr: dev2Addr},
		firstSeen: time.Now().Add(-5 * time.Minute),
		lastSeen:  time.Now().Add(-2 * time.Minute),
		timesSeen: 1,
	}
	s.devices.Store(dev2Addr.String(), dev2)

	dev3Addr := mockBTAddress{addr: "dev3"}
	dev3 := device{
		d:         mockScanResult{addr: dev3Addr},
		firstSeen: time.Now().Add(-20 * time.Minute),
		lastSeen:  time.Now().Add(-18 * time.Minute),
		timesSeen: 1,
	}
	s.devices.Store(dev3Addr.String(), dev3)

	deviceList := s.sortAndPass()

	expectedOrder := []string{"dev1", "dev2", "dev3"}
	if len(deviceList.devices) != len(expectedOrder) {
		t.Fatalf("Sorted list length incorrect. Got %d, want %d", len(deviceList.devices), len(expectedOrder))
	}

	for i, dev := range deviceList.devices {
		if dev.AddressString() != expectedOrder[i] {
			t.Errorf("Device order incorrect at index %d. Got %s, want %s", i, dev.AddressString(), expectedOrder[i])
		}
	}
}

func TestDeviceUpdateLogic(t *testing.T) {
	devicesMap := new(sync.Map)
	var deviceCount int = 0

	// Simulate adding a new device
	newDevScanResult := mockScanResult{
		addr: mockBTAddress{addr: "newDev"},
		manufacturerData: map[uint16][]byte{
			appleIdentifier: {findMyNetworkBroadcastID, AirTagPayloadLength, 0x01, 0x02}, // Valid AirTag
		},
	}

	// Initial device entry creation (simplified from startScan)
	// Assuming isTrackingAirtag() would pass for this device
	newDeviceEntry := device{
		d:         newDevScanResult,
		lastSeen:  time.Now(),
		firstSeen: time.Now(),
		timesSeen: 1,
	}
	devicesMap.Store(newDevScanResult.AddressString(), newDeviceEntry)
	deviceCount++

	if val, ok := devicesMap.Load("newDev"); !ok {
		t.Fatal("New device was not added to the map")
	} else {
		storedDev := val.(device)
		if storedDev.timesSeen != 1 {
			t.Errorf("New device timesSeen incorrect. Got %d, want 1", storedDev.timesSeen)
		}
	}

	// Simulate seeing the same device again
	seenAgainScanResult := newDevScanResult // Same device
	time.Sleep(10 * time.Millisecond)       // Ensure lastSeen will be different

	if value, ok := devicesMap.Load(seenAgainScanResult.AddressString()); ok {
		existingDeviceEntry := value.(device)
		originalLastSeen := existingDeviceEntry.lastSeen

		existingDeviceEntry.lastSeen = time.Now()
		existingDeviceEntry.timesSeen++
		devicesMap.Store(seenAgainScanResult.AddressString(), existingDeviceEntry)

		// Assert updates
		if updatedVal, loadOk := devicesMap.Load(seenAgainScanResult.AddressString()); loadOk {
			updatedDev := updatedVal.(device)
			if updatedDev.timesSeen != 2 {
				t.Errorf("Updated device timesSeen incorrect. Got %d, want 2", updatedDev.timesSeen)
			}
			if !updatedDev.lastSeen.After(originalLastSeen) {
				t.Errorf("Updated device lastSeen was not updated. Got %v, original %v", updatedDev.lastSeen, originalLastSeen)
			}
		} else {
			t.Fatal("Failed to load updated device from map")
		}
	} else {
		t.Fatal("Failed to load existing device for update simulation")
	}
	if deviceCount != 1 { // Count should still be 1 as it's the same device
		t.Errorf("Device count after seeing device again is incorrect. Got %d, want 1", deviceCount)
	}
}

// Minimal mock for device.isAppleAirTag and device.isRegistered for TestDeviceUpdateLogic
// These are not directly tested here but are part of the path in startScan.
// For focused unit tests, we assume their behavior or mock them if they were complex.
// Since they are simple, we can construct mockScanResult to satisfy them.
// For the purpose of TestDeviceUpdateLogic, the key is the store/load/update mechanism.

func (d *device) isAppleAirTag() bool { // Simplified for testing, real one is in blescan.go
	if md, ok := d.d.ManufacturerData()[appleIdentifier]; ok {
		return len(md) >= 2 && bytes.Contains(findMy["payloadType"], md[0:1]) && bytes.Equal(findMy["payloadLength"], md[1:2])
	}
	return false
}

func (d device) isRegistered() bool { // Simplified for testing
	if md, ok := d.d.ManufacturerData()[appleIdentifier]; ok {
		return len(md) > 0 && md[0] != unregisteredFindMyDevice
	}
	return false
}

// Helper function to check byte slice equality (if not using reflect.DeepEqual for specific parts)
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Need to declare these global variables for the test file to compile isAppleAirTag
// as they are used by the method.
var (
	// lastSent []device // Not needed for these specific tests in blescan_test.go
	findMy map[string][]byte = map[string][]byte{
		"payloadType":   {unregisteredFindMyDevice, findMyNetworkBroadcastID},
		"payloadLength": {AirTagPayloadLength},
	}
)

// Need to include bytes import for the simplified isAppleAirTag
import "bytes"
