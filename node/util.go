package node

import (
	"github.com/BurntSushi/toml"
)

type Service struct {
	Address  string `toml:"address"`
	MaxConns int    `toml:"max_conns"`
	Enabled  bool   `toml:"enabled"`
	Timeout  string `toml:"timeout"`
	Session  map[string]string
}

type logger struct {
	Path  string `toml:"path, omitempty"`
	Level string `toml:"level, omitempty"`
}

type Sys struct {
	Threads int `toml:"threads"`
}

type API struct {
	Apikey   string `toml:"apikey"`
	Model    string `toml:"model"`
	UseProxy bool   `toml:"useproxy"`
}

type Config struct {
	Sys     Sys
	API     API
	Service Service
	Logger  logger
}

func ReadConfig(path string) (*Config, error) {
	config := &Config{}
	if _, err := toml.DecodeFile(path, config); err != nil {
		return nil, err
	}
	return config, nil
}
