package adapters

import (
	"os"

	"github.com/joho/godotenv"
)

type Adapter interface {
	Upload(filePath string) (string, error)
}

var err error = godotenv.Load()

var Adpaters map[string]Adapter = map[string]Adapter{
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
}
