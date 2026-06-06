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
	"regexp"
	"strings"
)

type Sendvid struct {
	apiKey string
	client *http.Client
}

// func (s *Sendvid) getContentLength(filePath string) (int64, error) {
// 	bodyBuffer := &bytes.Buffer{}
// 	writer := multipart.NewWriter(bodyBuffer)

// 	file, err := os.Open(filePath)
// 	if err != nil {
// 		return 0, fmt.Errorf("[uqload.getContentLength] failed to open file: %w", err)
// 	}

// 	defer file.Close()

// 	if err := writer.WriteField("sess_id", s.sessId); err != nil {
// 		return 0, fmt.Errorf("[uqload.getContentLength] failed to write sess_id: %w", err)
// 	}

// 	_, err = writer.CreateFormFile("file", filepath.Base(file.Name()))
// 	if err != nil {
// 		return 0, fmt.Errorf("[uqload.getContentLength] failed to create form file: %w", err)
// 	}

// 	fileInfo, err := file.Stat()
// 	if err != nil {
// 		return 0, fmt.Errorf("[uqload.getContentLength] failed to get file stat: %w", err)
// 	}

// 	_ = writer.Close()
// 	contentLength := fileInfo.Size() + int64(bodyBuffer.Len())

// 	return contentLength, nil
// }

type LoginResponse struct {
	Success bool `json:"success"`
}

func (s *Sendvid) Ping() (string, error) {
	ctx := context.Background()
	url := "https://sendvid.com/?li=t"

	re := regexp.MustCompile(`meta content="([^"]+)" name="csrf-token"`)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("[sendvid.Ping] failed to create request: %w", err)
	}

	res, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("[sendvid.Ping] failed to execute request: %w", err)
	}
	defer res.Body.Close()

	html, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("[sendvid.Ping] failed to read response body: %w", err)
	}

	matches := re.FindStringSubmatch(string(html))
	if len(matches) < 2 {
		return "", fmt.Errorf("[sendvid.Ping] CSRF token not found!")
	}

	scrapedToken := matches[1]
	return scrapedToken, nil
}

func (s *Sendvid) Login() error {
	token, err := s.Ping()
	if err != nil {
		return fmt.Errorf("[sendvid.Login] ping failed: %w", err)
	}

	keys := strings.SplitN(s.apiKey, ":", 2)

	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)

	if err := w.WriteField("user[email]", keys[0]); err != nil {
		return fmt.Errorf("[sendvid.Login] failed to write email: %w", err)
	}
	if err := w.WriteField("user[password]", keys[1]); err != nil {
		return fmt.Errorf("[sendvid.Login] failed to write password: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("[sendvid.Login] failed to close multipart writer: %w", err)
	}

	ctx := context.Background()
	url := "https://sendvid.com/users/sign_in"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-CSRF-Token", token)

	if err != nil {
		return fmt.Errorf("[sendvid.Login] failed to create request: %w", err)
	}

	res, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("[sendvid.Login] failed to execute request: %w", err)
	}
	defer res.Body.Close()

	var loginRes LoginResponse
	if err := json.NewDecoder(res.Body).Decode(&loginRes); err != nil {
		return fmt.Errorf("[sendvid.Login] failed to decode json: %w", err)
	}

	return nil
}

func (s *Sendvid) GetVideos() error {
	if err := s.Login(); err != nil {
		return fmt.Errorf("[sendvid.GetVideos] failed to login: %w", err)
	}

	ctx := context.Background()
	url := "https://sendvid.com/videos"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	if err != nil {
		return fmt.Errorf("[sendvid.GetVideos] failed to create request: %w", err)
	}

	res, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("[sendvid.GetVideos] failed to execute request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("[sendvid.GetVideos] failed to fetch page: %w", err)
	}

	html, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("[sendvid.GetVideos] failed to read response: %w", err)
	}
	fmt.Printf("\nserver returned: %s", html)
	return nil
}

// {"status":0,"video":{"slug":"u7spbhp5","secret":"3d34e318-4e64-4016-9b2c-e267451b6fd5"}}
type UploadResponse struct {
	Status string `json:"status"`
	Video  struct {
		Slug   string `json:"slug"`
		Secret string `json:"secret"`
	} `json:"video"`
}

func (s *Sendvid) Upload(filePath string) (string, error) {
	if err := s.Login(); err != nil {
		return "", fmt.Errorf("[sendvid.Upload] failed to login: %w", err)
	}

	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)

	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("[sendvid.Upload] failed to open file: %w", err)
	}
	defer file.Close()

	token, err := s.Ping()
	if err != nil {
		return "", fmt.Errorf("[sendvid.Upload] failed to ping server: %w", err)
	}

	if err := w.WriteField("utf8", "✓"); err != nil {
		return "", fmt.Errorf("[sendvid.Upload] failed to write authenticity_token: %w", err)
	}
	if err := w.WriteField("authenticity_token", token); err != nil {
		return "", fmt.Errorf("[sendvid.Upload] failed to write authenticity_token: %w", err)
	}

	part, err := w.CreateFormField("video")
	if err != nil {
		return "", fmt.Errorf("[sendvid.Upload] failed to create video: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return "", fmt.Errorf("[sendvid.Upload] failed to copy video: %w", err)
	}
	w.Close()

	url := "https://sendvid.com/api/v1/videos"
	ctx := context.Background()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, buf)
	if err != nil {
		return "", fmt.Errorf("[sendvid.Upload] failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	// req.Header.Set("X-Requested-With", "XMLHttpRequest")
	// req.Header.Set("Accept", "text/plain, */*; q=0.01")
	req.Header.Set("X-CSRF-Token", token)
	req.Header.Set("Session-Id", "15588222222")
	// req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:151.0) Gecko/20100101 Firefox/151.0")
	// req.Header.Set("Referer", "https://sendvid.com/")
	// req.Header.Set("Origin", "https://sendvid.com")
	// req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	// req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")

	res, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("[sendvid.Upload] request failed with status code %s", res.Status)
	}

	var uploadRes UploadResponse
	json.NewDecoder(res.Body).Decode(&uploadRes)

	return "", nil
}

// curl 'https://sendvid.com/api/v1/videos' \
//   -X POST \
//   -H 'User-Agent: Mozilla/5.0 (X11; Linux x86_64; rv:151.0) Gecko/20100101 Firefox/151.0' \
//   -H 'Accept: text/plain, */*; q=0.01' \
//   -H 'Accept-Language: en-US,en;q=0.9' \
//   -H 'Accept-Encoding: gzip, deflate, br, zstd' \
//   -H 'Referer: https://sendvid.com/' \
//   -H 'Origin: https://sendvid.com' \
//   -H 'DNT: 1' \
//   -H 'Connection: keep-alive' \
//   -H 'Cookie: gsc=IjE5YTQxMDg3LTA5NGQtNDYzMi1hYjJiLTFlYzRmY2M4NjVhNiI%3D--de709d32fcdd9240727cee56716bd6319c4940de; _sendvid_session=VU1ZVWsvaUl5c2hRTGtwajNPbjZVODBQRWx1WEg1TlJFV1gyWUk0ZDJ1RklYODVad21LWEs2ZUFaTjZJb1ZvS3NUcGVZTm5od3U0UUJWTDBRUjVaZzRSWkEyOElwLzNpK3lIcFlNYmk5RVFOblFnWW55VVdWeXZJejdVMGRxRDZGdDlydU5rT1JPajNNM0JoSG1jeTJYckh3SU00ckw2WC9SdXJqN3M3SzNzMDdhRGlyMTdiZ21PMVNjT3RhN0ZkcGFYZ1RYUCtLNXliTTJiN1BiNndoQktGUTR1U2FnWlA3R1dSblF2OEw5WUZtcEZHTndMRE1LVGhPSFN5Z0lESy0tRk5OK2szZUVISEZVZmxKOGJFbTExUT09--3ea850dba0aabcba2d0f88a79217cf761782f902; adpref=1' \
//   -H 'Sec-Fetch-Dest: empty' \
//   -H 'Sec-Fetch-Mode: no-cors' \
//   -H 'Sec-Fetch-Site: same-origin' \
//   -H 'Sec-GPC: 1' \
//   -H 'X-CSRF-Token: BfgPRyUghM9dEDVMoCBKcqQNemdeBxso34+cFS2H5SI=' \
//   -H 'X-Requested-With: XMLHttpRequest' \
//   -H 'Content-Type: multipart/form-data; boundary=----geckoformboundary7bca17a5974c296eb0c6c6b38c5477b3' \
//   -H 'Pragma: no-cache' \
//   -H 'Cache-Control: no-cache' \
//   -H 'Priority: u=4' \
//   --data-binary \
//   $'------geckoformboundary7bca17a5974c296eb0c6c6b38c5477b3\r\nContent-Disposition: form-data; name="browser"\r\n\r\n{"userAgent":"Mozilla/5.0 (X11; Linux x86_64; rv:151.0) Gecko/20100101 Firefox/151.0","plugins":[{"name":"PDF Viewer","description":"Portable Document Format"},{"name":"Chrome PDF Viewer","description":"Portable Document Format"},{"name":"Chromium PDF Viewer","description":"Portable Document Format"},{"name":"Microsoft Edge PDF Viewer","description":"Portable Document Format"},{"name":"WebKit built-in PDF","description":"Portable Document Format"}],"fipi":"86ae13ad300dbbbde7981486ee030eae"}\r\n------geckoformboundary7bca17a5974c296eb0c6c6b38c5477b3\r\nContent-Disposition: form-data; name="video"; filename="video.mp4"\r\nContent-Type: video/mp4\r\n\r\n------geckoformboundary7bca17a5974c296eb0c6c6b38c5477b3--\r\n'
