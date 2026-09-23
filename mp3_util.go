package main

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/tcolgate/mp3"
)

type InvalidID3Error struct{}

func (e *InvalidID3Error) Error() string {
	return "! Invalid: file does not contain valid ID3v2.3 metadata"
}

func getId3Artist(dat []byte) (string, error) {
	// read TPE2 (TPE1 as fallback?)
	tpe2, err := getTextFrameValue(dat, "TPE2")

	if err != nil || len(tpe2) == 0 {
		return getTextFrameValue(dat, "TPE1")
	} else {
		return tpe2, nil
	}
}

func getId3Album(dat []byte) (string, error) {
	return getTextFrameValue(dat, "TALB")
}

func getId3Title(dat []byte) (string, error) {
	return getTextFrameValue(dat, "TIT2")
}

func createLyricsFrame(lyrics string, language string) ([]byte, error) {
	// build frame content
	// UTF16 flag
	contentBytes := []byte{0x01}
	// 3 byte language code
	contentBytes = append(contentBytes, []byte(language)...)
	// empty content description (blank utf16 string)
	contentBytes = append(contentBytes, 0xFF, 0xFE, 0x00, 0x00)
	// lyrics
	contentBytes = append(contentBytes, stringToBytes(lyrics)...)
	// terminator bytes
	contentBytes = append(contentBytes, 0x00, 0x00)

	// build frame header
	// USLT
	headerBytes := []byte{0x55, 0x53, 0x4C, 0x54}
	// content size (4 bytes)
	contentLengthBytes, err := intToBytes(uint32(len(contentBytes)))
	if err != nil {
		return []byte{}, err
	}
	headerBytes = append(headerBytes, contentLengthBytes...)
	// flags (2 bytes)
	headerBytes = append(headerBytes, 0x00, 0x00)

	return append(headerBytes, contentBytes...), nil
}

func addLyricsToID3Data(lyrics string, language string, data []byte) ([]byte, error) {
	// verify file has ID3v2.3 block
	id3Tag := [4]byte{0x49, 0x44, 0x33, 0x03}
	if [4]byte(data[0:5]) != id3Tag {
		return []byte{}, &InvalidID3Error{}
	}

	// get current ID3 size
	// the size is the last 4 bytes of the 10 byte header
	// size bytes use the special 'syncsafe' format
	id3Size := syncsafeToInt([4]byte(data[6:10]))

	// check for existing lyrics tag, bail if found - I'll implement proper handling of this case later
	_, readLyricsErr := getId3Lyrics(data)
	if _, ok := errors.AsType[*NoLyricsTagError](readLyricsErr); !ok {
		return []byte{}, &ExistingLyricsTagError{}
	}

	// create the lyrics frame
	lyricsFrame, err := createLyricsFrame(lyrics, language)
	if err != nil {
		return []byte{}, err
	}

	// insert the lyrics frame at the end of the ID3 block
	data = slices.Insert(data, 10, lyricsFrame...)

	// update the ID3 size value to include the added lyrics frame
	newSize := uint32(int(id3Size) + len(lyricsFrame))
	newSizeBytes := intToSyncsafe(newSize)
	data = slices.Replace(data, 6, 10, newSizeBytes[:]...)

	return data, nil
}

func getTextFrameValue(dat []byte, id string) (string, error) {
	datString := string(dat)

	// find tag
	cursor := strings.Index(datString, id)
	if cursor == -1 {
		return "", &TextFrameNotFoundError{id}
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
	// skip over encoding byte
	cursor += 1

	contentBytes := dat[cursor : cursor+int(contentSize-1)]

	if terminatorSize == 2 {
		return bytesToString(contentBytes), nil
	} else {
		return string(contentBytes), nil
	}
}

func getId3Lyrics(dat []byte) (string, error) {
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

// thanks https://stackoverflow.com/a/60294163/4038809
func getMp3Len(fp string) (int64, error) {
	var t int64
	r, err := os.Open(fp)
	if err != nil {
		return 0, err
	}

	d := mp3.NewDecoder(r)
	var f mp3.Frame
	skipped := 0

	for {
		if err := d.Decode(&f, &skipped); err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}

		t = t + f.Duration().Milliseconds()
	}

	return t / 1000, nil
}
