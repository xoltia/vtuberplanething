package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"net/url"
	"os"
	"path"
	"strconv"

	xdraw "golang.org/x/image/draw"
)

const iconSize = 50

var (
	spritePath = flag.String("sprite-path", "sprites", "path to write sprites to")
	iconPath   = flag.String("icon-path", "icons", "path to read icons from")
	vtuberFile = flag.String("vtubers", "vtubers.json", "path to vtuber data file")
)

func hashURL(urlString string) (string, error) {
	u, err := url.Parse(urlString)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(urlString))
	name := hex.EncodeToString(hash[:])
	if ext := path.Ext(u.Path); ext != "" {
		name += ext
	}
	return name, err
}

func readIconData(fileName string) (image.Image, error) {
	iconFile, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer iconFile.Close()
	return jpeg.Decode(iconFile)
}

func createSprite(files []string) (image.Image, error) {
	gridSize := int(math.Ceil(math.Sqrt(float64(len(files)))))
	pxSize := gridSize * iconSize
	sprite := image.NewRGBA(image.Rect(0, 0, pxSize, pxSize))
	for i, name := range files {
		data, err := readIconData(name)
		if err != nil {
			return nil, err
		}
		x := (i % gridSize) * iconSize
		y := (i / gridSize) * iconSize
		dr := image.Rect(x, y, x+iconSize, y+iconSize)
		xdraw.CatmullRom.Scale(sprite, dr, data, data.Bounds(), xdraw.Src, nil)
	}
	return sprite, nil
}

type vtuber struct {
	Name        string `json:"name"`
	Affiliation string `json:"affiliation"`
	Image       string `json:"image"`
	Gender      string `json:"gender"`
	Language    string `json:"language"`
}

func readVtuberData(filePath string) ([]vtuber, error) {
	var vtubers []vtuber

	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&vtubers)
	if err != nil {
		return nil, err
	}

	return vtubers, nil
}

func makeIconGroups(vtubers []vtuber) [][]vtuber {
	affiliationGroups := make(map[string][]vtuber, 0)
	for _, v := range vtubers {
		affiliationGroups[v.Affiliation] = append(affiliationGroups[v.Affiliation], v)
	}

	groups := make([][]vtuber, 0, len(affiliationGroups))
	for _, g := range affiliationGroups {
		groups = append(groups, g)
	}

	// TODO: merge small groups, break big groups
	return groups
}

func createSpriteFromVtubers(groupID string, vtubers []vtuber) error {
	iconPaths := make([]string, 0)

	for _, v := range vtubers {
		iconHash, err := hashURL(v.Image)
		if err != nil {
			return err
		}
		path := path.Join(*iconPath, iconHash)
		iconPaths = append(iconPaths, path)
	}

	sprite, err := createSprite(iconPaths)
	if err != nil {
		return err
	}

	outPath := path.Join(*spritePath, fmt.Sprintf("%s.png", groupID))
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, sprite); err != nil {
		return err
	}

	return nil
}

func main() {
	flag.Parse()

	err := os.MkdirAll(*spritePath, 0700)
	if err != nil {
		panic(err)
	}

	outFile, err := os.Create("vtubers-small.jsonl")
	if err != nil {
		panic(err)
	}
	defer outFile.Close()
	encoder := json.NewEncoder(outFile)

	vtubers, err := readVtuberData(*vtuberFile)
	if err != nil {
		panic(err)
	}

	groups := makeIconGroups(vtubers)
	for i, g := range groups {
		err := createSpriteFromVtubers(strconv.Itoa(i), g)
		if err != nil {
			panic(err)
		}

		for j, v := range g {
			v.Image = fmt.Sprintf("%d:%d", i, j)
			err = encoder.Encode(v)
			if err != nil {
				panic(err)
			}
		}
	}
}
