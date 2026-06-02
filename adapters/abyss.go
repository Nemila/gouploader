package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Abyss struct {
	apiKey string
}

type abyssUploadResponse struct {
	Status bool   `json:"status"`
	Slug   string `json:"slug"`
}

func (a *Abyss) Upload(filePath string) (string, error) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("[abyss.Upload] failed to open file: %w", err)
	}
	defer file.Close()

	go func() {
		defer pw.Close()
		defer writer.Close()

		part, err := writer.CreateFormFile("file", filepath.Base(file.Name()))
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		_, err = io.Copy(part, file)
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
	}()

	ctx := context.Background()
	client := &http.Client{
		Timeout: 30 * time.Minute,
	}

	url := "http://up.hydrax.net/" + a.apiKey

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, pr)
	if err != nil {
		return "", fmt.Errorf("[abyss.Upload] failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("[abyss.Upload] failed to execute request: %w", err)
	}
	defer res.Body.Close()

	var uploadRes abyssUploadResponse
	if err := json.NewDecoder(res.Body).Decode(&uploadRes); err != nil {
		return "", err
	}

	return uploadRes.Slug, nil
}
