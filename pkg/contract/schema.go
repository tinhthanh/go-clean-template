package contract

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
)

// ErrSchema is a value its schema refuses.
var ErrSchema = errors.New("schema")

// SchemaError is ErrSchema at one place in the value: "$.items[2].phone".
type SchemaError struct {
	Path   string
	Reason string
}

func (e *SchemaError) Error() string { return fmt.Sprintf("%s: %s: %s", ErrSchema, e.Path, e.Reason) }

// Unwrap makes errors.Is(err, ErrSchema) true.
func (e *SchemaError) Unwrap() error { return ErrSchema }

// Field is the top-level property the error is in ("phone" of "$.phone"), or "".
func (e *SchemaError) Field() string {
	rest, ok := strings.CutPrefix(e.Path, "$.")
	if !ok {
		return ""
	}

	if i := strings.IndexAny(rest, ".["); i >= 0 {
		return rest[:i]
	}

	return rest
}

func refuse(at, reason string) error { return &SchemaError{Path: at, Reason: reason} }

const _refPrefix = "#/components/schemas/"

// node is the subset of JSON Schema the contract uses: type, required,
// properties, additionalProperties: false, enum, items, allOf, anyOf,
// minimum, $ref and the date-time and date formats. A contract that needs
// more grows this, with a test.
type node struct {
	Ref                  string                     `json:"$ref"`
	Type                 json.RawMessage            `json:"type"`
	Required             []string                   `json:"required"`
	Properties           map[string]json.RawMessage `json:"properties"`
	AdditionalProperties json.RawMessage            `json:"additionalProperties"`
	Enum                 []any                      `json:"enum"`
	Items                json.RawMessage            `json:"items"`
	AllOf                []json.RawMessage          `json:"allOf"`
	AnyOf                []json.RawMessage          `json:"anyOf"`
	Minimum              *float64                   `json:"minimum"`
	Format               string                     `json:"format"`
}

// Validate checks a decoded JSON value (from json.Unmarshal into any) against a schema of this contract.
func (c *Contract) Validate(schema json.RawMessage, value any) error {
	return c.check(schema, value, "$")
}

// ValidateJSON decodes body and checks it.
func (c *Contract) ValidateJSON(schema json.RawMessage, body []byte) error {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return refuse("$", "is not JSON: "+err.Error())
	}

	return c.Validate(schema, v)
}

// Schema is a named schema of components.schemas.
func (c *Contract) Schema(name string) json.RawMessage { return c.schemas[name] }

func (c *Contract) check(raw json.RawMessage, v any, at string) error {
	var n node
	if err := json.Unmarshal(raw, &n); err != nil {
		return refuse(at, "unreadable schema: "+err.Error())
	}

	if n.Ref != "" {
		target, ok := c.schemas[strings.TrimPrefix(n.Ref, _refPrefix)]
		if !ok {
			return refuse(at, fmt.Sprintf("no schema %s", n.Ref))
		}

		return c.check(target, v, at)
	}

	if err := c.combined(&n, v, at); err != nil {
		return err
	}

	if err := checkType(&n, v, at); err != nil {
		return err
	}

	switch val := v.(type) {
	case map[string]any:
		return c.object(&n, val, at)
	case []any:
		return c.array(&n, val, at)
	default:
		return scalar(&n, v, at)
	}
}

// combined holds a value to the schema's allOf, anyOf and enum.
func (c *Contract) combined(n *node, v any, at string) error {
	for _, part := range n.AllOf {
		if err := c.check(part, v, at); err != nil {
			return err
		}
	}

	if len(n.AnyOf) > 0 && !c.anyOf(n.AnyOf, v, at) {
		return refuse(at, "matches none of anyOf")
	}

	if len(n.Enum) > 0 && !slices.ContainsFunc(n.Enum, func(e any) bool { return e == v }) {
		return refuse(at, fmt.Sprintf("%v is not one of %v", v, n.Enum))
	}

	return nil
}

func (c *Contract) anyOf(parts []json.RawMessage, v any, at string) bool {
	for _, part := range parts {
		if c.check(part, v, at) == nil {
			return true
		}
	}

	return false
}

func (c *Contract) object(n *node, v map[string]any, at string) error {
	for _, name := range n.Required {
		if _, ok := v[name]; !ok {
			return refuse(at+"."+name, "is required")
		}
	}

	closed := strings.TrimSpace(string(n.AdditionalProperties)) == "false"

	for name, value := range v {
		prop, ok := n.Properties[name]
		if !ok {
			if closed {
				return refuse(at+"."+name, "is not a property")
			}

			continue
		}

		if err := c.check(prop, value, at+"."+name); err != nil {
			return err
		}
	}

	return nil
}

func (c *Contract) array(n *node, v []any, at string) error {
	if n.Items == nil {
		return nil
	}

	for i, item := range v {
		if err := c.check(n.Items, item, fmt.Sprintf("%s[%d]", at, i)); err != nil {
			return err
		}
	}

	return nil
}

func scalar(n *node, v any, at string) error {
	if f, ok := v.(float64); ok && n.Minimum != nil && f < *n.Minimum {
		return refuse(at, fmt.Sprintf("%v is under the minimum %v", f, *n.Minimum))
	}

	s, ok := v.(string)
	if !ok {
		return nil
	}

	layout := map[string]string{"date-time": time.RFC3339, "date": time.DateOnly}[n.Format]
	if layout == "" {
		return nil
	}

	if _, err := time.Parse(layout, s); err != nil {
		return refuse(at, fmt.Sprintf("%q is not a %s", s, n.Format))
	}

	return nil
}

// checkType holds a value to the schema's type, one or a list of them.
func checkType(n *node, v any, at string) error {
	if n.Type == nil {
		return nil
	}

	var types Words
	if err := json.Unmarshal(n.Type, &types); err != nil {
		return refuse(at, "unreadable type: "+err.Error())
	}

	for _, t := range types {
		if isType(t, v) {
			return nil
		}
	}

	return refuse(at, fmt.Sprintf("%v is not %s", v, strings.Join(types, " or ")))
}

func isType(t string, v any) bool {
	switch t {
	case "object":
		_, ok := v.(map[string]any)

		return ok
	case "array":
		_, ok := v.([]any)

		return ok
	case "string":
		_, ok := v.(string)

		return ok
	case "boolean":
		_, ok := v.(bool)

		return ok
	case "number":
		_, ok := v.(float64)

		return ok
	case "integer":
		f, ok := v.(float64)

		return ok && f == math.Trunc(f)
	case "null":
		return v == nil
	default:
		return false
	}
}
