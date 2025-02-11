package config

import (
	"encoding/json"
	"io"
	"log"
	"os"
)

type ConnectionType int

const (
	UNKNOWN = iota
	SERIAL_CONNECTION
	TCP_CONNECTION
)

type Config struct {
	Connections []Connection `json:"connections"`
	Settings    Settings     `json:"settings"`
}

type Connection struct {
	ConnectionType ConnectionType
	Name           string `json:"name"`
	Hostname       string `json:"hostname"`
	Port           int    `json:"port"`
	SerialDevice   string `json:"device"`
}

type Settings struct {
	AllowTCP      bool `json:"allow_tcp"`
	AllowSerial   bool `json:"allow_serial"`
	AllowTransmit bool `json:"allow_transmit"`
}

var config Config

func InitConfig() {
	configFile, err := os.Open("config.json")
	if err != nil {
		log.Fatal(err)
	}
	configBytes, _ := io.ReadAll(configFile)
	json.Unmarshal(configBytes, &config)
	for i, connection := range config.Connections {
		if connection.Port == 0 {
			config.Connections[i].Port = 4403
		}
		if connection.Hostname != "" {
			config.Connections[i].ConnectionType = TCP_CONNECTION
		}
		if connection.SerialDevice != "" {
			config.Connections[i].ConnectionType = SERIAL_CONNECTION
		}
	}
}

func GetConfig() Config {
	return config
}
