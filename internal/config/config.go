package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DNSPort         int    `yaml:"dns_port"`
	HTTPPort        int    `yaml:"http_port"`
	HTTPSPort       int    `yaml:"https_port"`
	UpstreamDNS     string `yaml:"upstream_dns"`
	Storage         string `yaml:"storage"`
	SitesDir        string `yaml:"sites_dir"`
	LetsEncryptEmail string `yaml:"letsencrypt_email"`
	LetsEncryptDir  string `yaml:"letsencrypt_dir"`
	AdminToken      string `yaml:"admin_token"`
}

func Default() *Config {
	return &Config{
		DNSPort:         53,
		HTTPPort:        80,
		HTTPSPort:       443,
		UpstreamDNS:     "1.1.1.1:53",
		Storage:         "./storage/registry.db",
		SitesDir:        "./sites",
		LetsEncryptEmail: "",
		LetsEncryptDir:  "./storage/certs",
		AdminToken:      "change-me",
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if cfg.AdminToken == "" || cfg.AdminToken == "change-me" {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			return nil, fmt.Errorf("failed to generate admin token: %w", err)
		}
		cfg.AdminToken = hex.EncodeToString(buf)
		log.Printf("[security] generated random admin token (set admin_token in config to override)")
	}
	return cfg, nil
}
