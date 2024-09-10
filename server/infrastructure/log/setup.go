package log

import (
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func NewLogger(config *viper.Viper) (*zap.Logger, error) {
	logLevel := config.GetString("log.mode")

	if strings.ToLower(logLevel) == "production" {
		return zap.NewProduction()
	} else {
		return zap.NewDevelopment()
	}
}
