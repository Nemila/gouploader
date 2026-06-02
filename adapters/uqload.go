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
	"path/filepath"
	"time"
)

type uqload struct {
	apiKey string
}

func NewUqloadAdapter(apiKey string) *uqload {
	return &uqload{
		apiKey: apiKey,
	}
}

type uqloadUploadResponse struct {
	Msg    string `json:"msg"`
	Status int    `json:"status"`
	Files  []struct {
		Filecode string `json:"filecode"`
		Filename string `json:"filename"`
		Status   string `json:"status"`
	} `json:"files"`
}
type uqloadGetUploadServerResponse struct {
	Msg        string `json:"msg"`
	ServerTime string `json:"server_time"`
	Status     int    `json:"status"`
	Result     string `json:"result"`
}

func (a *uqload) getContentLength(filePath string) (int64, error) {
	bodyBuffer := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuffer)

	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	if err := writer.WriteField("key", a.apiKey); err != nil {
		return 0, err
	}
	_, err = writer.CreateFormFile("file", filepath.Base(file.Name()))
	if err != nil {
		return 0, err
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return 0, err
	}

	_ = writer.Close()
	contentLength := fileInfo.Size() + int64(bodyBuffer.Len())
	return contentLength, nil
}

func (a *uqload) Upload(filePath string) (string, error) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	url, err := a.getUploadServer()
	if err != nil {
		return "", err
	}

	go func() {
		defer pw.Close()
		defer writer.Close()

		if err := writer.WriteField("key", a.apiKey); err != nil {
			_ = pw.CloseWithError(err)
			return
		}

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
		Timeout: 2 * time.Minute,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, pr)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	contentLength, err := a.getContentLength(filePath)
	if err != nil {
		return "", err
	}
	req.ContentLength = contentLength

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("server returned status %d: %s", res.StatusCode, body)
	}

	var uploadRes uqloadUploadResponse
	if err := json.NewDecoder(res.Body).Decode(&uploadRes); err != nil {
		return "", err
	}

	if len(uploadRes.Files) == 0 {
		return "", fmt.Errorf("upload succeeded but server returned no file data")
	}

	fmt.Println(uploadRes)
	return uploadRes.Files[0].Filecode, nil
}

func (a *uqload) getUploadServer() (string, error) {
	url := "https://uqload.is/api/upload/server?key=" + a.apiKey

	ctx := context.Background()
	client := &http.Client{
		Timeout: 2 * time.Minute,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var getUploadServerRes uqloadGetUploadServerResponse
	if err := json.NewDecoder(res.Body).Decode(&getUploadServerRes); err != nil {
		return "", err
	}

	return getUploadServerRes.Result, nil
}
