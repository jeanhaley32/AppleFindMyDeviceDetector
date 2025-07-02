package device

import (
	"bytes"
	"time"

	"tinygo.org/x/bluetooth"
)

const (
	appleIdentifier          = byte(0x004C) // 0x004C is the company identifier for Apple.
	findMyNetworkBroadcastID = byte(0x12)   // 0x12 is the broadcast ID for the FindMy network.
	unregisteredFindMyDevice = byte(0x07)   // 0x07 is the broadcast ID for the FindMy network broadcast by an unregistered airtag.
	AirTagPayloadLength      = byte(0x19)   // 0x19 is the length of the AirTag payload.
)

var (
	findMy = map[string][]byte{
		"payloadType":   {unregisteredFindMyDevice, findMyNetworkBroadcastID},
		"payloadLength": {AirTagPayloadLength},
	}
)

// ScanResult is an interface that wraps the methods needed from bluetooth.ScanResult.
type ScanResult interface {
	ManufacturerData() map[uint16][]byte
	Address() bluetooth.Address
	LocalName() string
}

// Device content
type Device struct {
	D         ScanResult
	LastSeen  time.Time
	FirstSeen time.Time
	TimesSeen int
}

// time since first seen
func (d Device) SinceFirstSeen() time.Duration {
	return time.Since(d.FirstSeen)
}

// time since last seen
func (d Device) SinceLastSeen() time.Duration {
	return time.Since(d.LastSeen)
}

// returns the time since the device was first detected and the last time it was detected.
func (d Device) DetectedFor() time.Duration {
	return d.LastSeen.Sub(d.FirstSeen)
}

// list of devices
type List struct {
	Devices   []Device
	ScanCount int
}

// returns the length of the list
// used to satisfy the sort.Interface
func (d List) Len() int {
	return len(d.Devices)
}

// return true if the device id is less than the device id at index j
// used to satisfy the sort.Interface
func (d List) Less(i, j int) bool {
	return d.Devices[j].DetectedFor() < d.Devices[i].DetectedFor()
}

// swaps the devices at index i and j
// used to satisfy the sort.Interface
func (d List) Swap(i, j int) {
	d.Devices[i], d.Devices[j] = d.Devices[j], d.Devices[i]
}

// Returns the device address.
func (d Device) Address() bluetooth.Address {
	return d.D.Address()
}

// Returns the device address as a string.
func (d Device) AddressString() string {
	return d.D.Address().String()
}

func (d Device) ManufacturerData() map[uint16][]byte {
	return d.D.ManufacturerData()
}

// returns the device's local name.
func (d Device) LocalName() string {
	return d.D.LocalName()
}

// returns the device's company uint16 identifier.
func (d Device) CompanyIdent() uint16 {
	if len(d.ManufacturerData()) > 0 {
		for manId := range d.ManufacturerData() {
			return manId
		}
	}
	return 0
}

// Checks if a device is potentiall an Apple AirTag.
func (d *Device) IsAppleAirTag() bool {
	if len(d.ManufacturerData()) == 0 {
		return false
	}
	if val, ok := d.ManufacturerData()[uint16(appleIdentifier)]; ok {
		if len(val) > 0 {
			// check if the first byte is a FindMy network broadcast ID. And the second byte is the correct payload length.
			if bytes.Contains(findMy["payloadType"], val[0:1]) && bytes.Equal(findMy["payloadLength"], val[1:2]) {
				return true
			}
		}
	}
	return false
}

// Returns true if the device is a registered apple air tag
func (d Device) IsTrackingAirtag() bool {
	if d.IsRegistered() && d.IsAppleAirTag() {
		return true
	}
	return false
}

// Checks if a device is potentially an Apple "FindMy" device.
func (d *Device) IsFindMyDevice() bool {
	var findMy = map[string][]byte{
		"payloadType":   {unregisteredFindMyDevice, findMyNetworkBroadcastID},
		"payloadLength": {AirTagPayloadLength},
	}
	// Check if the device is broadcasting any manufacterer specific data.
	if len(d.ManufacturerData()) == 0 {
		return false
	}
	// pulls Apple manufacturer data from the device.
	if val, ok := d.ManufacturerData()[uint16(appleIdentifier)]; ok {
		if len(val) > 0 {
			// Looks for a "findMy" AD type.
			if bytes.Contains(findMy["payloadType"], val[0:1]) {
				return true
			}
		}
	}
	return false
}

// Check if AirTag is registered or unregistered.
func (d Device) IsRegistered() bool {
	if len(d.ManufacturerData()) == 0 || !d.IsAppleAirTag() {
		return false
	}
	return d.ManufacturerData()[uint16(appleIdentifier)][0] != unregisteredFindMyDevice
}
