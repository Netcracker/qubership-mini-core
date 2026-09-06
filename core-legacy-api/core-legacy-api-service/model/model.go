package model

import "github.com/google/uuid"

type ApplicationWithProfiles struct {
	Name     string   `json:"name"`
	Profiles []string `json:"profiles"`
}
type ConfigProfile struct {
	ID          uuid.UUID        `json:"id"`
	Application string           `json:"application"`
	Profile     string           `json:"profile"`
	Version     int              `json:"version"`
	Properties  []ConfigProperty `json:"properties"`
}

// SetPropertiesFromMap replaces the Properties slice with the values from the map.
func (c *ConfigProfile) SetPropertiesFromMap(source map[string]ConfigProperty) {
	properties := make([]ConfigProperty, 0, len(source))
	for _, property := range source {
		properties = append(properties, property)
	}
	c.Properties = properties
}

// GetPropertiesAsMap returns the properties keyed by property key.
func (c *ConfigProfile) GetPropertiesAsMap() map[string]ConfigProperty {
	result := make(map[string]ConfigProperty, len(c.Properties))
	for _, property := range c.Properties {
		result[property.Key] = property
	}
	return result
}

type ConfigProperty struct {
	ID        uuid.UUID `json:"id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Encrypted *bool     `json:"encrypted"`
}

// IsEncrypted returns false if Encrypted is nil.
func (c *ConfigProperty) IsEncrypted() bool {
	if c.Encrypted == nil {
		return false
	}
	return *c.Encrypted
}

type Environment struct {
	Name            string           `json:"name"`
	Profiles        []string         `json:"profiles"`
	Label           *string          `json:"label"`
	Version         string           `json:"version"`
	State           *string          `json:"state"`
	PropertySources []PropertySource `json:"propertySources"`
}

type PropertySource struct {
	Name   string            `json:"name"`
	Source map[string]string `json:"source"`
}

func (e *Environment) AddPropertySource(ps PropertySource) {
	e.PropertySources = append(e.PropertySources, ps)
}
