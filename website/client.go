package website

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

type Client struct {
	BaseUrl    string
	HttpClient *http.Client
}

type CheckFileRes struct {
	Exists bool   `json:"exists"`
	Msg    string `json:"msg"`
}

type ImportRes struct {
	Msg string `json:"msg"`
}

func (c *Client) CheckFile(filePath, hostName string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	parsedUrl, err := url.Parse(fmt.Sprintf("%s/api/import/check-host", c.BaseUrl))
	if err != nil {
		return false, err
	}

	params := url.Values{}
	params.Add("fileName", filepath.Base(file.Name()))
	params.Add("hostName", hostName)
	parsedUrl.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, parsedUrl.String(), nil)
	if err != nil {
		return false, err
	}

	res, err := c.HttpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	var data CheckFileRes
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return false, err
	}

	if res.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to check %s", data.Msg)
	}
	return data.Exists, nil
}

func (c *Client) ImportToWebsite(log *slog.Logger, filePath, hostName, slug string) error {
	parsedUrl, err := url.Parse(fmt.Sprintf("%s/api/import", c.BaseUrl))
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Add("fileName", filepath.Base(filePath))
	params.Add("hostName", hostName)
	params.Add("slug", slug)
	parsedUrl.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, parsedUrl.String(), nil)
	if err != nil {
		return err
	}

	res, err := c.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var data ImportRes
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to import to website: %s", data.Msg)
	}

	log.Info("File imported", "data", data)
	return nil
}
