package main

import (
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
	usltStart := strings.Index(datString, "USLT")
	if usltStart == -1 {
		panic("lyrics tag not found")
	}

	fmt.Printf("% +q \n", datString[usltStart:usltStart+50])
	fmt.Print(datString[usltStart + 10 : usltStart+3286+10])

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
