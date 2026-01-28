package main

// AirTag Protocol Configuration
//
// This file contains all magic bytes and detection logic for Apple AirTag
// and FindMy network devices. Modify these values if Apple updates their
// protocol or if detection stops working.
//
// References:
// - Adam Catley AirTag Reverse Engineering: https://adamcatley.com/AirTag.html
// - furiousMAC Continuity Protocol: https://github.com/furiousMAC/continuity
// - OpenHaystack Project: https://github.com/seemoo-lab/openhaystack
//
// BLE Advertisement Structure:
//   Manufacturer Data key: 0x004C (Apple)
//   Payload: [type][length][status][public_key...][hint]
//
// Apple Continuity Type Bytes (known):
//   0x02 - iBeacon
//   0x05 - AirDrop
//   0x07 - AirPods (NOT AirTag!)
//   0x0a - AirPlay
//   0x0c - Handoff
//   0x0d - Wi-Fi Settings
//   0x0e - Instant Hotspot
//   0x0f - Wi-Fi Join
//   0x10 - Nearby
//   0x11 - Apple Watch
//   0x12 - FindMy Network (AirTags, FindMy accessories)
//
// Last updated: 2026-01-28

// AppleProtocol defines the Bluetooth protocol identifiers for Apple devices.
var AppleProtocol = struct {
	// CompanyID is Apple's Bluetooth SIG assigned company identifier.
	// This is the key in manufacturer data: 0x004C = 76 decimal = "Apple, Inc."
	CompanyID uint16

	// FindMyType is the payload type byte for FindMy network devices (AirTags).
	// This is the primary identifier for AirTag detection.
	FindMyType byte

	// AirPodsType is the payload type for AirPods (previously misidentified as unregistered AirTag).
	// Note: 0x07 is AirPods, NOT unregistered AirTags!
	AirPodsType byte

	// NearbyType is the payload type for Nearby messages (device state broadcasting).
	NearbyType byte

	// FindMyPayloadLength is the expected length byte in FindMy advertisements.
	// 0x19 = 25 bytes (contains partial public key + status + hint)
	FindMyPayloadLength byte
}{
	CompanyID:           0x004C,
	FindMyType:          0x12,
	AirPodsType:         0x07,
	NearbyType:          0x10,
	FindMyPayloadLength: 0x19,
}

// FindMy Payload Structure (25 bytes after type+length):
//   Byte 0:     Status byte
//               - Bit 2: "Maintained" (owner connected within 15 min rotation period)
//               - Bits 6-7: Battery level (if maintained bit set)
//   Bytes 1-22: Partial public key (bytes 6-27 of EC P-224 x-coordinate)
//   Byte 23:    Public key bits (bits 6-7 of byte 0 of x-coordinate)
//   Byte 24:    Hint (byte 5 of BT_ADDR)
//
// Note: The remaining 6 bytes of the public key are encoded in the BLE MAC address.

// isAppleDevice checks if the device has Apple manufacturer data.
func (d *device) isAppleDevice() bool {
	_, ok := d.ManufacturerData()[AppleProtocol.CompanyID]
	return ok
}

// getApplePayload returns the Apple manufacturer data payload, or nil if not present.
func (d *device) getApplePayload() []byte {
	if payload, ok := d.ManufacturerData()[AppleProtocol.CompanyID]; ok {
		return payload
	}
	return nil
}

// getApplePayloadType returns the type byte from Apple manufacturer data, or 0 if not present.
func (d *device) getApplePayloadType() byte {
	payload := d.getApplePayload()
	if len(payload) < 1 {
		return 0
	}
	return payload[0]
}

// getApplePayloadLength returns the length byte from Apple manufacturer data, or 0 if not present.
func (d *device) getApplePayloadLength() byte {
	payload := d.getApplePayload()
	if len(payload) < 2 {
		return 0
	}
	return payload[1]
}

// getStatusByte returns the status byte from FindMy payload (byte index 2), or 0 if not present.
// Status byte contains:
//   - Bit 2: "Maintained" flag (owner connected within 15 min key rotation period)
//   - Bits 6-7: Battery level (only valid if maintained bit is set)
func (d *device) getStatusByte() byte {
	payload := d.getApplePayload()
	if len(payload) < 3 {
		return 0
	}
	return payload[2]
}

// isAppleAirTag checks if a device matches the Apple AirTag/FindMy payload signature.
// Looks for: type=0x12 (FindMy) and length=0x19 (25 bytes).
func (d *device) isAppleAirTag() bool {
	payloadType := d.getApplePayloadType()
	payloadLength := d.getApplePayloadLength()

	isFindMyType := payloadType == AppleProtocol.FindMyType
	isCorrectLength := payloadLength == AppleProtocol.FindMyPayloadLength

	return isFindMyType && isCorrectLength
}

// isFindMyDevice checks if a device is broadcasting on the Apple FindMy network.
// This checks for type=0x12 which indicates FindMy network participation.
func (d *device) isFindMyDevice() bool {
	return d.getApplePayloadType() == AppleProtocol.FindMyType
}

// isAirPods checks if a device is Apple AirPods.
// AirPods use type=0x07 (previously misidentified as unregistered AirTag).
func (d *device) isAirPods() bool {
	return d.getApplePayloadType() == AppleProtocol.AirPodsType
}

// isNearbyMessage checks if a device is broadcasting Apple Nearby messages.
// Nearby messages (type=0x10) indicate device state (locked, active, calling, etc.)
func (d *device) isNearbyMessage() bool {
	return d.getApplePayloadType() == AppleProtocol.NearbyType
}

// isOwnerNearby checks if an AirTag has the "maintained" bit set in status byte.
// The maintained bit (bit 2) indicates the owner's device connected within the
// current 15-minute key rotation period.
//
// Note: This is a best-effort check. The status byte interpretation may vary.
func (d *device) isOwnerNearby() bool {
	if !d.isAppleAirTag() {
		return false
	}
	// Check maintained bit (bit 2) in status byte
	statusByte := d.getStatusByte()
	maintainedBit := (statusByte >> 2) & 0x01
	return maintainedBit == 1
}

// getBatteryLevel returns the battery level from the status byte.
// Battery level is encoded in bits 6-7 (only valid if maintained bit is set).
//
// Returns: "Full", "Medium", "Low", "Critical", or "Unknown"
func (d *device) getBatteryLevel() string {
	if !d.isAppleAirTag() {
		return ""
	}

	// Battery level only valid if maintained bit is set
	if !d.isOwnerNearby() {
		return "?"
	}

	statusByte := d.getStatusByte()
	batteryBits := (statusByte >> 6) & 0x03

	switch batteryBits {
	case 0:
		return "Full"
	case 1:
		return "Med"
	case 2:
		return "Low"
	case 3:
		return "Crit"
	default:
		return "?"
	}
}
