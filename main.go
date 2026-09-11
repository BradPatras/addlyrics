package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"unicode/utf16"
)

type InvalidID3Error struct{}

func (e *InvalidID3Error) Error() string {
	return "Invalid: file does not contain valid ID3v2.3 metadata"
}

type NoLyricsTagError struct{}

func (e *NoLyricsTagError) Error() string {
	return "Missing lyrics: USLT tag not found"
}

// var myErr *MyError
// if errors.As(err, &myErr) {
//     if myErr.Code == 42 {
//         // Handle the error
//     }
// }

type lyricsframe struct {
	language string
	text     string
}

func main() {
	// fmt.Println(readLyricsFromFile("test.mp3"))
	writeLyricsToFile("", "test2.mp3")
}

func readLyricsFromFile(fp string) (string, error) {
	dat, err := os.ReadFile(fp)

	if err != nil {
		return "", err
	}

	// for now assume ID3v2.3
	// https://www.thebroadcastbridge.com/content/entry/21824/standards-id3-metadata-tagging
	// https://www.the-roberts-family.net/metadata/mp3.html
	datString := string(dat)

	// find USLT tag
	cursor := strings.Index(datString, "USLT")
	if cursor == -1 {
		return "", &NoLyricsTagError{}
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
		return bytesToString(lyricsBytes), nil
	} else {
		return string(lyricsBytes), nil
	}
}

func writeLyricsToFile(lyrics string, fp string) error {
	dat, err := os.ReadFile(fp)

	if err != nil {
		return err
	}

	// verify file has ID3v2.3 block
	id3Tag := [4]byte{0x49, 0x44, 0x33, 0x03}
	if [4]byte(dat[0:5]) != id3Tag {
		return &InvalidID3Error{}
	}

	// get current ID3 size
	// the size is the last 4 bytes of the 10 byte header
	// size bytes use the special 'syncsafe' format
	id3Size := syncsafeToInt([4]byte(dat[6:10]))
	fmt.Println(id3Size)

	// check for existing lyrics tag, bail if found - I'll implement proper handling of this case later
	_, readLyricsErr := readLyricsFromFile(fp)
	if _, ok := errors.AsType[*NoLyricsTagError](readLyricsErr); !ok {
		panic("Lyrics metadata already present, overwriting not yet supported")
	}

	// create the lyrics frame

	// insert the lyrics frame at the end of the ID3 block

	// update the ID3 size value to reflect the added lyrics frame

	return nil
}

func createLyricsFrame(lyrics string, language string) ([]byte, error) {
	// build content
	contentBytes := []byte{
		0x01, // UTF16 flag
	}
	// 3 byte language code
	contentBytes = append(contentBytes, []byte(language)...)
	// empty content description (blank utf16 string)
	contentBytes = append(contentBytes, 0xFF, 0xFE, 0x00, 0x00)
	// lyrics
	contentBytes = append(contentBytes, stringToBytes(lyrics)...)
	// terminator bytes
	contentBytes = append(contentBytes, 0x00, 0x00)

	contentLengthBytes, err := intToBytes(uint32(len(contentBytes)))
	if err != nil {
		return []byte{}, err
	}

	// USLT
	headerBytes := []byte{0x55, 0x53, 0x4C, 0x54}
	// content size (4 bytes)
	headerBytes = append(headerBytes, contentLengthBytes...)
	// flags (2 bytes)
	headerBytes = append(headerBytes, 0x00, 0x00)

	return append(headerBytes, contentBytes...), nil
}

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
