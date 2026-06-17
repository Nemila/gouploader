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
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
)

type Uqload struct {
	apiKey string
	sessId string
	client *http.Client
}

type uqloadGetUploadServer struct {
	Msg        string `json:"msg"`
	ServerTime string `json:"server_time"`
	Status     int    `json:"status"`
	Result     string `json:"result"`
}

func (u *Uqload) getContentLength(filePath string) (int64, error) {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)

	f, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	if _, err := w.CreateFormFile("file", filepath.Base(f.Name())); err != nil {
		return 0, err
	}

	if err := w.WriteField("sess_id", u.sessId); err != nil {
		return 0, err
	}

	if err := w.Close(); err != nil {
		return 0, err
	}

	i, err := f.Stat()
	if err != nil {
		return 0, err
	}

	return i.Size() + int64(buf.Len()), nil
}

func (u *Uqload) getUploadServer() (string, error) {
	url := fmt.Sprintf("https://uqload.is/api/upload/server?key=%s", u.apiKey)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	res, err := u.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("[uqload.getUploadServer] failed to execute request: %w", err)
	}
	defer res.Body.Close()

	var data uqloadGetUploadServer
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", err
	}

	if data.Status != 200 || len(data.Result) < 1 {
		return "", fmt.Errorf("failed to get upload server %s", data.Msg)
	}

	return data.Result, nil
}

func (u *Uqload) Upload(filePath string) (string, error) {
	pr, pw := io.Pipe()
	w := multipart.NewWriter(pw)

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	fileName := filepath.Base(file.Name())

	go func() {
		defer pw.Close()
		defer w.Close()

		if err := w.WriteField("sess_id", u.sessId); err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		part, err := w.CreateFormFile("file", fileName)
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

	url, err := u.getUploadServer()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, pr)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", w.FormDataContentType())

	n, err := u.getContentLength(filePath)
	if err != nil {
		return "", err
	}
	req.ContentLength = n

	res, err := u.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("uqload upload failed %s", res.Status)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return "", err
	}
	slug := doc.Find(`[name="fn"]`).Text()

	if fileName == slug || utf8.RuneCountInString(slug) < 1 {
		// htmlContent, err := doc.Html()
		// if err != nil {
		// 	return "", err
		// }
		msg := doc.Find(`[name="st"]`).Text()
		return "", fmt.Errorf("failed to upload or extract slug %s", msg)
	}

	return slug, nil
}
