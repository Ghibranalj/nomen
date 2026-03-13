package main

import "github.com/spf13/viper"

type Mikrotik struct {
	Name    string `mapstructure:"name"`
	IP       string `mapstructure:"IP"`
	Port     int    `mapstructure:"Port"`
	User     string `mapstructure:"User"`
	Password string `mapstructure:"Password"`
	TLD      string `mapstructure:"TLD"`
}

type Config struct {
	Mikrotik              []Mikrotik `mapstructure:"Mikrotik"`
	DohServer             string     `mapstructure:"DohServer"`
	RedisURL              string     `mapstructure:"RedisURL"`
	Port                  int        `mapstructure:"Port"`
	Proto                 string     `mapstructure:"Proto"`
	ScrapeIntervalMinutes int        `mapstructure:"ScrapeIntervalMinutes"`
	DnsTTLMinutes         int        `mapstructure:"DnsTTLMinutes"`
	RouterTLD             string     `mapstructure:"RouterTLD"` // Global TLD for router names
}

var (
	cfg = Config{}
)

func ReadConfig(pathToConfig string) error {
	v := viper.New()
	v.SetConfigFile(pathToConfig)

	if err := v.ReadInConfig(); err != nil {
		return err
	}

	return v.Unmarshal(&cfg)
}
