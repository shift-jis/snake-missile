package application

import (
	"encoding/json"
	"os"

	"github.com/dablelv/cyan/file"
)

type ProgramProperties struct {
	ServerAddress string `json:"server_address"`
	ProxyListPath string `json:"proxy_list_path"`
	BotsPerProxy  int    `json:"bots_per_proxy"`
}

func (properties *ProgramProperties) ReadProxyList() ([]string, error) {
	return file.ReadLines(properties.ProxyListPath)
}

type ProgramArguments struct {
	PropertiesPath string `short:"c" long:"config" default:"snake-missile.json" description:"Path to the property file"`
}

func (arguments *ProgramArguments) LoadProperties() (*ProgramProperties, error) {
	fileBytes, err := os.ReadFile(arguments.PropertiesPath)
	if err != nil {
		return nil, err
	}

	var properties ProgramProperties
	err = json.Unmarshal(fileBytes, &properties)
	return &properties, err
}
