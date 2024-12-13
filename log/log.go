package log

import (
	"os"

	"github.com/rs/zerolog/log"
)

type MsLog struct {
}

func (mslog *MsLog) Init(path string) {
	logFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	log.Logger = log.Output(logFile)

}

func (mslog *MsLog) Info(msg string) {
	log.Info().Msg(msg)
}

func (mslog *MsLog) Warn(msg string) {
	log.Warn().Msg(msg)
}

func (mslog *MsLog) Error(msg string) {
	log.Error().Msg(msg)
}
