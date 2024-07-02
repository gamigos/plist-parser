package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

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

var requestCache = make(map[string]string)

func SearchYoutube(track Track) (*string, error) {
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
