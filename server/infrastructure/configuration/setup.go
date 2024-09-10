package configuration

import "github.com/spf13/viper"

func NewConfig() (*viper.Viper, error) {
	config := viper.New()
	config.AddConfigPath("./configs")
	config.SetConfigName("wh")
	config.SetConfigType("json")
	config.AutomaticEnv()

	if err := config.ReadInConfig(); err != nil {
		return nil, err
	}

	return config, nil
}
