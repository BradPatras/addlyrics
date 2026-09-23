package main

import (
	"encoding/binary"
	"slices"
	"unicode/utf16"
)

// decode a byte array into a utf16 string
func bytesToString(bytes []byte) string {
	// default to big endian = false (little endian)
	var isBigEndian bool

	// check for BOM
	be := []byte{0xFE, 0xFF}
	le := []byte{0xFF, 0xFE}
	bom := bytes[0:2]
	if slices.Equal(bom, be) || slices.Equal(bom, le) {
		isBigEndian = slices.Equal(bom, be)
		bytes = bytes[2:]
	}

	// interpret the bytes as uint16s
	uints := make([]uint16, len(bytes)/2)

	for i := 0; i < len(bytes); i += 2 {
		if isBigEndian {
			uints[i/2] = binary.BigEndian.Uint16(bytes[i : i+2])
		} else {
			uints[i/2] = binary.LittleEndian.Uint16(bytes[i : i+2])
		}
	}

	// trim termination bytes
	for len(uints) > 0 {
		if uints[len(uints)-1] == 0 {
			uints = uints[:len(uints)-1]
		} else {
			break
		}
	}

	return string(utf16.Decode(uints))
}

// convert a string to bytes in utf16 format
func stringToBytes(s string) (bytes []byte) {
	// add little endian byte order marker
	bytes = append(bytes, 0xFF, 0xFE)
	encoded := utf16.Encode([]rune(s))
	for i := range len(encoded) {
		bytes = binary.LittleEndian.AppendUint16(bytes, encoded[i])
	}

	return bytes
}

// ellipsize middle keep head and tail
func ellipsize(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	} else {
		return string(runes[:n]) + "..." + string(runes[len(runes)-n:])
	}
}

func indnt(s string, level int) string {
	r := s
	for range level {
		r = "  " + r
	}
	return r
}