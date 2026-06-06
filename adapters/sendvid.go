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
	"strings"
)

type Sendvid struct {
	apiKey string
	client *http.Client
}

type sendvidUploadResponse struct {
	Status int `json:"status"`
	Video  struct {
		Slug   string `json:"slug"`
		Secret string `json:"secret"`
	} `json:"video"`
}

func (s *Sendvid) ping() (string, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://sendvid.com/?li=t", nil)
	if err != nil {
		return "", err
	}

	res, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	html, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`meta content="([^"]+)" name="csrf-token"`)
	matches := re.FindStringSubmatch(string(html))
	if len(matches) < 2 {
		return "", fmt.Errorf("CSRF token not found")
	}

	return matches[1], nil
}

func (s *Sendvid) login() error {
	token, err := s.ping()
	if err != nil {
		return err
	}

	keys := strings.SplitN(s.apiKey, ":", 2)

	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)

	if err := w.WriteField("user[email]", keys[0]); err != nil {
		return err
	}

	if err := w.WriteField("user[password]", keys[1]); err != nil {
		return err
	}

	if err := w.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://sendvid.com/users/sign_in", buf)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-CSRF-Token", token)

	res, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var loginRes struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(res.Body).Decode(&loginRes); err != nil {
		return err
	}

	if !loginRes.Success {
		return fmt.Errorf("login failed")
	}

	return nil
}

func (s *Sendvid) Upload(filePath string) (string, error) {
	if err := s.login(); err != nil {
		return "", err
	}

	token, err := s.ping()
	if err != nil {
		return "", err
	}

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

		if err := w.WriteField("authenticity_token", token); err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		part, err := w.CreateFormFile("video", filepath.Base(file.Name()))
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		if _, err := io.Copy(part, file); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
	}()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://sendvid.com/api/v1/videos", pr)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-CSRF-Token", token)

	res, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("unexpected status: %s", res.Status)
	}

	var data sendvidUploadResponse
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", err
	}

	if len(data.Video.Slug) < 1 {
		return "", fmt.Errorf("invalid video slug %s", data.Video.Slug)
	}

	return data.Video.Slug, nil
}
