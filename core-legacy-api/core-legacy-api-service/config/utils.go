package config

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/netcracker/qubership-mini-core/core-legacy-api/core-legacy-api-service/model"

	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
	"gopkg.in/yaml.v3"
)

func splitAppNameAndProfile(appNameWithProfile string) (string, string) {
	if !strings.Contains(appNameWithProfile, ",") {
		return appNameWithProfile, defaultProfileName
	}
	appAndProfileList := strings.Split(appNameWithProfile, ",")

	return appAndProfileList[0], appAndProfileList[1]
}

func toConfigProperties(properties []*api.KVPair, prefix string) []model.ConfigProperty {
	if properties == nil {
		return []model.ConfigProperty{}
	}

	result := make([]model.ConfigProperty, 0, len(properties))

	for _, prop := range properties {
		// Skip values whose key is exactly the prefix.
		if prop.Key == prefix {
			continue
		}

		key, _ := cutPrefix(prop.Key, prefix)
		key = strings.ReplaceAll(key, "/", ".")
		key = unescape(key)

		value := ""
		if prop.Value != nil {
			value = string(prop.Value)
		}

		result = append(result, model.ConfigProperty{
			Key:   key,
			Value: value,
		})
	}

	return result
}

var replacements = [][]string{
	{"/", "&sol;"},
	{"*", "&ast;"},
	{"?", "&quest;"},
	{"'", "&apos;"},
	{"%", "&percnt;"},
}

func unescape(s string) string {
	for _, replacement := range replacements {
		s = strings.ReplaceAll(s, replacement[1], replacement[0])
	}
	return s
}

func escape(s string) string {
	for _, replacement := range replacements {
		s = strings.ReplaceAll(s, replacement[0], replacement[1])
	}
	return s
}

func cutPrefix(s string, prefix string) (string, error) {
	if s == prefix {
		return "", fmt.Errorf("property key cannot be equal to prefix %s", prefix)
	}
	prefixSize := len(prefix)
	if !strings.HasSuffix(prefix, "/") {
		prefixSize++
	}
	return s[prefixSize:], nil
}

func formatKeyPrefix(namespace string, appName string, profileName string) string {
	if appName == configPropertiesGlobalApplicationName {
		appName = consulGlobalApplicationName
	}
	result := consulConfigPrefix + "/" + namespace + "/" + appName
	if profileName != defaultProfileName {
		result += "," + profileName
	}
	return result + "/"
}

func calculateOperationSize(key, value string) int {
	keySize := len([]byte(key))
	valueSize := len([]byte(value))

	// The value is encoded in Base64, which increases the size by roughly 4/3.
	// Add 100 bytes for the JSON structure/syntax.
	return keySize + ((valueSize / 3) * 4) + 100
}

func groupProfilesByApplication(profiles []model.ConfigProfile) map[string][]string {
	applications := make(map[string][]string)

	for _, profile := range profiles {
		applications[profile.Application] = append(
			applications[profile.Application],
			profile.Profile,
		)
	}

	return applications
}

func toApplicationResponses(applications map[string][]string) []model.ApplicationWithProfiles {
	result := make([]model.ApplicationWithProfiles, 0, len(applications))

	for application, profiles := range applications {
		result = append(result, model.ApplicationWithProfiles{
			Name:     application,
			Profiles: profiles,
		})
	}

	return result
}

func optional[T comparable](value T, empty T) *T {
	if value == empty {
		return nil
	}
	return &value
}

func mergeConfigProperties(
	dst map[string]model.ConfigProperty,
	src map[string]model.ConfigProperty,
) {
	for key, property := range src {
		dst[key] = property
	}
}

func buildEnvironment(
	application string,
	profiles []string,
	label *string,
	properties map[string]model.ConfigProperty,
) model.Environment {
	propertyMap := make(map[string]string, len(properties))

	for _, property := range properties {
		propertyMap[property.Key] = property.Value
	}

	return model.Environment{
		Name:     application,
		Profiles: profiles,
		Label:    label,
		Version:  strconv.Itoa(0),
		PropertySources: []model.PropertySource{
			{
				Name:   consulPropertiesSource,
				Source: propertyMap,
			},
		},
	}
}

// buildNestedProperties converts properties
// into a nested map suitable for JSON serialization.
//
// If a key conflicts with an already-set value (e.g. "message" is a plain
// value but "message.text" tries to nest under it, or vice versa), the
// conflicting key is preserved as-is (flat, with dots) instead of
// overwriting existing data.
func buildNestedProperties(properties map[string]model.ConfigProperty) map[string]interface{} {
	result := make(map[string]interface{})

	//Need to sort for correct conflict resolution
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		property := properties[key]
		addNestedProperty(result, property.Key, property.Value)
	}
	return result
}

// addNestedProperty adds a property as a nested map/list structure when
// the key has a valid path. Otherwise, it keeps the key exactly as provided.
//
// Example:
//
//	"server.port" -> {"server": {"port": "8080"}}
//	"items[0].name" -> {"items": [{"name": "..."}]}
//	"invalid..key" -> {"invalid..key": "..."}
func addNestedProperty(
	target map[string]interface{},
	key string,
	value string,
) {
	pathTokens, ok := parsePropertyPath(key)
	if !ok {
		target[key] = value
		return
	}

	if _, ok := setNestedValue(target, pathTokens, value); !ok {
		target[key] = value
	}
}

const maxListIndex = 10000 // guard against list[999999999] allocating huge slices

var (
	// Matches "app", "app[0]", "app[0][1]", etc.
	partRe = regexp.MustCompile(`^([^\[\]]*)((?:\[\d+])*)$`)

	// Matches "[0]", "[1]", "[123]", etc.
	idxRe = regexp.MustCompile(`\[(\d+)]`)
)

type pathToken struct {
	name    string // name is the map key. Only meaningful when isIndex is false.
	index   int    // index is the list position. Only meaningful when isIndex is true.
	isIndex bool   // isIndex is true for a list step like [0], false for a map key step.
}

// parsePropertyPath parses a key like "a.b[0].c[1][2]"
// into path tokens: a, b, [0], c, [1], [2].
//
// Invalid paths return ok=false and should be kept as flat keys.
func parsePropertyPath(key string) (tokens []pathToken, ok bool) {
	for _, segment := range strings.Split(key, ".") {
		matches := partRe.FindStringSubmatch(segment)
		if matches == nil {
			return nil, false
		}

		propertyName := matches[1]
		indexPart := matches[2]

		if propertyName != "" {
			tokens = append(tokens, pathToken{name: propertyName})
		} else if indexPart == "" {
			// Example: "a..b" contains an empty path segment.
			return nil, false
		}

		for _, indexMatch := range idxRe.FindAllStringSubmatch(indexPart, -1) {
			index, err := strconv.Atoi(indexMatch[1])
			if err != nil || index > maxListIndex {
				return nil, false
			}

			tokens = append(tokens, pathToken{
				index:   index,
				isIndex: true,
			})
		}
	}

	if len(tokens) == 0 || tokens[0].isIndex {
		// The root must always be a map, so a key cannot start with [0].
		return nil, false
	}

	return tokens, true
}

// renderPropertyPath converts path tokens back into a key string.
//
// Example:
//
//	[a, b, [0], c] -> "a.b[0].c"
//
// This is used when a nested structure cannot be created because of
// a type conflict. In that case, the remaining path is stored as a flat key.
func renderPropertyPath(tokens []pathToken) string {
	var builder strings.Builder

	for i, token := range tokens {
		if token.isIndex {
			builder.WriteString("[" + strconv.Itoa(token.index) + "]")
			continue
		}

		if i > 0 {
			builder.WriteByte('.')
		}
		builder.WriteString(token.name)
	}

	return builder.String()
}

// setNestedValue creates the structure described by pathTokens and stores value.
//
// For example:
//
//	"a.b[0]" = "hello"
//
// becomes:
//
//	map[string]interface{}{
//	    "a": map[string]interface{}{
//	        "b": []interface{}{"hello"},
//	    },
//	}
//
// The operation fails when an existing value has the wrong type.
func setNestedValue(
	currentValue interface{},
	pathTokens []pathToken,
	value string,
) (interface{}, bool) {
	currentToken := pathTokens[0]

	if currentToken.isIndex {
		return setListValue(currentValue, pathTokens, value)
	}

	return setMapValue(currentValue, pathTokens, value)
}

// setListValue adds a value to a list at the index described by pathTokens.
func setListValue(
	currentValue interface{},
	pathTokens []pathToken,
	value string,
) (interface{}, bool) {
	var list []interface{}

	if currentValue != nil {
		var ok bool
		list, ok = currentValue.([]interface{})
		if !ok {
			// Example: existing value is "hello", but the key requires [0].
			return currentValue, false
		}
	}

	currentToken := pathTokens[0]

	// Grow the list so the requested index exists.
	for len(list) <= currentToken.index {
		list = append(list, nil)
	}

	if len(pathTokens) == 1 {
		list[currentToken.index] = value
		return list, true
	}

	childValue, ok := setNestedValue(
		list[currentToken.index],
		pathTokens[1:],
		value,
	)
	if !ok {
		return currentValue, false
	}

	list[currentToken.index] = childValue
	return list, true
}

// setMapValue adds a value to a map using the property name from pathTokens.
func setMapValue(
	currentValue interface{},
	pathTokens []pathToken,
	value string,
) (interface{}, bool) {
	var properties map[string]interface{}

	if currentValue == nil {
		properties = make(map[string]interface{})
	} else {
		var ok bool
		properties, ok = currentValue.(map[string]interface{})
		if !ok {
			// Example: "a" already contains "hello",
			// but we now need "a.b".
			return currentValue, false
		}
	}

	currentToken := pathTokens[0]

	if len(pathTokens) == 1 {
		properties[currentToken.name] = value
		return properties, true
	}

	childValue, ok := setNestedValue(
		properties[currentToken.name],
		pathTokens[1:],
		value,
	)
	if !ok {
		// The existing value prevents us from creating the nested structure.
		// Keep the original value and store the new property as a flat key.
		properties[renderPropertyPath(pathTokens)] = value
		return properties, true
	}

	properties[currentToken.name] = childValue
	return properties, true
}

func resolveProperties(properties map[string]model.ConfigProperty) error {
	for key, property := range properties {
		var err error
		property.Value, err = resolvePropertyValue(property.Value, properties, nil)
		if err != nil {
			return err
		}
		properties[key] = property
	}
	return nil
}

// Compile once; matches ${key} and ${key:default}.
var placeholderRe = regexp.MustCompile(`\$\{([^}:]+)(?::([^}]*))?}`)

func resolvePropertyValue(value string, properties map[string]model.ConfigProperty, resolving map[string]bool) (string, error) {
	if resolving == nil {
		// Track properties being resolved to detect circular references.
		resolving = make(map[string]bool)
	}

	matches := placeholderRe.FindAllStringSubmatchIndex(value, -1)
	if matches == nil {
		return value, nil
	}

	var sb strings.Builder
	last := 0

	for _, m := range matches {
		// Copy the text before this placeholder.
		sb.WriteString(value[last:m[0]])
		last = m[1]

		key := value[m[2]:m[3]]
		hasDefault := m[4] != -1

		prop, exists := properties[key]

		var raw string
		switch {
		case exists:
			if resolving[key] {
				return "", fmt.Errorf("circular property reference detected: %s", key)
			}
			raw = prop.Value
		case hasDefault:
			raw = value[m[4]:m[5]]
		default:
			// Unresolved and no default: keep the placeholder as-is and move on.
			sb.WriteString(value[m[0]:m[1]])
			continue
		}

		// Only properties (not inline defaults) can participate in a cycle.
		if exists {
			resolving[key] = true
		}

		resolved, err := resolvePropertyValue(raw, properties, resolving)

		if exists {
			// Allow the property to be resolved again elsewhere.
			delete(resolving, key)
		}
		if err != nil {
			return "", err
		}

		sb.WriteString(resolved)
	}

	sb.WriteString(value[last:])
	return sb.String(), nil
}

func buildPropertiesText(properties map[string]model.ConfigProperty) string {

	var builder strings.Builder

	for _, property := range properties {
		builder.WriteString(property.Key)
		builder.WriteString(": ")
		builder.WriteString(property.Value)
		builder.WriteString("\n")
	}

	return builder.String()
}

func groupPropertiesByApplication(pairs []*api.KVPair) map[string][]*api.KVPair {

	propertiesByApp := make(map[string][]*api.KVPair)

	for _, pair := range pairs {
		parts := strings.Split(pair.Key, "/")
		if len(parts) < 3 {
			continue
		}

		app := parts[2]
		propertiesByApp[app] = append(propertiesByApp[app], pair)
	}

	return propertiesByApp
}

func buildConfigProfiles(
	propertiesByApp map[string][]*api.KVPair,
	prefix string,
) []model.ConfigProfile {

	profiles := make([]model.ConfigProfile, 0, len(propertiesByApp))

	for app := range propertiesByApp {
		appName, profile := splitAppNameAndProfile(app)

		if strings.EqualFold(appName, consulGlobalApplicationName) {
			appName = configPropertiesGlobalApplicationName
		}

		profiles = append(profiles, model.ConfigProfile{
			ID:          uuid.Nil,
			Application: appName,
			Profile:     profile,
			Version:     0,
			Properties: toConfigProperties(
				propertiesByApp[app],
				prefix+app,
			),
		})
	}

	return profiles
}

func marshalWithSingleQuotes(v interface{}) ([]byte, error) {
	// First marshal normally to get a Node tree
	var node yaml.Node
	tmp, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(tmp, &node); err != nil {
		return nil, err
	}
	// Walk the tree and convert double-quoted scalars to single-quoted
	forceSingleQuotes(&node)

	return yaml.Marshal(&node)
}

func forceSingleQuotes(node *yaml.Node) {
	if node.Kind == yaml.ScalarNode && node.Style == yaml.DoubleQuotedStyle {
		node.Style = yaml.SingleQuotedStyle
	}
	for _, child := range node.Content {
		forceSingleQuotes(child)
	}
}

func GetFiberParam(fiberCtx *fiber.Ctx, paramName string) string {
	paramValue := fiberCtx.Params(paramName)
	unescapedStr, err := url.PathUnescape(paramValue)
	if err != nil {
		return utils.CopyString(paramValue)
	}
	return utils.CopyString(unescapedStr)
}

func splitApplicationAndProfiles(nameWithProfiles string) (string, string, error) {
	idx := strings.LastIndex(nameWithProfiles, "-")

	if idx <= 0 || idx == len(nameWithProfiles)-1 {
		return "", "", fmt.Errorf("invalid application-profile value: %q", nameWithProfiles)
	}

	application := nameWithProfiles[:idx]
	profiles := nameWithProfiles[idx+1:]

	return application, profiles, nil
}
