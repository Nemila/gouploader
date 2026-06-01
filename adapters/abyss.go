package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

type Abyss struct {
	apiKey string
}

func NewAbyssAdapter(apiKey string) *Abyss {
	return &Abyss{
		apiKey: apiKey,
	}
}

type uploadResponse struct {
	Status bool   `json:"status"`
	Slug   string `json:"slug"`
}

func (a *Abyss) Upload(filePath string) (string, error) {
	var body bytes.Buffer
	writter := multipart.NewWriter(&body)

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	part, err := writter.CreateFormFile("file", file.Name())
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}

	ctx := context.Background()
	client := &http.Client{
		Timeout: 2 * time.Minute,
	}

	url := "http://up.hydrax.net/" + a.apiKey
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return "", err
	}

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()

	var uploadRes uploadResponse
	if err := json.NewDecoder(res.Body).Decode(&uploadRes); err != nil {
		return "", err
	}

	fmt.Println(uploadRes)
	return uploadRes.Slug, nil
}
