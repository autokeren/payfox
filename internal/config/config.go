package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type MCPServer struct {
	Name    string            `yaml:"name"`
	Command []string          `yaml:"command"`
	Env     map[string]string `yaml:"env,omitempty"`
}

type Messenger struct {
	AccessToken string `yaml:"access_token,omitempty"`
	VerifyToken string `yaml:"verify_token,omitempty"`
	AppSecret   string `yaml:"app_secret,omitempty"`
	PageID      string `yaml:"page_id,omitempty"`
}

type WhatsApp struct {
	AccessToken string `yaml:"access_token,omitempty"`
	PhoneID      string `yaml:"phone_id,omitempty"`
	VerifyToken string `yaml:"verify_token,omitempty"`
}

type Config struct {
	MetaAPIKey   string     `yaml:"meta_api_key"`
	BaseURL      string     `yaml:"base_url,omitempty"`
	Model        string     `yaml:"model,omitempty"`
	Workspace    string     `yaml:"workspace"`
	PublicURL     string     `yaml:"public_url,omitempty"`
	MemoryDir     string     `yaml:"memory_dir,omitempty"`
	MCPServers    []MCPServer `yaml:"mcp_servers,omitempty"`
	Port         int        `yaml:"port,omitempty"`
	Messenger    Messenger  `yaml:"messenger,omitempty"`
	WhatsApp     WhatsApp   `yaml:"whatsapp,omitempty"`
}

func Dir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".kancil"
	}
	return filepath.Join(home, ".kancil")
}

func Path() string {
	return filepath.Join(Dir(), "config.yaml")
}

func Load() (*Config, error) {
	raw, err := os.ReadFile(Path())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	if c.BaseURL == "" {
		c.BaseURL = "https://api.meta.ai/v1"
	}
	if c.Model == "" {
		c.Model = "muse-spark-1.3"
	}
	if c.Port == 0 {
		c.Port = 8487
	}
	if c.PublicURL == "" {
		c.PublicURL = "http://localhost:8487"
	}
	if c.MemoryDir == "" {
		c.MemoryDir = filepath.Join(Dir(), "memory")
	}
	return &c, nil
}

func (c *Config) Save() error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	raw, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(Path(), raw, 0o600)
}

// Validate ensures the minimum to run: key + workspace.
func (c *Config) Validate() error {
	if c.MetaAPIKey == "" {
		return fmt.Errorf("meta api key belum di-set — jalankan 'kancil' untuk wizard")
	}
	if c.Workspace == "" {
		return fmt.Errorf("workspace belum di-set")
	}
	return nil
}
