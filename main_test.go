package main

import (
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
