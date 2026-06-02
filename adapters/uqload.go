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

type Uqload struct {
	apiKey string
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

func (u *Uqload) getContentLength(filePath string) (int64, error) {
	bodyBuffer := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuffer)

	file, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("[uqload.getContentLength] failed to open file: %w", err)
	}
	defer file.Close()

	if err := writer.WriteField("key", u.apiKey); err != nil {
		return 0, fmt.Errorf("[uqload.getContentLength] failed to write key: %w", err)
	}
	_, err = writer.CreateFormFile("file", filepath.Base(file.Name()))
	if err != nil {
		return 0, fmt.Errorf("[uqload.getContentLength] failed to create form file: %w", err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("[uqload.getContentLength] failed to get file stat: %w", err)
	}

	_ = writer.Close()
	contentLength := fileInfo.Size() + int64(bodyBuffer.Len())

	return contentLength, nil
}

func (u *Uqload) Upload(filePath string) (string, error) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("[uqload.Upload] failed to open file: %w", err)
	}
	defer file.Close()

	url, err := u.getUploadServer()
	if err != nil {
		return "", fmt.Errorf("[uqload.Upload] failed to get upload server: %w", err)
	}

	go func() {
		defer pw.Close()
		defer writer.Close()

		if err := writer.WriteField("key", u.apiKey); err != nil {
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
		return "", fmt.Errorf("[uqload.Upload] failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	contentLength, err := u.getContentLength(filePath)
	if err != nil {
		return "", fmt.Errorf("[uqload.Upload] failed to get content length: %w", err)
	}
	req.ContentLength = contentLength

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("[uqload.Upload] server returned status %d: %s", res.StatusCode, body)
	}

	var uploadRes uqloadUploadResponse
	if err := json.NewDecoder(res.Body).Decode(&uploadRes); err != nil {
		return "", err
	}

	if len(uploadRes.Files) == 0 {
		return "", fmt.Errorf("[uqload.Upload] upload succeeded but server returned no file data")
	}

	return uploadRes.Files[0].Filecode, nil
}

func (u *Uqload) getUploadServer() (string, error) {
	url := "https://uqload.is/api/upload/server?key=" + u.apiKey

	ctx := context.Background()
	client := &http.Client{
		Timeout: 2 * time.Minute,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("[uqload.getUploadServer] failed to create request: %w", err)
	}

	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("[uqload.getUploadServer] failed to execute request: %w", err)
	}
	defer res.Body.Close()

	var getUploadServerRes uqloadGetUploadServerResponse
	if err := json.NewDecoder(res.Body).Decode(&getUploadServerRes); err != nil {
		return "", fmt.Errorf("[uqload.getUploadServer] failed to decode json: %w", err)
	}

	return getUploadServerRes.Result, nil
}
