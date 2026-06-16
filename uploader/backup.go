package uploader

import (
	"os"
	"path/filepath"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const chatId = -1003702063699

func UploadToChannel(bot *tgbotapi.BotAPI, filePath string) (bool, error) {

	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	video := tgbotapi.NewVideo(chatId, tgbotapi.FileReader{
		Name:   filepath.Base(file.Name()),
		Reader: file,
	})
	video.Caption = filepath.Base(file.Name())
	if _, err := bot.Send(video); err != nil {
		return false, err
	}

	return true, nil
}
