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
	"regexp"
	"time"
)

type Vidhide struct {
	apiKey string
	sessId string
}

type vidhideGetUploadServerResponse struct {
	Msg        string `json:"msg"`
	ServerTime string `json:"server_time"`
	Status     int    `json:"status"`
	Result     string `json:"result"`
}

func (vh *Vidhide) getContentLength(filePath string) (int64, error) {
	bodyBuffer := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuffer)

	file, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("[vidhide.getContentLength] failed to open file: %w", err)
	}

	defer file.Close()

	if err := writer.WriteField("sess_id", vh.sessId); err != nil {
		return 0, fmt.Errorf("[vidhide.getContentLength] failed to write sess_id: %w", err)
	}

	_, err = writer.CreateFormFile("file", filepath.Base(file.Name()))
	if err != nil {
		return 0, fmt.Errorf("[vidhide.getContentLength] failed to create form file: %w", err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("[vidhide.getContentLength] failed to get file stat: %w", err)
	}

	_ = writer.Close()
	contentLength := fileInfo.Size() + int64(bodyBuffer.Len())

	return contentLength, nil
}

func (vh *Vidhide) Upload(filePath string) (string, error) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("[vidhide.Upload] failed to open file: %w", err)
	}
	defer file.Close()

	url, err := vh.getUploadServer()
	if err != nil {
		return "", fmt.Errorf("[vidhide.Upload] failed to get upload server: %w", err)
	}

	go func() {
		defer pw.Close()
		defer writer.Close()

		if err := writer.WriteField("sess_id", vh.sessId); err != nil {
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
		Timeout: 30 * time.Minute,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, pr)
	if err != nil {
		return "", fmt.Errorf("[vidhide.Upload] failed create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	contentLength, err := vh.getContentLength(filePath)
	if err != nil {
		return "", fmt.Errorf("[vidhide.Upload] failed to get content length: %w", err)
	}
	req.ContentLength = contentLength

	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("[vidhide.Upload] failed to execute request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("[vidhide.Upload] server returned status %d: %s", res.StatusCode, body)
	}

	uploadRes, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("[vidhide.Upload] failed to read response: %w", err)
	}

	re := regexp.MustCompile(`name="fn">([^<]+)`)
	matches := re.FindStringSubmatch(string(uploadRes))
	if len(matches) < 1 {
		return "", fmt.Errorf("[vidhide.Upload] failed to extract slug")
	}

	return matches[1], nil
}

func (vh *Vidhide) getUploadServer() (string, error) {
	url := "https://earnvidsapi.com/api/upload/server?key=" + vh.apiKey

	ctx := context.Background()
	client := &http.Client{
		Timeout: 30 * time.Minute,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("[vidhide.getUploadServer] failed to create request: %w", err)
	}

	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("[vidhide.getUploadServer] failed to execute request: %w", err)
	}
	defer res.Body.Close()

	var getUploadServerRes uqloadGetUploadServerResponse
	if err := json.NewDecoder(res.Body).Decode(&getUploadServerRes); err != nil {
		return "", fmt.Errorf("[vidhide.getUploadServer] failed to decode json: %w", err)
	}

	return getUploadServerRes.Result, nil
}
