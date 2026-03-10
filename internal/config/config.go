package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type File struct {
	Path              string
	Auth              AuthStorage
	Source            Source        `yaml:"source"`
	Sink              Sink          `yaml:"sink"`
	Filters           []Filter      `yaml:"filters,omitempty"`
	Transformations   []Transformer `yaml:"transformations,omitempty"`
	Sync              Sync          `yaml:"sync"`
	UpdateConcurrency int           `yaml:"updateConcurrency,omitempty"`
}

func NewFromFile(path string) (*File, error) {
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}

	config := File{
		Path: path,
		Auth: AuthStorage{
			StorageMode: "yaml",
			Config: CustomMap{
				"path": "./auth-storage.yaml",
			},
		},
	}

	if err := yaml.Unmarshal(yamlFile, &config); err != nil {
		return nil, fmt.Errorf("cannot unmarshal config file: %w", err)
	}

	return &config, nil
}

type AuthStorage struct {
	StorageMode string `yaml:"storage_mode" json:"storageMode"`
	// Any kind of parameter which can be passed to the StorageMode
	Config CustomMap `yaml:"config" json:"config"`
}

type Source struct {
	Adapter Adapter `yaml:"adapter" json:"adapter"`
}

type Sink struct {
	Adapter Adapter `yaml:"adapter" json:"adapter"`
}

type Adapter struct {
	// Type of the adapter (client) to use for the source calendar
	Type string `yaml:"type" json:"type"`
	// ID of the calendar in which the adapter will work.
	Calendar string `yaml:"calendar" json:"calendar"`
	// CustomMap is an adapter-specific map to configure it.
	Config CustomMap `yaml:"config" json:"config"`
	// OAuth values for the adapter
	OAuth OAuth `yaml:"oAuth" json:"oAuth"`
}

type OAuth struct {
	ClientID  string `yaml:"clientId,omitempty" json:"clientId,omitempty"`
	ClientKey string `yaml:"clientKey,omitempty" json:"clientKey,omitempty"`
	TenantID  string `yaml:"tenantId,omitempty" json:"tenantId,omitempty"`
}

// CustomMap is meant to provide custom parameters to different adapters/transformers.
type CustomMap map[string]interface{}

// Transformer configures the name
type Transformer struct {
	// Name of the transformer to run
	Name string `yaml:"name" json:"name"`
	// Any kind of parameter which can be passed to a transformer.
	Config CustomMap `yaml:"config" json:"config"`
}

type Filter struct {
	// Name of the filter
	Name string `yaml:"name" json:"name"`
	// Any kind of parameter which can be passed to a filter.
	Config CustomMap `yaml:"config" json:"config"`
}

// Sync configuration
type Sync struct {
	StartTime SyncTime `yaml:"start" json:"start"`
	EndTime   SyncTime `yaml:"end" json:"end"`
}

type SyncTime struct {
	Identifier string `yaml:"identifier" json:"identifier"`
	Offset     int    `yaml:"offset,omitempty" json:"offset,omitempty"`
}
