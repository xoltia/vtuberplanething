package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
)

const iconSize = 50

var (
	httpClient         = http.DefaultClient
	defaultMaxAttempts = 7
)

var (
	iconPath   = flag.String("icon-path", "icons", "path to write icons to")
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

type vtuber struct {
	Name        string `json:"name"`
	Affiliation string `json:"affiliation"`
	Image       string `json:"image"`
}

func readVtuberData() ([]vtuber, error) {
	var vtubers []vtuber

	f, err := os.Open(*vtuberFile)
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

func checkIconExists(path string) (bool, error) {
	_, err := os.Stat(path)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	default:
		return false, err
	}
}

func getIcon(ctx context.Context, imgURL string) (io.ReadCloser, error) {
	return getIconWithRetry(ctx, imgURL, 0, defaultMaxAttempts)
}

func getIconWithRetry(ctx context.Context, imgURL string, attempt, maxAttempt int) (io.ReadCloser, error) {
	if attempt == maxAttempt {
		return nil, errors.New("max attempts exceeded")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imgURL, nil)
	if err != nil {
		return nil, err
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		// Sometimes it returns 404 even though it exists?
		if res.StatusCode == http.StatusNotFound {
			return getIconWithRetry(ctx, imgURL, attempt+1, defaultMaxAttempts)
		}
		return nil, fmt.Errorf("status not ok: %s", res.Status)
	}

	return res.Body, nil
}

func writeIcon(path string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	contentType := http.DetectContentType(data)
	if contentType != "image/jpeg" {
		return errors.New("invalid content-type")
	}

	tmp, err := os.CreateTemp(*iconPath, "*")
	if err != nil {
		return err
	}
	defer tmp.Close()

	_, err = tmp.Write(data)
	if err != nil {
		return err
	}

	return os.Rename(tmp.Name(), path)
}

func getAndWriteIcon(ctx context.Context, imgURL string) (string, error) {
	filePath, err := hashURL(imgURL)
	if err != nil {
		return "", err
	}
	filePath = path.Join(*iconPath, filePath)

	exists, err := checkIconExists(filePath)
	if err != nil || exists {
		return filePath, err
	}

	icon, err := getIcon(ctx, imgURL)
	if err != nil {
		return "", err
	}
	defer icon.Close()

	if err := writeIcon(filePath, icon); err != nil {
		return "", err
	}

	return filePath, nil
}

func main() {
	flag.Parse()

	err := os.MkdirAll(*iconPath, 0700)
	if err != nil {
		panic(err)
	}

	vtubers, err := readVtuberData()
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	for _, v := range vtubers {
		fmt.Printf("downloading %s\n", v.Image)
		p, err := getAndWriteIcon(ctx, v.Image)
		fmt.Printf("output to %s\n", p)

		if err != nil {
			fmt.Printf("failed to put icon (%s): %s\n", v.Name, err)
			os.Exit(1)
		}
	}
}
