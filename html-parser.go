package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

var htmlRequester = &http.Client{Timeout: 10 * time.Second}

var supportedServices = map[string]bool{
	"music.apple.com":  true,
	"open.spotify.com": true,
}

var titleRegexps = map[string]*regexp.Regexp{
	"music.apple.com":  regexp.MustCompile("\u200e(?P<name>.+) – Song by (?P<author>.+) – Apple\u00a0Music"),
	"open.spotify.com": regexp.MustCompile(`(?P<name>.+) - song and lyrics by (?P<author>.+) \| Spotify`),
}

func ParseURL(URL string) {
	slog.Info("Parsing", "url", URL)
	baseUrl, err := url.Parse(strings.ReplaceAll(URL, "\\", ""))

	if err != nil {
		slog.Error(err.Error())
		return
	}

	service := baseUrl.Host
	if !supportedServices[service] {
		slog.Error("Unsupported", "service", service, "supported", supportedServices)
		return
	}

	slog.Info("Detected supported", "service", service)

	resp, err := htmlRequester.Get(baseUrl.String())

	if resp != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		slog.Error(err.Error())
		return
	}

	slog.Info("Parsing HTML")
	rootNode, err := html.Parse(resp.Body)
	if err != nil {
		slog.Error(err.Error())
		return
	}

	var track Track

	slog.Info("Looking through the metadata")
	for c := rootNode.FirstChild; c != nil; c = c.NextSibling {
		if c.Data != "html" {
			continue
		}

		htmlNode := c
		for c := htmlNode.FirstChild; c != nil; c = c.NextSibling {
			if c.Data != "head" {
				continue
			}

			headNode := c
			for c := headNode.FirstChild; c != nil; c = c.NextSibling {
				if c.Data != "title" {
					continue
				}

				submatch := titleRegexps[service].FindStringSubmatch(c.FirstChild.Data)
				track = Track{Name: submatch[1], Artist: submatch[2]}
			}
		}
	}

	if (track == Track{}) {
		slog.Error("Couldn't extract song info")
		return
	}
	slog.Info("Parsed song info", "track", track)

	slog.Info("Searching Youtube")
	youtubeUrl, err := SearchYoutube(track)

	if err != nil {
		slog.Error(err.Error())
		return
	}

	slog.Info("Found")
	fmt.Println(*youtubeUrl)
}
