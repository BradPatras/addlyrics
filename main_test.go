package main

import (
	"slices"
	"testing"
)

func TestSyncsafeToInt(t *testing.T) {
	bytes := []byte{0x00, 0x04, 0x5B, 0x04}
	integer := uint32(77188)
	converted := syncsafeToInt([4]byte(bytes))
	if integer != converted {
		t.Errorf(`syncsafeToInt([0x00, 0x04, 0x5B, 0x04]) = %d, want "%d", error`, converted, integer)
	}
}

func TestIntToSyncsafe(t *testing.T) {
	bytes := [4]byte{0x00, 0x04, 0x5B, 0x04}
	integer := uint32(77188)
	converted := intToSyncsafe(integer)
	if bytes != converted {
		t.Errorf(`intToSyncsafe(77198) = %d, want "%d", error`, converted, bytes)
	}
}

func TestStringToBytes(t *testing.T) {
	// "Imagination" in utf16 bytes
	bytes := []byte{0xFF, 0xFE, 0x49, 0x00, 0x6D, 0x00, 0x61, 0x00, 0x67, 0x00, 0x69, 0x00, 0x6E, 0x00, 0x61, 0x00, 0x74, 0x00, 0x69, 0x00, 0x6F, 0x00, 0x6E, 0x00}
	converted := bytesToString(bytes)
	expected := "Imagination"
	if converted != expected {
		t.Errorf(`bytesToString(...) = "%s", want "%s"`, converted, expected)
	}
}

func TestBytesToString(t *testing.T) {
	converted := stringToBytes("Imagination")
	expected := []byte{0xFF, 0xFE, 0x49, 0x00, 0x6D, 0x00, 0x61, 0x00, 0x67, 0x00, 0x69, 0x00, 0x6E, 0x00, 0x61, 0x00, 0x74, 0x00, 0x69, 0x00, 0x6F, 0x00, 0x6E, 0x00}
	if !slices.Equal(converted, expected) {
		t.Errorf(`stringToBytes("Imagination") = "%s", want "%s"`, converted, expected)
	}
}

func TestCreateLyricsFrame(t *testing.T) {
	expected := []byte{
		0x55, 0x53, 0x4C, 0x54, // USLT
		0x00, 0x00, 0x00, 0x14, // Content length uint32
		0x00, 0x00, // flags
		0x01,             // utf16
		0x65, 0x6E, 0x67, // language (eng)
		0xFF, 0xFE, 0x00, 0x00, // empty content desc
		0xFF, 0xFE, 0x4C, 0x00, 0x6F, 0x00, 0x6F, 0x00, 0x6B, 0x00, 0x00, 0x00, // lyrics ("Look")
	}
	actual, err := createLyricsFrame("Look", "eng")
	if err != nil || !slices.Equal(expected, actual) {
		t.Errorf(`createLyricsFrame("Look", "eng") = "%s", want "%s"`, actual, expected)
	}
}
