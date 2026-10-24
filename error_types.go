package main

import "fmt"

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
	return fmt.Sprint("✔ Skipping: Lyrics already present")
}

type FailedToFetchLyricsError struct {
	title      string
	status     int
	errMessage string
	errName    string
}

func (e *FailedToFetchLyricsError) Error() string {
	return fmt.Sprintf("! Failed to fetch lyrics for %s. status code: {%d} error: {%s} message: {%s}", e.title, e.status, e.errName, e.errMessage)
}

type InstrumentalError struct{}

func (e *InstrumentalError) Error() string {
	return fmt.Sprint("✔ Skipping: Instrumental")
}

type ApiRateLimitError struct {
	waitSeconds int
}

func (e *ApiRateLimitError) Error() string {
	return fmt.Sprintf("! API rate limit tripped, waiting %d seconds", e.waitSeconds)
}

type ResponseMissingPlainLyricsError struct{}

func (e *ResponseMissingPlainLyricsError) Error() string {
	return fmt.Sprintf("! Server response missing plain lyrics")
}
