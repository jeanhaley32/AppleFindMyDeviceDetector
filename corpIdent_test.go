package main

import "testing"

func TestExtractCompanyID_PrefersApple(t *testing.T) {
	data := ManufacturerData{0x0001: {1}, appleCompanyID: {2}, 0x9999: {3}}
	for i := 0; i < 20; i++ {
		if got := extractCompanyID(data); got != appleCompanyID {
			t.Fatalf("extractCompanyID() = %#x, want %#x (Apple)", got, appleCompanyID)
		}
	}
}

func TestExtractCompanyID_DeterministicWithoutApple(t *testing.T) {
	data := ManufacturerData{0x0050: {1}, 0x0010: {2}, 0x0030: {3}}
	for i := 0; i < 20; i++ {
		if got := extractCompanyID(data); got != 0x0010 {
			t.Fatalf("extractCompanyID() = %#x, want %#x (lowest)", got, 0x0010)
		}
	}
}

func TestExtractCompanyID_Empty(t *testing.T) {
	if got := extractCompanyID(ManufacturerData{}); got != 0 {
		t.Fatalf("extractCompanyID() on empty map = %#x, want 0", got)
	}
}
