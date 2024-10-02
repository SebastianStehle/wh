package domain

import (
	"time"

	"github.com/spf13/viper"
)

func SetDefaultConfig(config *viper.Viper) {
	config.SetDefault("dataFolder", "./data")
	config.SetDefault("auth.apiKey", "KEY")
	config.SetDefault("auth.blockKey", "siTAgTsT51hkE64ltan7eCLbV9exuKIX")
	config.SetDefault("auth.hashKey", "xTxxg9fCasLXVRGe5dvHTLO6zKGAaOKz")
	config.SetDefault("grpc.address", "0.0.0.0:5010")
	config.SetDefault("http.address", "0.0.0.0:5000")
	config.SetDefault("log.maxEntries", 100)
	config.SetDefault("log.maxSize", 100_000_000)
	config.SetDefault("request.maxSize", 10_000_000)
	config.SetDefault("request.timeout", 30*time.Minute)
}
