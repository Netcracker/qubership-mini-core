package config

import (
	"strings"
	"testing"

	"github.com/Netcracker/qubership-mini-core/core-legacy-api/core-legacy-api-service/model"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/assert"
)

func TestSplitAppNameAndProfile(t *testing.T) {
	n1 := "global,default"
	n2 := "application,dev"
	n3 := "global"

	name1, profile1 := splitAppNameAndProfile(n1)
	name2, profile2 := splitAppNameAndProfile(n2)
	name3, profile3 := splitAppNameAndProfile(n3)

	assert.Equal(t, name1, "global")
	assert.Equal(t, profile1, "default")
	assert.Equal(t, name2, "application")
	assert.Equal(t, profile2, "dev")
	assert.Equal(t, name3, "global")
	assert.Equal(t, profile3, defaultProfileName)
}

func TestToConfigProperties_NilInput(t *testing.T) {
	result := toConfigProperties(nil, "config/app/default/")

	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestToConfigProperties_EmptyInput(t *testing.T) {
	result := toConfigProperties([]*api.KVPair{}, "config/app/default/")

	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestToConfigProperties_SkipsPrefixKey(t *testing.T) {
	properties := []*api.KVPair{
		{
			Key:   "config/app/default/",
			Value: []byte("ignored"),
		},
	}

	result := toConfigProperties(properties, "config/app/default/")

	assert.Empty(t, result)
}

func TestToConfigProperties_ConvertsProperties(t *testing.T) {
	properties := []*api.KVPair{
		{
			Key:   "config/app/default/database/url",
			Value: []byte("jdbc:postgresql://localhost"),
		},
		{
			Key:   "config/app/default/server/port",
			Value: []byte("8080"),
		},
	}

	result := toConfigProperties(properties, "config/app/default/")

	expected := []model.ConfigProperty{
		{
			Key:       "database.url",
			Value:     "jdbc:postgresql://localhost",
			Encrypted: nil,
		},
		{
			Key:       "server.port",
			Value:     "8080",
			Encrypted: nil,
		},
	}

	assert.Equal(t, expected, result)
}

func TestEscape(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "NoSpecialCharacters",
			input:    "hello.world",
			expected: "hello.world",
		},
		{
			name:     "EscapesSlash",
			input:    "foo/bar",
			expected: "foo&sol;bar",
		},
		{
			name:     "EscapesAsterisk",
			input:    "foo*bar",
			expected: "foo&ast;bar",
		},
		{
			name:     "EscapesQuestionMark",
			input:    "foo?bar",
			expected: "foo&quest;bar",
		},
		{
			name:     "EscapesApostrophe",
			input:    "foo'bar",
			expected: "foo&apos;bar",
		},
		{
			name:     "EscapesPercent",
			input:    "100%",
			expected: "100&percnt;",
		},
		{
			name:     "EscapesMultipleCharacters",
			input:    "foo/bar*baz?'100%",
			expected: "foo&sol;bar&ast;baz&quest;&apos;100&percnt;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, escape(tt.input))
		})
	}
}

func TestUnescape(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "NoEscapedCharacters",
			input:    "hello.world",
			expected: "hello.world",
		},
		{
			name:     "UnescapesSlash",
			input:    "foo&sol;bar",
			expected: "foo/bar",
		},
		{
			name:     "UnescapesAsterisk",
			input:    "foo&ast;bar",
			expected: "foo*bar",
		},
		{
			name:     "UnescapesQuestionMark",
			input:    "foo&quest;bar",
			expected: "foo?bar",
		},
		{
			name:     "UnescapesApostrophe",
			input:    "foo&apos;bar",
			expected: "foo'bar",
		},
		{
			name:     "UnescapesPercent",
			input:    "100&percnt;",
			expected: "100%",
		},
		{
			name:     "UnescapesMultipleCharacters",
			input:    "foo&sol;bar&ast;baz&quest;&apos;100&percnt;",
			expected: "foo/bar*baz?'100%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, unescape(tt.input))
		})
	}
}

func TestEscapeAndUnescape_RoundTrip(t *testing.T) {
	tests := []string{
		"",
		"plain text",
		"foo/bar",
		"foo*bar",
		"foo?bar",
		"foo'bar",
		"100%",
		"foo/bar*baz?'100%",
		"/**?''%%",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			assert.Equal(t, input, unescape(escape(input)))
		})
	}
}

func TestCutPrefix(t *testing.T) {
	tests := []struct {
		name      string
		s         string
		prefix    string
		expected  string
		expectErr bool
	}{
		{
			name:      "BasicPrefixWithSlash",
			s:         "config/app/default/database/url",
			prefix:    "config/app/default/",
			expected:  "database/url",
			expectErr: false,
		},
		{
			name:      "PrefixWithoutTrailingSlash",
			s:         "config/app/default/database/url",
			prefix:    "config/app/default",
			expected:  "database/url",
			expectErr: false,
		},
		{
			name:      "MatchesPrefix",
			s:         "config/app/default/",
			prefix:    "config/app/default/",
			expected:  "",
			expectErr: true,
		},
		{
			name:      "SingleLevel",
			s:         "config/database",
			prefix:    "config/",
			expected:  "database",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := cutPrefix(tt.s, tt.prefix)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestCalculateOperationSize(t *testing.T) {
	tests := []struct {
		name            string
		key             string
		value           string
		expectedMinSize int
		expectedMaxSize int
	}{
		{
			name:            "EmptyKeyAndValue",
			key:             "",
			value:           "",
			expectedMinSize: 100,
			expectedMaxSize: 101,
		},
		{
			name:            "SimpleKeyAndValue",
			key:             "app.name",
			value:           "myapp",
			expectedMinSize: 100,
			expectedMaxSize: 200,
		},
		{
			name:            "LargeValue",
			key:             "config.data",
			value:           "x" + strings.Repeat("y", 1000),
			expectedMinSize: 1200,
			expectedMaxSize: 1500,
		},
		{
			name:            "UnicodeKey",
			key:             "app.名前",
			value:           "test",
			expectedMinSize: 100,
			expectedMaxSize: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := calculateOperationSize(tt.key, tt.value)
			assert.GreaterOrEqual(t, size, tt.expectedMinSize)
			assert.LessOrEqual(t, size, tt.expectedMaxSize)
		})
	}
}

func TestGroupProfilesByApplication(t *testing.T) {
	tests := []struct {
		name     string
		profiles []model.ConfigProfile
		expected map[string][]string
	}{
		{
			name:     "EmptyProfiles",
			profiles: []model.ConfigProfile{},
			expected: map[string][]string{},
		},
		{
			name: "SingleApplicationSingleProfile",
			profiles: []model.ConfigProfile{
				{
					Application: "app1",
					Profile:     "dev",
				},
			},
			expected: map[string][]string{
				"app1": {"dev"},
			},
		},
		{
			name: "SingleApplicationMultipleProfiles",
			profiles: []model.ConfigProfile{
				{
					Application: "app1",
					Profile:     "dev",
				},
				{
					Application: "app1",
					Profile:     "prod",
				},
				{
					Application: "app1",
					Profile:     "staging",
				},
			},
			expected: map[string][]string{
				"app1": {"dev", "prod", "staging"},
			},
		},
		{
			name: "MultipleApplicationsMultipleProfiles",
			profiles: []model.ConfigProfile{
				{
					Application: "app1",
					Profile:     "dev",
				},
				{
					Application: "app2",
					Profile:     "dev",
				},
				{
					Application: "app1",
					Profile:     "prod",
				},
				{
					Application: "app3",
					Profile:     "staging",
				},
			},
			expected: map[string][]string{
				"app1": {"dev", "prod"},
				"app2": {"dev"},
				"app3": {"staging"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := groupProfilesByApplication(tt.profiles)
			assert.Equal(t, len(tt.expected), len(result))
			for app, expectedProfiles := range tt.expected {
				assert.ElementsMatch(t, expectedProfiles, result[app])
			}
		})
	}
}

func TestToApplicationResponses(t *testing.T) {
	tests := []struct {
		name         string
		applications map[string][]string
		expectedLen  int
	}{
		{
			name:         "EmptyApplications",
			applications: map[string][]string{},
			expectedLen:  0,
		},
		{
			name: "SingleApplicationSingleProfile",
			applications: map[string][]string{
				"app1": {"dev"},
			},
			expectedLen: 1,
		},
		{
			name: "MultipleApplicationsMultipleProfiles",
			applications: map[string][]string{
				"app1": {"dev", "prod"},
				"app2": {"staging"},
				"app3": {"dev", "qa", "prod"},
			},
			expectedLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toApplicationResponses(tt.applications)
			assert.Equal(t, tt.expectedLen, len(result))

			// Verify structure
			for _, response := range result {
				assert.NotEmpty(t, response.Name)
				assert.NotNil(t, response.Profiles)
				assert.Equal(t, len(tt.applications[response.Name]), len(response.Profiles))
				assert.ElementsMatch(t, tt.applications[response.Name], response.Profiles)
			}
		})
	}
}

func TestOptionalString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *string
	}{
		{
			name:     "EmptyString",
			input:    "",
			expected: nil,
		},
		{
			name:     "NonEmptyString",
			input:    "value",
			expected: func() *string { s := "value"; return &s }(),
		},
		{
			name:     "StringWithSpaces",
			input:    "  ",
			expected: func() *string { s := "  "; return &s }(),
		},
		{
			name:     "StringWithSpecialChars",
			input:    "special!@#$%",
			expected: func() *string { s := "special!@#$%"; return &s }(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := optional(tt.input, "")
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, *tt.expected, *result)
			}
		})
	}
}

func TestMergeConfigProperties(t *testing.T) {
	tests := []struct {
		name     string
		dst      map[string]model.ConfigProperty
		src      map[string]model.ConfigProperty
		expected map[string]model.ConfigProperty
	}{
		{
			name: "MergeIntoEmptyMap",
			dst:  map[string]model.ConfigProperty{},
			src: map[string]model.ConfigProperty{
				"key1": {
					Key:   "key1",
					Value: "value1",
				},
			},
			expected: map[string]model.ConfigProperty{
				"key1": {
					Key:   "key1",
					Value: "value1",
				},
			},
		},
		{
			name: "MergeEmptySourceMap",
			dst: map[string]model.ConfigProperty{
				"key1": {
					Key:   "key1",
					Value: "value1",
				},
			},
			src: map[string]model.ConfigProperty{},
			expected: map[string]model.ConfigProperty{
				"key1": {
					Key:   "key1",
					Value: "value1",
				},
			},
		},
		{
			name: "OverwriteExistingKey",
			dst: map[string]model.ConfigProperty{
				"key1": {
					Key:   "key1",
					Value: "old_value",
				},
			},
			src: map[string]model.ConfigProperty{
				"key1": {
					Key:   "key1",
					Value: "new_value",
				},
			},
			expected: map[string]model.ConfigProperty{
				"key1": {
					Key:   "key1",
					Value: "new_value",
				},
			},
		},
		{
			name: "MergeMultipleProperties",
			dst: map[string]model.ConfigProperty{
				"key1": {
					Key:   "key1",
					Value: "value1",
				},
			},
			src: map[string]model.ConfigProperty{
				"key2": {
					Key:   "key2",
					Value: "value2",
				},
				"key3": {
					Key:   "key3",
					Value: "value3",
				},
			},
			expected: map[string]model.ConfigProperty{
				"key1": {
					Key:   "key1",
					Value: "value1",
				},
				"key2": {
					Key:   "key2",
					Value: "value2",
				},
				"key3": {
					Key:   "key3",
					Value: "value3",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mergeConfigProperties(tt.dst, tt.src)
			assert.Equal(t, tt.expected, tt.dst)
		})
	}
}

func TestBuildEnvironment(t *testing.T) {
	tests := []struct {
		name       string
		app        string
		profiles   []string
		label      *string
		properties map[string]model.ConfigProperty
	}{
		{
			name:       "SimpleEnvironment",
			app:        "myapp",
			profiles:   []string{"dev"},
			label:      nil,
			properties: map[string]model.ConfigProperty{},
		},
		{
			name:     "EnvironmentWithLabel",
			app:      "myapp",
			profiles: []string{"prod", "api"},
			label: func() *string {
				s := "v1.0"
				return &s
			}(),
			properties: map[string]model.ConfigProperty{
				"db.host": {
					Key:   "db.host",
					Value: "localhost",
				},
			},
		},
		{
			name:     "EnvironmentWithMultipleProperties",
			app:      "testapp",
			profiles: []string{"test"},
			label:    nil,
			properties: map[string]model.ConfigProperty{
				"app.name": {
					Key:   "app.name",
					Value: "testapp",
				},
				"app.port": {
					Key:   "app.port",
					Value: "8080",
				},
				"app.debug": {
					Key:   "app.debug",
					Value: "false",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := buildEnvironment(tt.app, tt.profiles, tt.label, tt.properties)

			assert.Equal(t, tt.app, env.Name)
			assert.Equal(t, tt.profiles, env.Profiles)
			assert.Equal(t, tt.label, env.Label)
			assert.NotNil(t, env.PropertySources)
			assert.GreaterOrEqual(t, len(env.PropertySources), 1)
			assert.Equal(t, "consul", env.PropertySources[0].Name)
			assert.Equal(t, len(tt.properties), len(env.PropertySources[0].Source))
		})
	}
}

func TestBuildNestedProperties(t *testing.T) {
	tests := []struct {
		name       string
		properties map[string]model.ConfigProperty
		validate   func(t *testing.T, result map[string]interface{})
	}{
		{
			name:       "EmptyProperties",
			properties: map[string]model.ConfigProperty{},
			validate: func(t *testing.T, result map[string]interface{}) {
				assert.Empty(t, result)
			},
		},
		{
			name: "FlatProperties",
			properties: map[string]model.ConfigProperty{
				"app.name": {
					Key:   "app.name",
					Value: "myapp",
				},
				"app.version": {
					Key:   "app.version",
					Value: "1.0",
				},
			},
			validate: func(t *testing.T, result map[string]interface{}) {
				assert.NotNil(t, result["app"])
				appMap, ok := result["app"].(map[string]interface{})
				assert.True(t, ok)
				assert.Equal(t, "myapp", appMap["name"])
				assert.Equal(t, "1.0", appMap["version"])
			},
		},
		{
			name: "DeeplyNestedProperties",
			properties: map[string]model.ConfigProperty{
				"database.connection.host": {
					Key:   "database.connection.host",
					Value: "localhost",
				},
				"database.connection.port": {
					Key:   "database.connection.port",
					Value: "5432",
				},
			},
			validate: func(t *testing.T, result map[string]interface{}) {
				db, ok := result["database"].(map[string]interface{})
				assert.True(t, ok)
				conn, ok := db["connection"].(map[string]interface{})
				assert.True(t, ok)
				assert.Equal(t, "localhost", conn["host"])
				assert.Equal(t, "5432", conn["port"])
			},
		},
		{
			name: "MixedFlatAndNestedProperties",
			properties: map[string]model.ConfigProperty{
				"app": {
					Key:   "app",
					Value: "myapp",
				},
				"app.version": {
					Key:   "app.version",
					Value: "1.0",
				},
			},
			validate: func(t *testing.T, result map[string]interface{}) {
				// First property "app" should be preserved as leaf value
				appValue, ok := result["app"]
				assert.True(t, ok)
				// The second property "app.version" should be stored as flat key due to conflict
				if _, isString := appValue.(string); isString {
					assert.Equal(t, "myapp", appValue)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildNestedProperties(tt.properties)
			tt.validate(t, result)
		})
	}
}

func TestResolvePropertyValue(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		properties map[string]model.ConfigProperty
		expected   string
	}{
		{
			name:       "NoPlaceholder",
			value:      "plain_value",
			properties: map[string]model.ConfigProperty{},
			expected:   "plain_value",
		},
		{
			name:  "SimplePlaceholder",
			value: "${db.host}",
			properties: map[string]model.ConfigProperty{
				"db.host": {
					Key:   "db.host",
					Value: "localhost",
				},
			},
			expected: "localhost",
		},
		{
			name:  "PlaceholderInMiddle",
			value: "jdbc:postgresql://${db.host}:5432",
			properties: map[string]model.ConfigProperty{
				"db.host": {
					Key:   "db.host",
					Value: "postgres-server",
				},
			},
			expected: "jdbc:postgresql://postgres-server:5432",
		},
		{
			name:       "PlaceholderWithDefaultValue",
			value:      "${missing.key:default_value}",
			properties: map[string]model.ConfigProperty{},
			expected:   "default_value",
		},
		{
			name:  "MultiplePlaceholders",
			value: "${user}:${password}",
			properties: map[string]model.ConfigProperty{
				"user": {
					Key:   "user",
					Value: "admin",
				},
				"password": {
					Key:   "password",
					Value: "secret",
				},
			},
			expected: "admin:secret",
		},
		{
			name:       "MissingPlaceholder",
			value:      "${missing.key}",
			properties: map[string]model.ConfigProperty{},
			expected:   "${missing.key}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := resolvePropertyValue(tt.value, tt.properties, nil)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildPropertiesText(t *testing.T) {
	tests := []struct {
		name       string
		properties map[string]model.ConfigProperty
		validate   func(t *testing.T, result string)
	}{
		{
			name:       "EmptyProperties",
			properties: map[string]model.ConfigProperty{},
			validate: func(t *testing.T, result string) {
				assert.Equal(t, "", result)
			},
		},
		{
			name: "SingleProperty",
			properties: map[string]model.ConfigProperty{
				"app.name": {
					Key:   "app.name",
					Value: "myapp",
				},
			},
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "app.name: myapp")
				assert.Contains(t, result, "\n")
			},
		},
		{
			name: "MultipleProperties",
			properties: map[string]model.ConfigProperty{
				"app.name": {
					Key:   "app.name",
					Value: "myapp",
				},
				"app.version": {
					Key:   "app.version",
					Value: "1.0.0",
				},
				"app.port": {
					Key:   "app.port",
					Value: "8080",
				},
			},
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "app.name: myapp")
				assert.Contains(t, result, "app.version: 1.0.0")
				assert.Contains(t, result, "app.port: 8080")
				lines := strings.Split(result, "\n")
				assert.GreaterOrEqual(t, len(lines), 3)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildPropertiesText(tt.properties)
			tt.validate(t, result)
		})
	}
}

func TestGroupPropertiesByApplication(t *testing.T) {
	tests := []struct {
		name     string
		pairs    []*api.KVPair
		validate func(t *testing.T, result map[string][]*api.KVPair)
	}{
		{
			name:  "EmptyPairs",
			pairs: []*api.KVPair{},
			validate: func(t *testing.T, result map[string][]*api.KVPair) {
				assert.Empty(t, result)
			},
		},
		{
			name: "ValidPairs",
			pairs: []*api.KVPair{
				{
					Key:   "config/default/app1/property1",
					Value: []byte("value1"),
				},
				{
					Key:   "config/default/app1/property2",
					Value: []byte("value2"),
				},
				{
					Key:   "config/default/app2/property1",
					Value: []byte("value1"),
				},
			},
			validate: func(t *testing.T, result map[string][]*api.KVPair) {
				assert.Equal(t, 2, len(result))
				assert.Equal(t, 2, len(result["app1"]))
				assert.Equal(t, 1, len(result["app2"]))
			},
		},
		{
			name: "InvalidPairsSkipped",
			pairs: []*api.KVPair{
				{
					Key:   "config",
					Value: []byte("value"),
				},
				{
					Key:   "config/default",
					Value: []byte("value"),
				},
				{
					Key:   "config/default/app1/property1",
					Value: []byte("value1"),
				},
			},
			validate: func(t *testing.T, result map[string][]*api.KVPair) {
				assert.Equal(t, 1, len(result))
				assert.Equal(t, 1, len(result["app1"]))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := groupPropertiesByApplication(tt.pairs)
			tt.validate(t, result)
		})
	}
}
