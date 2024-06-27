package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"howett.net/plist"
)

type Track struct {
	Name   string
	Artist string
}

type PlaylistItem struct {
	TrackID int `plist:"Track ID"`
}

type Playlist struct {
	Name   string
	Tracks []PlaylistItem `plist:"Playlist Items"`
}

type Library struct {
	Tracks    map[string]Track
	Playlists []Playlist
}

type YoutubeSearchResp struct {
	Items []YoutubeSearchRespItem `json:"items"`
}

type YoutubeSearchRespItem struct {
	Id YoutubeSearchRespItemId `json:"id"`
}

type YoutubeSearchRespItemId struct {
	Video string `json:"videoId"`
}

const YOUTUBE_API = "https://www.googleapis.com/youtube/v3/search"

var searchClient = &http.Client{Timeout: 10 * time.Second}

var requestCache = make(map[string]string)

func searchYoutube(track Track) (*string, error) {
	searchStr := fmt.Sprintf("\"%s\" by \"%s\"\n", track.Name, track.Artist)
	searchUrl, err := getUrl(searchStr)

	if err != nil {
		return nil, err
	}

	if videoUrl, ok := requestCache[searchStr]; ok {
		videoUrl = fmt.Sprintf("%s [cached]", videoUrl)
		return &videoUrl, nil
	}

	r, err := searchClient.Get(searchUrl.String())
	defer r.Body.Close()

	if err != nil {
		return nil, err
	}

	var youtubeSearchResp YoutubeSearchResp
	err = json.NewDecoder(r.Body).Decode(&youtubeSearchResp)

	if err != nil {
		return nil, err
	}

	if len(youtubeSearchResp.Items) == 0 {
		return nil, errors.New("track not found")
	}

	videoUrl := fmt.Sprintf("https://youtube.com/watch?v=%s", youtubeSearchResp.Items[0].Id.Video)
	requestCache[searchStr] = videoUrl
	return &videoUrl, nil
}

func getUrl(searchQuery string) (*url.URL, error) {
	baseUrl, err := url.Parse(YOUTUBE_API)

	if err != nil {
		return nil, err
	}

	apiKey, ok := os.LookupEnv("YOUTUBE_API_KEY")
	if !ok {
		return nil, errors.New("Can't retrieve Youtube URL: 'YOUTUBE_API_KEY' is not set")
	}

	q := baseUrl.Query()
	q.Set("key", apiKey)
	q.Set("type", "video")
	q.Set("q", searchQuery)
	baseUrl.RawQuery = q.Encode()

	return baseUrl, err
}

type Choice struct {
	Id    int
	Track *Track
}

func prompt(reader *bufio.Reader, p *Playlist, library *Library) (*Track, error) {
	choices := make([]Choice, len(p.Tracks))

	for i, pTrack := range p.Tracks {
		track := library.Tracks[(strconv.Itoa(pTrack.TrackID))]
		choices[i] = Choice{i, &track}
	}

	const Template = "({{ .Id | cyan }}) {{ .Track.Name | bold }} by {{ .Track.Artist | italic }}"

	templates := &promptui.SelectTemplates{
		Active:   fmt.Sprintf("%s %s", promptui.IconSelect, Template),
		Inactive: fmt.Sprintf("%s %s", " ", Template),
		Selected: "{{ .Track.Name | bold }} by {{ .Track.Artist | italic }}",
	}

	// A searcher function is implemented which enabled the search mode for the select. The function follows
	// the required searcher signature and finds any pepper whose name contains the searched string.
	searcher := func(input string, index int) bool {
		choice := choices[index]
		name := strings.Replace(strings.ToLower(choice.Track.Name), " ", "", -1)
		input = strings.Replace(strings.ToLower(input), " ", "", -1)

		return strings.Contains(name, input)
	}

	prompt := promptui.Select{
		Label:             "Select a song",
		Items:             choices,
		Templates:         templates,
		Size:              7,
		Searcher:          searcher,
		StartInSearchMode: true,
	}

	i, _, err := prompt.Run()

	if err != nil {
		if errors.Is(err, promptui.ErrInterrupt) {
			fmt.Println(err)
			return nil, nil
		}
		return nil, errors.New(fmt.Sprintf("Prompt failed %v\n", err))
	}

	youtubeUrl, err := searchYoutube(*choices[i].Track)

	if err != nil {
		return nil, err
	}

	fmt.Println(*youtubeUrl)

	return choices[i].Track, nil
}

func parse(libraryPath, playlistName string) {
	fp := os.ExpandEnv(strings.Replace(libraryPath, "~", "$HOME", 1))
	f, err := os.Open(fp)

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	plistDecoder := plist.NewDecoder(f)

	var library Library

	if err := plistDecoder.Decode(&library); err != nil {
		fmt.Println(err.Error())
		return
	}

	for _, p := range library.Playlists {
		if p.Name != playlistName {
			continue
		}

		reader := bufio.NewReader(os.Stdin)

		track, err := prompt(reader, &p, &library)
		{
			if err != nil {
				fmt.Println(err)
				return
			}
		}

		for track != nil || err != nil {
			fmt.Println()
			track, err = prompt(reader, &p, &library)
			if err != nil {
				fmt.Println(err)
				return
			}
		}

		return
	}

	fmt.Println("Playlist", playlistName, "was not found")
}

func main() {
	var libraryPath = flag.String("path", "", "Path to the Apple Music library file")
	var playlistName = "Replay 2024"
	flag.Parse()
	parse(*libraryPath, playlistName)
}
