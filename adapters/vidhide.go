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
)

type Vidhide struct {
	apiKey string
	sessId string
	client *http.Client
}

type vidhideGetUploadServer struct {
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
		return 0, err
	}
	defer file.Close()

	if err := writer.WriteField("sess_id", vh.sessId); err != nil {
		return 0, err
	}

	if _, err := writer.CreateFormFile("file", filepath.Base(file.Name())); err != nil {
		return 0, err
	}

	if err := writer.Close(); err != nil {
		return 0, err
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return 0, err
	}

	return fileInfo.Size() + int64(bodyBuffer.Len()), nil
}

func (vh *Vidhide) getUploadServer() (string, error) {
	url := fmt.Sprintf("https://earnvidsapi.com/api/upload/server?key=%s", vh.apiKey)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	res, err := vh.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var data vidhideGetUploadServer
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", err
	}

	if data.Status != 200 || len(data.Result) < 1 {
		return "", fmt.Errorf("failed to get upload server %s", data.Msg)
	}

	return data.Result, nil
}

func (vh *Vidhide) Upload(filePath string) (string, error) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	fileName := filepath.Base(file.Name())

	go func() {
		defer pw.Close()
		defer writer.Close()

		if err := writer.WriteField("sess_id", vh.sessId); err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		part, err := writer.CreateFormFile("file", fileName)
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

	url, err := vh.getUploadServer()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, pr)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	n, err := vh.getContentLength(filePath)
	if err != nil {
		return "", err
	}
	req.ContentLength = n

	res, err := vh.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vidhide upload failed %s", res.Status)
	}

	html, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`name="fn">([^<]+)`)
	matches := re.FindStringSubmatch(string(html))
	if len(matches) < 1 {
		return "", fmt.Errorf("failed to extract slug")
	}

	if fileName == matches[1] {
		return "", fmt.Errorf("failed to upload, session may have expired")
	}

	return matches[1], nil
}
