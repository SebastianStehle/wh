package auth

import (
	"github.com/spf13/viper"
)

type authenticator struct {
	apiKey string
}

type Authenticator interface {
	Validate(apiKey string) bool
}

func NewAuthenticator(config *viper.Viper) Authenticator {
	return &authenticator{
		apiKey: config.GetString("auth.apiKey"),
	}
}

func (a authenticator) Validate(apiKey string) bool {
	return a.apiKey == apiKey
}
