package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- ConfigProfile.SetPropertiesFromMap ----------

func TestSetPropertiesFromMap(t *testing.T) {
	tests := []struct {
		name     string
		initial  []ConfigProperty
		source   map[string]ConfigProperty
		expected []ConfigProperty
	}{
		{
			name:     "NilMap",
			source:   nil,
			expected: []ConfigProperty{},
		},
		{
			name:     "EmptyMap",
			source:   map[string]ConfigProperty{},
			expected: []ConfigProperty{},
		},
		{
			name: "SingleProperty",
			source: map[string]ConfigProperty{
				"a": {Key: "a", Value: "1"},
			},
			expected: []ConfigProperty{{Key: "a", Value: "1"}},
		},
		{
			name: "MultipleProperties",
			source: map[string]ConfigProperty{
				"a": {Key: "a", Value: "1"},
				"b": {Key: "b", Value: "2"},
				"c": {Key: "c", Value: "3"},
			},
			expected: []ConfigProperty{
				{Key: "a", Value: "1"},
				{Key: "b", Value: "2"},
				{Key: "c", Value: "3"},
			},
		},
		{
			name:    "ReplacesExistingProperties",
			initial: []ConfigProperty{{Key: "old", Value: "old-value"}},
			source: map[string]ConfigProperty{
				"new": {Key: "new", Value: "new-value"},
			},
			expected: []ConfigProperty{{Key: "new", Value: "new-value"}},
		},
		{
			name:     "EmptyMapClearsExistingProperties",
			initial:  []ConfigProperty{{Key: "old", Value: "old-value"}},
			source:   map[string]ConfigProperty{},
			expected: []ConfigProperty{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := &ConfigProfile{Properties: tt.initial}

			profile.SetPropertiesFromMap(tt.source)

			require.NotNil(t, profile.Properties)
			// Map iteration order is random, so compare without order.
			assert.ElementsMatch(t, tt.expected, profile.Properties)
		})
	}
}

func TestSetPropertiesFromMap_UsesMapValuesNotKeys(t *testing.T) {
	// The map key is ignored; the Key field of each value is what ends up in the slice.
	profile := &ConfigProfile{}
	profile.SetPropertiesFromMap(map[string]ConfigProperty{
		"map-key": {Key: "property-key", Value: "v"},
	})

	assert.Equal(t, []ConfigProperty{{Key: "property-key", Value: "v"}}, profile.Properties)
}

// ---------- ConfigProfile.GetPropertiesAsMap ----------

func TestGetPropertiesAsMap(t *testing.T) {
	tests := []struct {
		name       string
		properties []ConfigProperty
		expected   map[string]ConfigProperty
	}{
		{
			name:       "NilProperties",
			properties: nil,
			expected:   map[string]ConfigProperty{},
		},
		{
			name:       "EmptyProperties",
			properties: []ConfigProperty{},
			expected:   map[string]ConfigProperty{},
		},
		{
			name:       "SingleProperty",
			properties: []ConfigProperty{{Key: "a", Value: "1"}},
			expected: map[string]ConfigProperty{
				"a": {Key: "a", Value: "1"},
			},
		},
		{
			name: "MultipleProperties",
			properties: []ConfigProperty{
				{Key: "a", Value: "1"},
				{Key: "b", Value: "2"},
			},
			expected: map[string]ConfigProperty{
				"a": {Key: "a", Value: "1"},
				"b": {Key: "b", Value: "2"},
			},
		},
		{
			name: "DuplicateKeysLastOneWins",
			properties: []ConfigProperty{
				{Key: "a", Value: "first"},
				{Key: "a", Value: "second"},
			},
			expected: map[string]ConfigProperty{
				"a": {Key: "a", Value: "second"},
			},
		},
		{
			name:       "EmptyValueIsKept",
			properties: []ConfigProperty{{Key: "a", Value: ""}},
			expected: map[string]ConfigProperty{
				"a": {Key: "a", Value: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := &ConfigProfile{Properties: tt.properties}

			result := profile.GetPropertiesAsMap()

			require.NotNil(t, result)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSetAndGetPropertiesRoundTrip(t *testing.T) {
	source := map[string]ConfigProperty{
		"a": {Key: "a", Value: "1"},
		"b": {Key: "b", Value: "2"},
	}
	profile := &ConfigProfile{}

	profile.SetPropertiesFromMap(source)

	assert.Equal(t, source, profile.GetPropertiesAsMap())
}

// ---------- Environment.AddPropertySource ----------

func TestAddPropertySource(t *testing.T) {
	t.Run("AddToNilSlice", func(t *testing.T) {
		env := &Environment{}
		ps := PropertySource{Name: "ps1", Source: map[string]string{"k": "v"}}

		env.AddPropertySource(ps)

		assert.Equal(t, []PropertySource{ps}, env.PropertySources)
	})

	t.Run("AppendsInOrder", func(t *testing.T) {
		env := &Environment{}
		first := PropertySource{Name: "first", Source: map[string]string{"a": "1"}}
		second := PropertySource{Name: "second", Source: map[string]string{"b": "2"}}

		env.AddPropertySource(first)
		env.AddPropertySource(second)

		assert.Equal(t, []PropertySource{first, second}, env.PropertySources)
	})

	t.Run("KeepsExistingSources", func(t *testing.T) {
		existing := PropertySource{Name: "existing"}
		env := &Environment{PropertySources: []PropertySource{existing}}
		added := PropertySource{Name: "added"}

		env.AddPropertySource(added)

		assert.Equal(t, []PropertySource{existing, added}, env.PropertySources)
	})

	t.Run("AddsSourceWithNilMap", func(t *testing.T) {
		env := &Environment{}

		env.AddPropertySource(PropertySource{Name: "empty"})

		require.Len(t, env.PropertySources, 1)
		assert.Nil(t, env.PropertySources[0].Source)
	})
}
