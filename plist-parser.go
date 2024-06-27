package main

import (
	"bufio"
	"embed"
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

//go:embed cache.db
var requestCacheEmbedFile embed.FS
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

	fmt.Println(searchQuery)

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

func prompt(reader *bufio.Reader, p *Playlist, library *Library) (*Track, error) {
	fmt.Print("Enter song id (\"q\" for exit): ")

	text, err := reader.ReadString('\n')

	if err != nil {
		return nil, err
	}

	if text == "q\n" {
		return nil, nil
	}

	pTrackId, err := strconv.Atoi(text[:len(text)-1])

	if err != nil {
		return nil, err
	}

	if pTrackId < 0 || pTrackId >= len(p.Tracks) {
		return nil, errors.New("invalid id")
	}

	pTrack := p.Tracks[pTrackId]
	track := library.Tracks[(strconv.Itoa(pTrack.TrackID))]

	youtubeUrl, err := searchYoutube(track)

	if err != nil {
		return nil, err
	}

	fmt.Println(*youtubeUrl)

	return &track, nil
}

func parse(libraryPath, playlistName string) {
	fp := os.ExpandEnv(strings.Replace(libraryPath, "~", "$HOME", 1))
	f, err := os.Open(fp)

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	plistDecoder := plist.NewDecoder(f)
	fmt.Println(plistDecoder)

	var library Library

	if err := plistDecoder.Decode(&library); err != nil {
		fmt.Println(err.Error())
		return
	}

	for _, p := range library.Playlists {
		if p.Name != playlistName {
			continue
		}

		for i, pTrack := range p.Tracks {
			track := library.Tracks[(strconv.Itoa(pTrack.TrackID))]
			fmt.Printf("(%d) \"%s\" by \"%s\"\n", i, track.Name, track.Artist)
		}
		fmt.Println()

		reader := bufio.NewReader(os.Stdin)

		track, err := prompt(reader, &p, &library)
		{
			if err != nil {
				fmt.Println(err)
			}
		}

		for track != nil || err != nil {
			fmt.Println()
			track, err = prompt(reader, &p, &library)
			if err != nil {
				fmt.Println(err)
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
