package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

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

type Choice struct {
	Id    int
	Track *Track
}

func prompt(p *Playlist, library *Library) (*Track, error) {
	choices := make([]Choice, len(p.Tracks))

	for i, pTrack := range p.Tracks {
		track := library.Tracks[(strconv.Itoa(pTrack.TrackID))]
		choices[i] = Choice{i, &track}
	}

	idLen := len(strconv.Itoa(len(choices)))
	template := fmt.Sprintf(`{{ printf "%%%dd" .Id | cyan }}  {{ .Track.Name | bold }} by {{ .Track.Artist | italic }}`, idLen)

	templates := &promptui.SelectTemplates{
		Active:   fmt.Sprintf("%s %s", promptui.IconSelect, template),
		Inactive: fmt.Sprintf("%s %s", " ", template),
		Selected: "{{ .Track.Name | bold }} by {{ .Track.Artist | italic }}",
	}

	// A searcher function is implemented which enabled the search mode for the select. The function follows
	// the required searcher signature and finds any pepper whose name contains the searched string.
	searcher := func(input string, index int) bool {
		choice := choices[index]

		name := strings.Replace(strings.ToLower(fmt.Sprintf("%d%s%s", index, choice.Track.Name, choice.Track.Artist)), " ", "", -1)
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

	youtubeUrl, err := SearchYoutube(*choices[i].Track)

	if err != nil {
		return nil, err
	}

	fmt.Println(*youtubeUrl)

	return choices[i].Track, nil
}

func ParsePlaylistPath(playlistPath string) {
	fp := os.ExpandEnv(strings.Replace(playlistPath, "~", "$HOME", 1))
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

	if len(library.Playlists) == 0 {
		fmt.Println("No playlists, exiting...")
		return
	}

	playlist := library.Playlists[0]

	track, err := prompt(&playlist, &library)
	{
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	for track != nil || err != nil {
		fmt.Println()
		track, err = prompt(&playlist, &library)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
}

func main() {
	var playlistPath = flag.String("path", "", "Path to the Apple Music playlist file")
	var url = flag.String("url", "", "Apple Music URL")

	flag.Parse()

	switch {
	case *playlistPath != "":
		ParsePlaylistPath(*playlistPath)
	case *url != "":
		ParseURL(*url)
	}
}
