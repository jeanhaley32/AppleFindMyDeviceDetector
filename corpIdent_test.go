package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIngestCorpDevices(t *testing.T) {
	t.Run("File not found", func(t *testing.T) {
		_, err := ingestCorpDevices("non_existent_file.yaml")
		if err == nil {
			t.Errorf("Expected an error for non-existent file, got nil")
		}
	})

	t.Run("Malformed YAML content", func(t *testing.T) {
		tmpDir := t.TempDir()
		malformedFile := filepath.Join(tmpDir, "malformed.yaml")
		err := os.WriteFile(malformedFile, []byte("company_identifiers: [{value: 1, name: \"Test Co\""), 0600)
		if err != nil {
			t.Fatalf("Failed to create malformed YAML file: %v", err)
		}

		_, err = ingestCorpDevices(malformedFile)
		if err == nil {
			t.Errorf("Expected an error for malformed YAML, got nil")
		}
	})

	t.Run("Successful loading", func(t *testing.T) {
		tmpDir := t.TempDir()
		validFile := filepath.Join(tmpDir, "valid.yaml")
		yamlContent := `
company_identifiers:
  - value: 76
    name: "Apple, Inc."
  - value: 1118
    name: "Google LLC"
`
		err := os.WriteFile(validFile, []byte(yamlContent), 0600)
		if err != nil {
			t.Fatalf("Failed to create valid YAML file: %v", err)
		}

		cmap, err := ingestCorpDevices(validFile)
		if err != nil {
			t.Errorf("Expected no error for valid YAML, got %v", err)
		}
		if cmap == nil {
			t.Fatal("Expected CorpIdentMap to be non-nil, got nil")
		}

		expectedMap := CorpIdentMap{
			76:   "Apple, Inc.",
			1118: "Google LLC",
		}
		if !reflect.DeepEqual(cmap, expectedMap) {
			t.Errorf("Loaded map does not match expected map.\nGot: %v\nExpected: %v", cmap, expectedMap)
		}
	})

	t.Run("Empty YAML file", func(t *testing.T) {
		tmpDir := t.TempDir()
		emptyFile := filepath.Join(tmpDir, "empty.yaml")
		err := os.WriteFile(emptyFile, []byte(""), 0600)
		if err != nil {
			t.Fatalf("Failed to create empty YAML file: %v", err)
		}

		// Depending on yaml.v2 behavior, this might error or return an empty map.
		// The current code structure with d.Decode(&c) will likely error if the structure isn't there.
		_, err = ingestCorpDevices(emptyFile)
		if err == nil {
			t.Errorf("Expected an error for empty YAML file (when struct is expected), got nil")
		}
	})

	t.Run("YAML with no company_identifiers key", func(t *testing.T) {
		tmpDir := t.TempDir()
		noKeyFile := filepath.Join(tmpDir, "nokey.yaml")
		yamlContent := `
some_other_key:
  - value: 1
    name: "Test"
`
		err := os.WriteFile(noKeyFile, []byte(yamlContent), 0600)
		if err != nil {
			t.Fatalf("Failed to create YAML file: %v", err)
		}

		cmap, err := ingestCorpDevices(noKeyFile)
		if err != nil {
			// This might not be an error from Decode, but the map will be empty.
			t.Errorf("Expected no error, got %v", err)
		}
		if cmap == nil {
			t.Fatal("Expected CorpIdentMap to be non-nil (but empty), got nil")
		}
		if len(cmap) != 0 {
			t.Errorf("Expected empty map, got %v elements", len(cmap))
		}
	})
}

// Mock for resolveCompanyIdent if needed, but it's simple enough.
// For this subtask, testing ingestCorpDevices is the priority.
// Testing resolveCompanyIdent would be:
func TestResolveCompanyIdent(t *testing.T) {
	cmap := CorpIdentMap{
		76: "Apple, Inc.",
	}
	tests := []struct {
		name     string
		id       uint16
		expected string
	}{
		{"Known ID", 76, "Apple, Inc."},
		{"Unknown ID", 1, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveCompanyIdent(&cmap, tt.id)
			if result != tt.expected {
				t.Errorf("resolveCompanyIdent(%d) = %s; want %s", tt.id, result, tt.expected)
			}
		})
	}
}

// Testing getCompanyIdent
func TestGetCompanyIdent(t *testing.T) {
	tests := []struct {
		name     string
		data     manData
		expected uint16
	}{
		{"Empty ManData", manData{}, 0},
		{"ManData with one entry", manData{76: []byte{0x01, 0x02}}, 76},
		{"ManData with multiple entries (first is returned)", manData{76: []byte{0x01}, 100: []byte{0x03}}, 76}, // Behavior depends on map iteration order
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: For the multi-entry case, map iteration order is not guaranteed.
			// This test might be flaky if the first element iterated changes.
			// However, getCompanyIdent as written just takes the first one it finds.
			result := getCompanyIdent(tt.data)
			// For the specific case of multiple entries, we can only reliably test if one of the keys is returned.
			// However, the current code *always* picks one, so the test as written for "ManData with multiple entries"
			// is okay for the current implementation, but it's good to be aware of this.
			if tt.name == "ManData with multiple entries (first is returned)" {
				if _, ok := tt.data[result]; !ok {
					t.Errorf("getCompanyIdent() returned %d which is not a key in the input map %v", result, tt.data)
				}
			} else {
				if result != tt.expected {
					t.Errorf("getCompanyIdent() = %d; want %d", result, tt.expected)
				}
			}
		})
	}
}
