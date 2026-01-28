package main

import (
	"encoding/binary"
	"fmt"
	"time"

	"tinygo.org/x/bluetooth"
)

// Non-owner GATT service and characteristic UUIDs for Find My accessories
// From IETF DULT (Detecting Unwanted Location Trackers) specification
var (
	// Non-owner service UUID: 15190001-12F4-C226-88ED-2AC5579F2A85
	NonOwnerServiceUUID = bluetooth.NewUUID([16]byte{
		0x15, 0x19, 0x00, 0x01, 0x12, 0xF4, 0xC2, 0x26,
		0x88, 0xED, 0x2A, 0xC5, 0x57, 0x9F, 0x2A, 0x85,
	})

	// Non-owner characteristic UUID: 8E0C0001-1D68-FB92-BF61-48377421680E
	NonOwnerCharacteristicUUID = bluetooth.NewUUID([16]byte{
		0x8E, 0x0C, 0x00, 0x01, 0x1D, 0x68, 0xFB, 0x92,
		0xBF, 0x61, 0x48, 0x37, 0x74, 0x21, 0x68, 0x0E,
	})
)

// Sound control opcodes (little endian)
const (
	OpcodeSoundStart     uint16 = 0x0300
	OpcodeSoundStop      uint16 = 0x0301
	OpcodeCommandResponse uint16 = 0x0302
	OpcodeSoundCompleted uint16 = 0x0303
)

// SoundTrigger handles connecting to and triggering sound on Find My devices
type SoundTrigger struct {
	adapter *bluetooth.Adapter
}

// NewSoundTrigger creates a new sound trigger instance
func NewSoundTrigger(adapter *bluetooth.Adapter) *SoundTrigger {
	return &SoundTrigger{adapter: adapter}
}

// TriggerSound connects to the device and sends the play sound command
func (st *SoundTrigger) TriggerSound(address bluetooth.Address) error {
	return st.sendSoundCommand(address, OpcodeSoundStart, false)
}

// TriggerSoundQuiet is like TriggerSound but with minimal logging (for batch operations)
func (st *SoundTrigger) TriggerSoundQuiet(address bluetooth.Address) error {
	return st.sendSoundCommand(address, OpcodeSoundStart, true)
}

// sendSoundCommand handles the common connection and command logic
func (st *SoundTrigger) sendSoundCommand(address bluetooth.Address, opcode uint16, quiet bool) error {
	if !quiet {
		fmt.Printf("Connecting to %s...\n", address.String())
	}

	// Connect to the device with shorter timeout for batch operations
	device, err := st.adapter.Connect(address, bluetooth.ConnectionParams{
		ConnectionTimeout: bluetooth.NewDuration(5 * time.Second),
	})
	if err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}
	defer device.Disconnect()

	if !quiet {
		fmt.Println("Connected. Discovering services...")
	}

	// Discover the non-owner service
	services, err := device.DiscoverServices([]bluetooth.UUID{NonOwnerServiceUUID})
	if err != nil {
		return fmt.Errorf("service discovery failed: %w", err)
	}

	if len(services) == 0 {
		return fmt.Errorf("non-owner service not found")
	}

	// Discover the non-owner characteristic
	chars, err := services[0].DiscoverCharacteristics([]bluetooth.UUID{NonOwnerCharacteristicUUID})
	if err != nil {
		return fmt.Errorf("characteristic discovery failed: %w", err)
	}

	if len(chars) == 0 {
		return fmt.Errorf("non-owner characteristic not found")
	}

	if !quiet {
		fmt.Println("Sending command...")
	}

	// Build the command (opcode in little endian)
	cmd := make([]byte, 2)
	binary.LittleEndian.PutUint16(cmd, opcode)

	// Write the command
	_, err = chars[0].WriteWithoutResponse(cmd)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	// Brief pause to ensure command is processed before disconnect
	time.Sleep(500 * time.Millisecond)

	return nil
}

// StopSound connects to the device and sends the stop sound command
func (st *SoundTrigger) StopSound(address bluetooth.Address) error {
	return st.sendSoundCommand(address, OpcodeSoundStop, false)
}

// StopSoundQuiet is like StopSound but with minimal logging (for batch operations)
func (st *SoundTrigger) StopSoundQuiet(address bluetooth.Address) error {
	return st.sendSoundCommand(address, OpcodeSoundStop, true)
}
