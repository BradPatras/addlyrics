package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"slices"
	"strings"
	"unicode/utf16"
)

type lyricsframe struct {
	language string
	text     string
}

func main() {
	fmt.Println(readLyricsFromFile("test.mp3"))
}

func readLyricsFromFile(fp string) string {
	dat, err := os.ReadFile(fp)
	check(err)

	// for now assume ID3v2.3
	// https://www.thebroadcastbridge.com/content/entry/21824/standards-id3-metadata-tagging
	// https://www.the-roberts-family.net/metadata/mp3.html
	datString := string(dat)

	// find USLT tag
	cursor := strings.Index(datString, "USLT")
	if cursor == -1 {
		panic("lyrics tag not found")
	}

	// skip over frame id
	cursor += 4

	// get content size
	contentSizeBytes := dat[cursor : cursor+4]
	contentSize := binary.BigEndian.Uint32(contentSizeBytes)
	// advance past content size (4 bytes) and flags bytes (2 bytes)
	cursor += 6

	// encoding byte determines the size of characters (00 = 1 byte, 01 = 2 bytes)
	encodingByte := dat[cursor]
	var terminatorSize int
	if encodingByte == 0x01 {
		terminatorSize = 2
	} else {
		terminatorSize = 1
	}
	// skip over encoding byte and language bytes
	cursor += 4

	// scan over the variable length content descriptor
	var terminated bool
	var contentDescStart = cursor
	for !terminated {
		if terminatorSize == 1 && dat[cursor] == 0x00 {
			terminated = true
			cursor += 1
		} else if terminatorSize == 2 && dat[cursor] == 0x00 && dat[cursor+1] == 0x00 {
			terminated = true
			cursor += 2
		} else {
			cursor += 1
		}
	}

	contentDescSize := cursor - contentDescStart
	lyricsBytes := dat[cursor : cursor+int(contentSize-uint32(contentDescSize+3+1))]

	if terminatorSize == 2 {
		return bytesToUtf16String(lyricsBytes)
	} else {
		return string(lyricsBytes)
	}
}

func writeLyricsToFile(lyrics string, fp string) {
	dat, err := os.ReadFile(fp)
	check(err)

	// only support ID3v2.3
	id3Tag := []byte{0x49, 0x44, 0x33, 0x03}

	// verify file already has ID3 block
	if !slices.Equal(dat[0:5], id3Tag) {
		panic("Unsupported: file does not contain ID3v2.3 data")
	}

}

// this is wild, thanks https://stackoverflow.com/a/5652842/4038809
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

func check(err error) {
	if err != nil {
		panic(err)
	}
}

// decode a byte array into a utf16 string
func bytesToUtf16String(bytes []byte) string {
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
	uints := make([]uint16, len(bytes))

	for i := 0; i < len(uints); i += 2 {
		if isBigEndian {
			uints[i] = binary.BigEndian.Uint16(bytes[i : i+2])
		} else {
			uints[i] = binary.LittleEndian.Uint16(bytes[i : i+2])
		}
	}

	return string(utf16.Decode(uints))
}

/*
from the mutagen python project:
class USLT(Frame):
    """Unsynchronised lyrics/text transcription.

    Lyrics have a three letter ISO language code ('lang'), a
    description ('desc'), and a block of plain text ('text').
    """

    _framespec = [
        EncodingSpec('encoding', default=Encoding.UTF16),
        StringSpec('lang', length=3, default=u"XXX"),
        EncodedTextSpec('desc'),
        EncodedTextSpec('text'),
    ]
*/
