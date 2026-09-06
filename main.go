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
	println("hello world")

	dat, err := os.ReadFile("test.mp3")
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

	frameTagBytes := dat[cursor : cursor+4]
	cursor += 4
	contentSizeBytes := dat[cursor : cursor+4]
	contentSize := binary.BigEndian.Uint32(contentSizeBytes)
	cursor += 4
	flagsBytes := dat[cursor : cursor+2]
	cursor += 2

	// encoding byte determines the size of characters (00 = 1 byte, 01 = 2 bytes)
	encodingByte := dat[cursor]
	var terminatorSize int
	if encodingByte == 0x01 {
		terminatorSize = 2
	} else {
		terminatorSize = 1
	}
	cursor += 1

	langBytes := dat[cursor : cursor+3]
	cursor += 3

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
	contentDesc := dat[contentDescStart:cursor]
	lyricsBytes := dat[cursor : cursor+int(contentSize-uint32(contentDescSize+3+1))]
	fmt.Println(string(frameTagBytes))
	fmt.Println(contentSize)
	fmt.Println(flagsBytes)
	fmt.Println(encodingByte)
	fmt.Println(string(langBytes))
	fmt.Println(decodeUtf16String(contentDesc))
	fmt.Println(decodeUtf16String(lyricsBytes))
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func printLyricsFrame(l lyricsframe) {
	fmt.Printf("language: %s\n, lyrics text: %s", l.language, l.text)
}

// decode a byte array into a utf16 string
func decodeUtf16String(bytes []byte) string {
	// default to little endian
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
