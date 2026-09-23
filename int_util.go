package main

import (
	"bytes"
	"encoding/binary"
)

// Convert uint32 to bytes
func intToBytes(i uint32) ([]byte, error) {
	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.BigEndian, i)
	if err != nil {
		return []byte{}, err
	}

	return buff.Bytes(), nil
}

// this is wild, thanks https://stackoverflow.com/a/5652842/4038809
// Basically, mp3 uses a long string of '1's so signify the start of the
// music data. The syncsafe int format ensures that a large number represented
// in binary won't be interpreted as the 'data start' signal.
func syncsafeToInt(bytes [4]byte) uint32 {
	byte0 := uint32(bytes[0])
	byte1 := uint32(bytes[1])
	byte2 := uint32(bytes[2])
	byte3 := uint32(bytes[3])

	return byte0<<21 | byte1<<14 | byte2<<7 | byte3
}

func intToSyncsafe(i uint32) [4]byte {
	// I'm going to immediately forget how this works, so:
	// move the bits that'll represent the desired byte down to the least-significant 7 bits
	// then the '& 0x7f' bitmask takes only those 7 bytes cause 0x7f == 01111111
	return [4]byte{
		byte((i >> 21) & 0x7F),
		byte((i >> 14) & 0x7F),
		byte((i >> 7) & 0x7F),
		byte(i & 0x7F),
	}
}
