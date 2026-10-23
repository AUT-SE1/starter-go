// Package contract validates another team's live responses against the shared
// OpenAPI document.
//
// The validator below understands the small slice of JSON Schema that
// openapi.yaml actually uses: $ref, type, required, properties, enum and the
// numeric and string bounds. It is deliberately dependency-free so the starter
// repository needs no module download. If you would rather use a real
// validator, getkin/kin-openapi is the usual Go choice — swap it in and keep
// the tests as they are.
package contract

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"unicode/utf8"
)

// Document is a parsed OpenAPI file.
type Document struct {
	root map[string]any
}

// LoadDocument reads an OpenAPI document from disk. Pass the .json file: the
// standard library does not read YAML.
func LoadDocument(path string) (*Document, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &Document{root: root}, nil
}

// Schema returns one schema from components.schemas by name.
func (d *Document) Schema(name string) (map[string]any, error) {
	components, _ := d.root["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	schema, ok := schemas[name].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema %q is not defined in the document", name)
	}
	return schema, nil
}

// Validate checks one decoded JSON value against a named schema and returns
// every problem it finds, so a failing test names all of them at once.
func (d *Document) Validate(schemaName string, value any) []string {
	schema, err := d.Schema(schemaName)
	if err != nil {
		return []string{err.Error()}
	}
	var problems []string
	d.check(schema, value, schemaName, &problems)
	return problems
}

func (d *Document) resolve(schema map[string]any) map[string]any {
	ref, ok := schema["$ref"].(string)
	if !ok {
		return schema
	}
	// Only local refs of the form #/components/schemas/Name are used here.
	name := ref[strings.LastIndex(ref, "/")+1:]
	resolved, err := d.Schema(name)
	if err != nil {
		return schema
	}
	return resolved
}

func (d *Document) check(schema map[string]any, value any, path string, problems *[]string) {
	schema = d.resolve(schema)

	expectedType, _ := schema["type"].(string)
	if expectedType != "" && !matchesType(expectedType, value) {
		*problems = append(*problems,
			fmt.Sprintf("%s: expected type %s, got %s", path, expectedType, describe(value)))
		return
	}

	if allowed, ok := schema["enum"].([]any); ok && !containsValue(allowed, value) {
		*problems = append(*problems,
			fmt.Sprintf("%s: value %v is not one of %v", path, value, allowed))
	}

	switch expectedType {
	case "object":
		object, _ := value.(map[string]any)
		if required, ok := schema["required"].([]any); ok {
			for _, nameAny := range required {
				name, _ := nameAny.(string)
				if _, present := object[name]; !present {
					*problems = append(*problems,
						fmt.Sprintf("%s: required field %q is missing", path, name))
				}
			}
		}
		properties, _ := schema["properties"].(map[string]any)
		for name, propertyAny := range properties {
			property, ok := propertyAny.(map[string]any)
			if !ok {
				continue
			}
			if fieldValue, present := object[name]; present {
				d.check(property, fieldValue, path+"."+name, problems)
			}
		}
	case "string":
		text, _ := value.(string)
		length := utf8.RuneCountInString(text)
		if minimum, ok := asInt(schema["minLength"]); ok && length < minimum {
			*problems = append(*problems,
				fmt.Sprintf("%s: shorter than minLength %d", path, minimum))
		}
		if maximum, ok := asInt(schema["maxLength"]); ok && length > maximum {
			*problems = append(*problems,
				fmt.Sprintf("%s: longer than maxLength %d", path, maximum))
		}
	case "integer", "number":
		number, _ := value.(float64)
		if minimum, ok := asFloat(schema["minimum"]); ok && number < minimum {
			*problems = append(*problems,
				fmt.Sprintf("%s: %v is below minimum %v", path, number, minimum))
		}
		if maximum, ok := asFloat(schema["maximum"]); ok && number > maximum {
			*problems = append(*problems,
				fmt.Sprintf("%s: %v is above maximum %v", path, number, maximum))
		}
	}
}

func matchesType(expected string, value any) bool {
	switch expected {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "number":
		_, ok := value.(float64)
		return ok
	case "integer":
		// encoding/json decodes every number as float64, so "integer" means
		// "a number with nothing after the decimal point".
		number, ok := value.(float64)
		return ok && number == math.Trunc(number)
	default:
		return true
	}
}

func describe(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64:
		if typed == math.Trunc(typed) {
			return "integer"
		}
		return "number"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func containsValue(allowed []any, value any) bool {
	for _, candidate := range allowed {
		if candidate == value {
			return true
		}
	}
	return false
}

func asInt(value any) (int, bool) {
	number, ok := value.(float64)
	return int(number), ok
}

func asFloat(value any) (float64, bool) {
	number, ok := value.(float64)
	return number, ok
}
