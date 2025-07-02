package config

import "time"

const (
	// ScanRate is the rate at which to scan for devices.
	ScanRate = 50 * time.Millisecond
	// ScanBufferSize is the buffer size for the scan channel.
	ScanBufferSize = 500
	// ScanLength is the length of time to scan for devices.
	ScanLength = 200 * time.Millisecond
	// WriteTime is the rate at which to write devices to the ingest path.
	WriteTime = 10 * time.Second
	// TrimTime is the rate at which to trim the map of old devices.
	TrimTime = 1 * time.Second
	// OldestDevice is the time to keep a device in the map.
	OldestDevice = 24 * time.Hour
)
