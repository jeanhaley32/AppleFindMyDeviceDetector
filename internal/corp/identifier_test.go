package corp

import (
	"os"
	"testing"
)

func TestIngestCorpDevices(t *testing.T) {
	// Create a dummy company_identifiers.yaml for testing
	dummyYAML := `
company_identifiers:
  - value: 0x0001
    name: 'Test Company 1'
  - value: 0x0002
    name: 'Test Company 2'
`

	testFilePath := "test_company_identifiers.yaml"
	err := os.WriteFile(testFilePath, []byte(dummyYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to create dummy YAML file: %v", err)
	}
	defer os.Remove(testFilePath) // Clean up the dummy file

	// Test successful ingestion
	cmap, err := IngestCorpDevices(testFilePath)
	if err != nil {
		t.Errorf("IngestCorpDevices() error = %v, want nil", err)
	}

	if len(cmap) != 2 {
		t.Errorf("IngestCorpDevices() got %v entries, want %v", len(cmap), 2)
	}

	if name, ok := cmap[0x0001]; !ok || name != "Test Company 1" {
		t.Errorf("IngestCorpDevices() got %v for 0x0001, want 'Test Company 1'", name)
	}

	if name, ok := cmap[0x0002]; !ok || name != "Test Company 2" {
			t.Errorf("IngestCorpDevices() got %v for 0x0002, want 'Test Company 2'", name)
	}

	// Test non-existent file
	_, err = IngestCorpDevices("non_existent_file.yaml")
	if err == nil {
		t.Errorf("IngestCorpDevices() for non-existent file expected an error, got nil")
	}
}

func TestResolve(t *testing.T) {
	cmap := make(CorpIdentMap)
	cmap[0x0001] = "Resolved Company"

	tests := []struct {
		name string
		id   uint16
		want string
	}{
		{
			name: "Known ID",
			id:   0x0001,
			want: "Resolved Company",
		},
		{
			name: "Unknown ID",
			id:   0x0003,
			want: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cmap.Resolve(tt.id); got != tt.want {
				t.Errorf("Resolve() = %v, want %v", got, tt.want)
			}
		})
	}
}
