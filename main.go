package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf16"

	"charm.land/lipgloss/v2"
	"github.com/bmatcuk/doublestar"

	"github.com/alexflint/go-arg"
	"github.com/tcolgate/mp3"
)

type InvalidID3Error struct{}

func (e *InvalidID3Error) Error() string {
	return "! Invalid: file does not contain valid ID3v2.3 metadata"
}

type NoLyricsTagError struct{}

func (e *NoLyricsTagError) Error() string {
	return "! Missing lyrics: USLT tag not found"
}

type NoTitleTagError struct{}

func (e *NoTitleTagError) Error() string {
	return "! Missing title: TIT2 tag not found"
}

type NoArtistTagError struct{}

func (e *NoArtistTagError) Error() string {
	return "! Missing artist: TPE1/TPE2 tag not found"
}

type TextFrameNotFoundError struct {
	frameId string
}

func (e *TextFrameNotFoundError) Error() string {
	return fmt.Sprintf("! Missing data frame: %s", e.frameId)
}

type ExistingLyricsTagError struct {
	filename string
}

func (e *ExistingLyricsTagError) Error() string {
	return fmt.Sprintf("! Lyrics already present, skipping %s", e.filename)
}

type FailedToFetchLyricsError struct {
	title string
}

func (e *FailedToFetchLyricsError) Error() string {
	return fmt.Sprintf("! Failed to fetch lyrics for %s", e.title)
}

type ApiRateLimitError struct {
	waitSeconds int
}

func (e *ApiRateLimitError) Error() string {
	return fmt.Sprintf("! API rate limit tripped, waiting %d seconds", e.waitSeconds)
}

// var myErr *MyError
// if errors.As(err, &myErr) {
//     if myErr.Code == 42 {
//         // Handle the error
//     }
// }

var args struct {
	Target      string `arg:"required,-t,--target" help:"The mp3 file or directory of mp3 files that should have lyrics added. If target is a directory, lyrics will be added to all mp3s in the directory (see also: --searchdepth)"`
	SearchDepth int    `arg:"-d, --searchdepth" help:"If target is directory, searchdepth determines how far into subdirectories to go when searching for mp3s. Useful when dealing with a directory of albums."`
}

var errorStyle = lipgloss.NewStyle().Foreground(lipgloss.BrightRed)
var accentStyle = lipgloss.NewStyle().Foreground(lipgloss.Green)
var foundCount = 0
var skippedCount = 0
var failedCount = 0
var lyricsAddedCount = 0

func main() {
	arg.MustParse(&args)

	f, err := os.Open(args.Target)
	if err != nil {
		printErr(err)
		panic("Failed to access target")
	}

	info, err := f.Stat()
	if err != nil {
		printErr(err)
		panic("Failed to access target")
	}

	var paths []string
	if info.IsDir() {
		fmt.Println("Searching target dir for mp3s...")
		paths, err = doublestar.Glob(args.Target + "/**/*.mp3")
		if err != nil {
			printErr(err)
			return
		}
		fmt.Printf("Found %d mp3s\n", foundCount)
	} else {
		paths = append(paths, args.Target)
	}

	foundCount = len(paths)
	addLyricsForPaths(paths)

	fmt.Printf("\naddlyrics summary:\n  MP3s found: %d\n  Skipped: %d\n  Failed: %d\n  Lyrics added: %d\n", foundCount, skippedCount, failedCount, lyricsAddedCount)
}

func addLyricsForPaths(paths []string) {
	for _, path := range paths {
		err := fetchAndWriteLyricsToFile(path)
		if err != nil {
			handleFetchError(err)
		} else {
			lyricsAddedCount += 1
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func handleFetchError(e error) {
	if _, ok := errors.AsType[*ExistingLyricsTagError](e); !ok {
		skippedCount += 1
		fmt.Println(indnt(e.Error(), 1))
	} else {
		failedCount += 1
		fmt.Println(errorStyle.Render(indnt(e.Error(), 1)))
	}
}

func printErr(e error) {
	if e != nil {
		fmt.Println(errorStyle.Render(indnt(e.Error(), 1)))
	}
}

func fetchAndWriteLyricsToFile(fp string) error {
	_, filename := filepath.Split(fp)
	fmt.Printf("Scanning %s\n", ellipsize(filename, 16))
	f, err := os.Open(fp)
	info, err := f.Stat()
	bytes := make([]byte, info.Size())
	_, err = f.Read(bytes)

	if err != nil {
		return err
	}

	// required
	title, err := getId3Title(bytes)
	if err != nil {
		return err
	}
	artist, err := getId3Artist(bytes)
	if err != nil {
		return err
	}

	name := strings.Join([]string{artist, title}, " - ")
	if len(name) == 0 {
		name = accentStyle.Render(filename)
	}

	// check for existing lyrics tag, bail if found - I'll implement proper handling of this case later maybe
	_, readLyricsErr := getId3Lyrics(bytes)
	if _, ok := errors.AsType[*NoLyricsTagError](readLyricsErr); !ok {
		return &ExistingLyricsTagError{name}
	} else {
		fmt.Printf("Fetching lyrics for %s\n", name)
	}

	// optional
	album, _ := getId3Album(bytes)
	duration, _ := getMp3Len(fp)

	lyrics, err := fetchLyrics(title, artist, album, duration)
	if err != nil {
		return err
	}

	return writeLyricsToFile(lyrics, fp, fp)
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
	// read TALB frame
	return getTextFrameValue(dat, "TALB")
}

func getId3Title(dat []byte) (string, error) {
	return getTextFrameValue(dat, "TIT2")
}

func readLyricsFromFile(fp string) (string, error) {
	dat, err := os.ReadFile(fp)

	if err != nil {
		return "", err
	}

	return getId3Lyrics(dat)
}

func writeLyricsToFile(lyrics string, inputfp string, outputfp string) error {
	dat, err := os.ReadFile(inputfp)

	if err != nil {
		return err
	}

	dat, err = addLyricsToID3Data(lyrics, "eng", dat)

	if err != nil {
		return err
	}

	return os.WriteFile(outputfp, dat, 0666)
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

func fetchLyrics(title string, artist string, album string, duration int64) (string, error) {
	endpoint := fmt.Sprintf("https://lrclib.net/api/get?track_name=%s&artist_name=%s&album_name=%s&duration=%d", url.QueryEscape(title), url.QueryEscape(artist), url.QueryEscape(album), duration)
	fmt.Println(endpoint)
	client := &http.Client{}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "addlyrics (github.com/BradPatras/addlyrics)")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var response any
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", err
	}

	responseMap := response.(map[string]any)
	if responseMap["plainLyrics"] != nil {
		return responseMap["plainLyrics"].(string), nil
	} else {
		return "", &FailedToFetchLyricsError{title}
	}
}

func indnt(s string, level int) string {
	r := s
	for range level {
		r = "	" + r
	}
	return r
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
