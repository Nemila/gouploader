package uploader

import (
	"log/slog"
	"os"
	"path/filepath"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const chatId = -1003891110576

func UploadToChannel(log *slog.Logger, bot *tgbotapi.BotAPI, filePath string) (bool, error) {
	log.Info("Uploading file to telegram")

	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	doc := tgbotapi.NewDocument(chatId, tgbotapi.FileReader{
		Name:   filepath.Base(file.Name()),
		Reader: file,
	})
	doc.Caption = filepath.Base(file.Name())
	if _, err := bot.Send(doc); err != nil {
		return false, err
	}

	return true, nil
}
