package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"test/modules/doujin"
	"time"
)

// DoujinData is intentionally exported as a type alias to the doujin.DoujinData
// This provides a single source of truth for the data structure while keeping
// internal APIs simple. Any changes to doujin.DoujinData will propagate here.
type DoujinData = doujin.DoujinData

// getExtension returns the file extension for a NHentai image type token.
// Mirrors logic used in handler.go: "j" -> jpg, "p" -> png, "g" -> gif.
func getExtension(t string) string {
	switch t {
	case "j":
		return "jpg"
	case "p":
		return "png"
	case "g":
		return "gif"
	default:
		return "jpg"
	}
}

// FetchAndDownloadDoujin fetches metadata for a given nhentai code and downloads
// all related images to destRoot/code directory.
// Returns the parsed DoujinData (alias type), the local directory path (codeDir), and an error if any.
func FetchAndDownloadDoujin(code string, destRoot string) (*DoujinData, string, error) {
	// 1) Fetch metadata
	url := fmt.Sprintf("https://nhentai.net/api/gallery/%s", code)
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("request creation failed: %w", err)
	}
	req.Header.Set("User-Agent", "NHentaiFetcher/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", fmt.Errorf("code %s not found", code)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}

	var data DoujinData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, "", fmt.Errorf("invalid JSON response: %w", err)
	}

	// 2) Prepare local filesystem
	codeDir := filepath.Join(destRoot, code)
	if err := os.MkdirAll(codeDir, 0o755); err != nil {
		return nil, "", fmt.Errorf("failed to create destination directory: %w", err)
	}

	// 3) Download cover image
	coverExt := "jpg"
	if len(data.Images.Pages) > 0 {
		coverExt = getExtension(data.Images.Pages[0].Type)
	}
	coverURL := fmt.Sprintf("https://t.nhentai.net/galleries/%s/cover.%s", data.MediaID, coverExt)
	coverPath := filepath.Join(codeDir, fmt.Sprintf("cover.%s", coverExt))
	fmt.Printf("Downloading cover %s...\n", coverPath)
	if err := downloadFile(coverURL, coverPath); err != nil {
		return &data, codeDir, fmt.Errorf("failed to download cover: %w", err)
	}

	// 4) Download all pages
	for i, p := range data.Images.Pages {
		ext := getExtension(p.Type)
		pageURL := fmt.Sprintf("https://i.nhentai.net/galleries/%s/%d.%s", data.MediaID, i+1, ext)
		pagePath := filepath.Join(codeDir, fmt.Sprintf("%03d.%s", i+1, ext))
		fmt.Printf("Downloading page %s...\n", pagePath)
		if err := downloadFile(pageURL, pagePath); err != nil {
			return &data, codeDir, fmt.Errorf("failed to download page %d: %w", i+1, err)
		}
	}

	return &data, codeDir, nil
}

// downloadFile streams the content from url to localPath with simple retries.
func downloadFile(url, localPath string) error {
	const maxRetries = 3
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err := http.Get(url)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
			continue
		}
		// Create file
		f, err := os.Create(localPath)
		if err != nil {
			resp.Body.Close()
			return err
		}
		_, err = io.Copy(f, resp.Body)
		resp.Body.Close()
		f.Close()
		if err != nil {
			lastErr = err
			// remove partial
			os.Remove(localPath)
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
			continue
		}
		return nil
	}
	return lastErr
}
