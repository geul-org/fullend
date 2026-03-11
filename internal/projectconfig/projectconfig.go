package projectconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProjectConfig represents the fullend.yaml project configuration.
type ProjectConfig struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Backend    Backend  `yaml:"backend"`
	Frontend   Frontend `yaml:"frontend"`
	Deploy     Deploy   `yaml:"deploy"`
	Session    *BuiltinBackend `yaml:"session"`
	Cache      *BuiltinBackend `yaml:"cache"`
	File       *FileBackend    `yaml:"file"`
}

type Metadata struct {
	Name string `yaml:"name"`
}

type Backend struct {
	Lang       string   `yaml:"lang"`
	Framework  string   `yaml:"framework"`
	Module     string   `yaml:"module"`
	Middleware []string `yaml:"middleware"`
	Auth       *Auth    `yaml:"auth"`
}

type Auth struct {
	SecretEnv string            `yaml:"secret_env"`
	Claims    map[string]string `yaml:"claims"` // FieldName → claim key (e.g. "ID" → "user_id")
}

type Frontend struct {
	Lang      string `yaml:"lang"`
	Framework string `yaml:"framework"`
	Bundler   string `yaml:"bundler"`
	Name      string `yaml:"name"`
}

type Deploy struct {
	Image  string `yaml:"image"`
	Domain string `yaml:"domain"`
}

// BuiltinBackend configures session/cache backend (postgres | memory).
type BuiltinBackend struct {
	Backend string `yaml:"backend"` // "postgres" or "memory"
}

// FileBackend configures file storage backend (s3 | local).
type FileBackend struct {
	Backend string     `yaml:"backend"` // "s3" or "local"
	S3      *S3Config  `yaml:"s3"`
	Local   *LocalConfig `yaml:"local"`
}

type S3Config struct {
	Bucket string `yaml:"bucket"`
	Region string `yaml:"region"`
}

type LocalConfig struct {
	Root string `yaml:"root"`
}

// Load reads and parses fullend.yaml from the given specs directory root.
func Load(specsDir string) (*ProjectConfig, error) {
	path := filepath.Join(specsDir, "fullend.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("fullend.yaml not found: %w", err)
	}

	var cfg ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("fullend.yaml parse error: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks required fields.
func (c *ProjectConfig) Validate() error {
	if c.APIVersion != "fullend/v1" {
		return fmt.Errorf("fullend.yaml: apiVersion must be \"fullend/v1\", got %q", c.APIVersion)
	}
	if c.Kind != "Project" {
		return fmt.Errorf("fullend.yaml: kind must be \"Project\", got %q", c.Kind)
	}
	if c.Metadata.Name == "" {
		return fmt.Errorf("fullend.yaml: metadata.name is required")
	}
	if c.Backend.Module == "" {
		return fmt.Errorf("fullend.yaml: backend.module is required")
	}
	return nil
}
