package uploader

import (
	"io"
	"os"
	"path/filepath"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const chatId = -1003702063699

func UploadToChannel(token, endpoint, filePath string) (bool, error) {
	bot, err := tgbotapi.NewBotAPIWithAPIEndpoint(token, endpoint)
	if err != nil {
		return false, err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return false, err
	}

	video := tgbotapi.NewVideo(chatId, tgbotapi.FileBytes{
		Bytes: b,
		Name:  filepath.Base(file.Name()),
	})
	video.Caption = filepath.Base(file.Name())
	if _, err := bot.Send(video); err != nil {
		return false, err
	}

	return true, nil
}
