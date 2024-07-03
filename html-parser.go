package main

import (
	"errors"
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

var supportedEntityTypes = map[string]bool{
	// TODO: Candidate for refactoring
	// "song" & "track" to support Apple Music & Spotify
	"track":    true,
	"song":     true,
	"playlist": true,
}

var titleRegexps = map[string]*regexp.Regexp{
	"music.apple.com":  regexp.MustCompile("\u200e(?P<name>.+) – Song by (?P<author>.+) – Apple\u00a0Music"),
	"open.spotify.com": regexp.MustCompile(`(?P<name>.+) - song and lyrics by (?P<author>.+) \| Spotify`),
}

var entityIdIndice = map[string]int{
	// https://music.apple.com/es/playlist/<name>/<user-id-or-smth>
	// https://open.spotify.com/playlist/<playlist-id>
	"music.apple.com":  2,
	"open.spotify.com": 1,
}

type ParseResult struct {
	Service    string
	RootNode   *html.Node
	EntityType string
}

func parseURL(URL string) (ParseResult, error) {
	var (
		parseResult ParseResult
		err         error
	)

	slog.Info("Parsing", "url", URL)
	baseUrl, err := url.Parse(strings.ReplaceAll(URL, "\\", ""))
	// TODO: add query string to localize results for en-US (platform specific prbly)
	// baseUrl.RawQuery = ""
	if err != nil {
		slog.Error(err.Error())
		return parseResult, errors.New("URL parse error")
	}

	service := baseUrl.Host
	parseResult.Service = service
	if !supportedServices[service] {
		slog.Error("Unsupported", "service", service, "supported", supportedServices)
		return parseResult, errors.New("unsupported service")
	}

	slog.Info("Detected supported", "service", service)

	entityType := strings.Split(baseUrl.Path, "/")[entityIdIndice[service]]
	parseResult.EntityType = entityType
	if !supportedEntityTypes[entityType] {
		slog.Error("Unsupported", "entity type", entityType, "supported", supportedEntityTypes)
		return parseResult, errors.New("unsupported entity type")
	}

	slog.Info("Detected supported", "entity type", entityType)

	slog.Info("Requesting HTML")
	resp, err := htmlRequester.Get(baseUrl.String())
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		slog.Error(err.Error())
		return parseResult, errors.New("HTML request error")
	}

	slog.Info("Parsing HTML")
	rootNode, err := html.Parse(resp.Body)
	parseResult.RootNode = rootNode
	if err != nil {
		slog.Error(err.Error())
		return parseResult, errors.New("HTML parse error")
	}

	return parseResult, nil
}

func ParseURL(URL string) {
	parseResult, err := parseURL(URL)

	if err != nil {
		slog.Error("Error parsing", "url", URL)
		return
	}

	switch parseResult.EntityType {
	case "track":
		track, err := getTrack(parseResult.RootNode, *titleRegexps[parseResult.Service])

		if err != nil {
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
	case "playlist":
		tracks, err := getTracks(parseResult.RootNode, *titleRegexps[parseResult.Service], 3)

		if err != nil {
			slog.Error("Couldn't extract tracks info")
			return
		}

		for _, track := range tracks {
			slog.Info("Searching Youtube", "track", track)
			youtubeUrl, err := SearchYoutube(track)

			if err != nil {
				slog.Error(err.Error())
				continue
			}

			slog.Info("Found")
			fmt.Println(*youtubeUrl)
		}
	}
}

func getTrack(rootNode *html.Node, titleRegexp regexp.Regexp) (Track, error) {
	track := Track{}

	slog.Info("Looking through the metadata")
	for c := rootNode.FirstChild; c != nil; c = c.NextSibling {
		if c.Data != "html" || c.Type != html.ElementNode {
			continue
		}

		htmlNode := c
		for c := htmlNode.FirstChild; c != nil; c = c.NextSibling {
			if c.Data != "head" || c.Type != html.ElementNode {
				continue
			}

			headNode := c
			for c := headNode.FirstChild; c != nil; c = c.NextSibling {
				if c.Data != "title" || c.Type != html.ElementNode {
					continue
				}

				submatch := titleRegexp.FindStringSubmatch(c.FirstChild.Data)
				track = Track{Name: submatch[1], Artist: submatch[2]}

				return track, nil
			}
			break
		}
		break
	}

	return track, errors.New("failed to determine track info")
}

func getTracks(rootNode *html.Node, titleRegexp regexp.Regexp, limit int) ([]Track, error) {
	tracks := []Track{}

	slog.Info("Looking through the metadata")
	for c := rootNode.FirstChild; c != nil; c = c.NextSibling {
		if c.Data != "html" || c.Type != html.ElementNode {
			continue
		}

		htmlNode := c
		for c := htmlNode.FirstChild; c != nil; c = c.NextSibling {
			if c.Data != "head" || c.Type != html.ElementNode {
				continue
			}

			headNode := c
			for c := headNode.FirstChild; c != nil; c = c.NextSibling {
				if c.Data != "meta" || c.Type != html.ElementNode {
					continue
				}

				attrs := map[string]string{}
				for _, attr := range c.Attr {
					attrs[attr.Key] = attr.Val
				}

				// TODO: Candidate for refactoring
				// Double condition to support Apple Music & Spotify
				if attrs["name"] != "music:song" && attrs["property"] != "music:song" {
					continue
				}

				url := attrs["content"]
				parseResult, err := parseURL(url)
				if err != nil {
					slog.Error("Error parsing", "url", url)
					continue
				}

				track, err := getTrack(parseResult.RootNode, titleRegexp)

				if err != nil {
					slog.Error("Couldn't extract song info")
					continue
				}
				slog.Info("Parsed song info", "track", track)

				tracks = append(tracks, track)

				if len(tracks) == limit {
					return tracks, nil
				}
			}
			break
		}
		break
	}

	return tracks, errors.New("failed to determine tracks info")
}
