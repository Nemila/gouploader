package adapters

import (
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Adapter interface {
	Upload(filePath string) (string, error)
}

var err error = godotenv.Load()
var jar, _ = cookiejar.New(nil)

var Adapters map[string]Adapter = map[string]Adapter{
	"abyss": &Abyss{
		apiKey: os.Getenv("ABYSS_KEY"),
	},
	"uqload": &Uqload{
		apiKey: os.Getenv("UQLOAD_KEY"),
		sessId: os.Getenv("UQLOAD_SESSID"),
	},
	"vidhide": &Vidhide{
		apiKey: os.Getenv("VIDHIDE_KEY"),
		sessId: os.Getenv("VIDHIDE_SESSID"),
	},
	"sendvid": &Sendvid{
		apiKey: os.Getenv("SENDVID_KEY"),
		client: &http.Client{
			Timeout: 60 * time.Minute,
			Jar:     jar,
		},
	},
}
