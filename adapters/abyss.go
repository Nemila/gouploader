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
)

type Abyss struct {
	apiKey string
	client *http.Client
}

type abyssUploadResponse struct {
	Status bool   `json:"status"`
	Slug   string `json:"slug"`
}

func (a *Abyss) Upload(filePath string) (string, error) {
	pr, pw := io.Pipe()
	w := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer w.Close()

		file, err := os.Open(filePath)
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		defer file.Close()

		part, err := w.CreateFormFile("file", filepath.Base(file.Name()))
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		if _, err := io.Copy(part, file); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
	}()

	url := fmt.Sprintf("http://up.hydrax.net/%s", a.apiKey)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, pr)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", w.FormDataContentType())

	res, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var data abyssUploadResponse
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", err
	}

	if !data.Status || len(data.Slug) < 1 {
		return "", fmt.Errorf("abyss upload failed %s", data.Slug)
	}

	return data.Slug, nil
}
