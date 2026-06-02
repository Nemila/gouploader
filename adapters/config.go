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
	"abyss":  NewAbyssAdapter(os.Getenv("ABYSS_KEY")),
	"uqload": NewUqloadAdapter(os.Getenv("UQLOAD_KEY")),
}
