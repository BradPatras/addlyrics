package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
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
	langBytes := dat[cursor : cursor+4]
	cursor += 4
	mystery := dat[cursor : cursor+6]
	cursor += 6
	lyricsBytes := dat[cursor : cursor+int(contentSize-10)]

	fmt.Println(string(frameTagBytes))
	fmt.Println(contentSize)
	fmt.Println(flagsBytes)
	fmt.Println(string(langBytes))
	fmt.Println(mystery)
	fmt.Println(string(lyricsBytes))
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func printLyricsFrame(l lyricsframe) {
	fmt.Printf("language: %s\n, lyrics text: %s", l.language, l.text)
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
