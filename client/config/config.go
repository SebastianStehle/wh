package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Configuration struct {
	Servers []Server `json:"servers"`
	Server  string   `json:"server"`
}

type Server struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	ApiKey   string `json:"apiKey"`
}

func GetServer() (*Server, error) {
	config, err := GetConfiguration()
	if err != nil {
		return nil, err
	}

	for _, server := range config.Servers {
		if server.Name == config.Server {
			return &server, nil
		}
	}

	if len(config.Servers) > 0 {
		return &config.Servers[0], nil
	}

	err = fmt.Errorf("failed to get server. Maybe the config file is missing. Try the `config add` command")

	return nil, err
}

func StoreConfiguration(config *Configuration) error {
	configFile, err := getConfigFile()
	if err != nil {
		return err
	}

	jsonData, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to convert to JSON: %v", err)
	}

	file, err := os.Create(configFile)
	if err != nil {
		return fmt.Errorf("failed to create config file: %v", err)
	}

	defer file.Close()

	_, err = file.Write(jsonData)
	if err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	return nil
}

func GetConfiguration() (*Configuration, error) {
	configFile, err := getConfigFile()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		defaultConfig := Configuration{
			Servers: make([]Server, 0),
			Server:  "",
		}

		return &defaultConfig, nil
	}

	jsonData, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	config := Configuration{}

	err = json.Unmarshal(jsonData, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to convert from JSON: %v", err)
	}

	return &config, nil
}

func GetWorkingDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to fetch user directory: %v", err)
	}

	configDir := filepath.Join(homeDir, ".wh")

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		err := os.MkdirAll(configDir, os.ModePerm)
		if err != nil {
			return "", fmt.Errorf("failed to create config directory: %v", err)
		}
	}

	return configDir, nil
}

func getConfigFile() (string, error) {
	workingDir, err := GetWorkingDir()
	if err != nil {
		return workingDir, err
	}

	configFile := filepath.Join(workingDir, "/config.json")
	return configFile, nil
}
