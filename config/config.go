package config

import (
	"fmt"
	"log/slog"
	"os"
	"slices"

	"github.com/joho/godotenv"
)

type Config struct {
	MediaPath     string
	WebsiteUrl    string
	AbyssKey      string
	VidhideKey    string
	VidhideSessId string
	UqloadKey     string
	UqloadSessId  string
	SendvidKey    string
	Env           string
	TgToken       string
	TgEndpoint    string
}

func folderExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

func Load() (*Config, error) {
	godotenv.Load()

	mediaPath := os.Getenv("MEDIA_PATH")
	if mediaPath == "" {
		return nil, fmt.Errorf("MEDIA_PATH is missing in env")
	}

	if !folderExists(mediaPath) {
		return nil, fmt.Errorf("❌ The folder %s does not exist (or is a file)", mediaPath)
	}

	tgToken := os.Getenv("TG_TOKEN")
	if tgToken == "" {
		return nil, fmt.Errorf("TG_TOKEN is missing in env")
	}

	tgEndpoint := os.Getenv("TG_ENDPOINT")
	if tgEndpoint == "" {
		return nil, fmt.Errorf("TG_ENDPOINT is missing in env")
	}

	websiteUrl := os.Getenv("WEBSITE_URL")
	if websiteUrl == "" {
		return nil, fmt.Errorf("WEBSITE_URL is missing in env")
	}

	abyssKey := os.Getenv("ABYSS_KEY")
	if abyssKey == "" {
		return nil, fmt.Errorf("ABYSS_KEY is missing in env")
	}

	vidhideKey := os.Getenv("VIDHIDE_KEY")
	if vidhideKey == "" {
		return nil, fmt.Errorf("VIDHIDE_KEY is missing in env")
	}

	videhideSessId := os.Getenv("VIDHIDE_SESSID")
	if videhideSessId == "" {
		return nil, fmt.Errorf("VIDHIDE_SESSID is missing in env")
	}

	uqloadKey := os.Getenv("UQLOAD_KEY")
	if uqloadKey == "" {
		return nil, fmt.Errorf("UQLOAD_KEY is missing in env")
	}

	uqloadSessId := os.Getenv("UQLOAD_SESSID")
	if uqloadSessId == "" {
		return nil, fmt.Errorf("UQLOAD_SESSID is missing in env")
	}

	sendvidKey := os.Getenv("SENDVID_KEY")
	if sendvidKey == "" {
		return nil, fmt.Errorf("SENDVID_KEY is missing in env")
	}

	envTypes := []string{"DEV", "PROD"}
	env := os.Getenv("ENV")
	if env == "" {
		return nil, fmt.Errorf("ENV is missing in env")
	}
	if isValid := slices.Contains(envTypes, env); !isValid {
		return nil, fmt.Errorf("invalid ENV value in env")
	}

	return &Config{
		MediaPath:     mediaPath,
		WebsiteUrl:    websiteUrl,
		AbyssKey:      abyssKey,
		VidhideKey:    vidhideKey,
		VidhideSessId: videhideSessId,
		UqloadKey:     uqloadKey,
		UqloadSessId:  uqloadSessId,
		SendvidKey:    sendvidKey,
		Env:           env,
		TgToken:       tgToken,
		TgEndpoint:    tgEndpoint,
	}, nil
}

func NewLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}
