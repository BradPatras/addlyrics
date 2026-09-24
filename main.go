package main

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/bmatcuk/doublestar"

	"github.com/alexflint/go-arg"
)

var args struct {
	Target string `arg:"required,-t,--target" help:"The mp3 file or directory of mp3 files that should have lyrics added. If target is a directory, lyrics will be added to all mp3s in the directory (see also: --searchdepth)"`
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
			fmt.Println(ind("Lyrics added!", 1))
			// Go easy on the free lyrics api
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func handleFetchError(e error) {
	if _, ok := errors.AsType[*ExistingLyricsTagError](e); ok {
		skippedCount += 1
		fmt.Println(ind(e.Error(), 1))
	} else {
		failedCount += 1
		fmt.Println(errorStyle.Render(ind(e.Error(), 1)))
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
		fmt.Printf(ind("Fetching lyrics for %s\n", 1), name)
	}

	// optional
	album, _ := getId3Album(bytes)
	duration, _ := getMp3Len(fp)

	var lyrics string
	shouldRetry := true
	for shouldRetry {
		shouldRetry = false
		lyrics, err = fetchLyrics(title, artist, album, duration)
		if err != nil {
			if rateLimitErr, ok := errors.AsType[*ApiRateLimitError](err); ok {
				fmt.Println(ind(rateLimitErr.Error(), 1))
				time.Sleep(time.Duration(rateLimitErr.waitSeconds) * time.Second)
			} else {
				return err
			}
		}
	}

	return writeLyricsToFile(lyrics, fp, fp)
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

func fetchLyrics(title string, artist string, album string, duration int64) (string, error) {
	endpoint := fmt.Sprintf("https://lrclib.net/api/get?track_name=%s&artist_name=%s&album_name=%s&duration=%d", url.QueryEscape(title), url.QueryEscape(artist), url.QueryEscape(album), duration)
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

	// Check for rate limiting
	if resp.StatusCode == 429 {
		for k, v := range resp.Header {
			if k == "Retry-After" && len(v) > 0 {
				waitTime, err := strconv.Atoi(v[0])
				if err == nil {
					return "", &ApiRateLimitError{waitTime}
				} else {
					return "", &FailedToFetchLyricsError{title}
				}
			}
		}
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

func printErr(e error) {
	if e != nil {
		fmt.Println(errorStyle.Render(ind(e.Error(), 1)))
	}
}
