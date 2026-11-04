// Package contract reads an OpenAPI 3.1 file (api/openapi.json, the client's
// own contract) and answers what each operation takes and does: its method,
// path and operationId, and the rules the contract writes beside it —
// x-permission, x-branch-scoped, x-masks and x-audit. The REST layer builds
// its routes and guards from it, so the server copies the contract rather
// than retyping it, and the tests hold answers to its schemas (schema.go).
package contract

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// The x-permission words that are not an action key.
const (
	Public   = "public"
	SignedIn = "signed-in"
)

// ErrBadContract is a contract this package cannot read.
var ErrBadContract = errors.New("contract: unreadable")

// Mask is one x-masks entry: the field at Path is hidden from a reader who
// has none of Unless. As is "masked" (asterisks), "absent" or "equals"
// (Value in its place); Where narrows the objects it applies to by their
// fields, Fields the changes[] lines by their field.
type Mask struct {
	Path   string            `json:"path"`
	Unless Words             `json:"unless"`
	As     string            `json:"as"`
	Value  any               `json:"value"`
	Where  map[string]string `json:"where"`
	Fields []string          `json:"fields"`
}

// Words is a string or a list of strings, as x-permission and unless are written.
type Words []string

// UnmarshalJSON reads "a" and ["a", "b"] alike.
func (w *Words) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*w = Words{one}

		return nil
	}

	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return fmt.Errorf("contract - Words: %w", err)
	}

	*w = many

	return nil
}

// Operation is one method on one path.
type Operation struct {
	ID     string
	Method string
	Path   string
	// Permission is what the caller needs: Public, SignedIn, or any one of these action keys.
	Permission Words
	// BranchScoped is x-branch-scoped as written: "false", "true", "filter", "hidden", "clinical"…
	BranchScoped string
	Masks        []Mask
	// Audit is x-audit: each line "<action> <resource>[: how often, or for whom]".
	Audit []string
	// Responses are the answers' schemas by status ("200", "default"); nil for one with no body.
	Responses map[string]json.RawMessage
	Body      json.RawMessage
}

// Write says whether the operation changes something: anything but GET.
func (o *Operation) Write() bool { return o.Method != http.MethodGet }

// IsPublic says whether anyone, signed in or not, may call it.
func (o *Operation) IsPublic() bool { return len(o.Permission) == 1 && o.Permission[0] == Public }

// Actions are the action keys of which the caller needs one; none for Public and SignedIn.
func (o *Operation) Actions() []string {
	if len(o.Permission) == 1 && (o.Permission[0] == Public || o.Permission[0] == SignedIn) {
		return nil
	}

	return o.Permission
}

// FiberPath is the path with {name} written :name, as Fiber routes it.
func (o *Operation) FiberPath() string {
	return strings.NewReplacer("{", ":", "}", "").Replace(o.Path)
}

// Contract is a read contract.
type Contract struct {
	ops     []*Operation
	schemas map[string]json.RawMessage
}

type rawOperation struct {
	OperationID  string                     `json:"operationId"`
	Permission   Words                      `json:"x-permission"`
	BranchScoped json.RawMessage            `json:"x-branch-scoped"`
	Masks        []Mask                     `json:"x-masks"`
	Audit        []string                   `json:"x-audit"`
	Responses    map[string]json.RawMessage `json:"responses"`
	RequestBody  *struct {
		Content map[string]struct {
			Schema json.RawMessage `json:"schema"`
		} `json:"content"`
	} `json:"requestBody"`
}

type rawFile struct {
	Paths      map[string]map[string]rawOperation `json:"paths"`
	Components struct {
		Schemas   map[string]json.RawMessage `json:"schemas"`
		Responses map[string]json.RawMessage `json:"responses"`
	} `json:"components"`
}

// Load reads the contract's operations, in path order, each with an operationId.
func Load(data []byte) (*Contract, error) {
	var f rawFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadContract, err)
	}

	c := &Contract{schemas: f.Components.Schemas}

	for path, methods := range f.Paths {
		for method := range methods {
			raw := methods[method]

			op, err := operation(path, method, &raw, f.Components.Responses)
			if err != nil {
				return nil, err
			}

			c.ops = append(c.ops, op)
		}
	}

	sort.Slice(c.ops, func(i, j int) bool {
		if c.ops[i].Path != c.ops[j].Path {
			return c.ops[i].Path < c.ops[j].Path
		}

		return c.ops[i].Method < c.ops[j].Method
	})

	return c, nil
}

func operation(path, method string, raw *rawOperation, shared map[string]json.RawMessage) (*Operation, error) {
	if raw.OperationID == "" || len(raw.Permission) == 0 {
		return nil, fmt.Errorf("%w: %s %s needs an operationId and an x-permission", ErrBadContract, strings.ToUpper(method), path)
	}

	op := &Operation{
		ID:           raw.OperationID,
		Method:       strings.ToUpper(method),
		Path:         path,
		Permission:   raw.Permission,
		BranchScoped: strings.Trim(string(raw.BranchScoped), `"`),
		Masks:        raw.Masks,
		Audit:        raw.Audit,
		Responses:    map[string]json.RawMessage{},
	}

	for status, r := range raw.Responses {
		if s := responseSchema(r, shared); s != nil {
			op.Responses[status] = s
		}
	}

	if raw.RequestBody != nil {
		op.Body = raw.RequestBody.Content["application/json"].Schema
	}

	return op, nil
}

// responseSchema is a response's JSON schema, through a $ref to components.responses.
func responseSchema(r json.RawMessage, shared map[string]json.RawMessage) json.RawMessage {
	var resp struct {
		Ref     string `json:"$ref"`
		Content map[string]struct {
			Schema json.RawMessage `json:"schema"`
		} `json:"content"`
	}
	if json.Unmarshal(r, &resp) != nil {
		return nil
	}

	if resp.Ref != "" {
		if target, ok := shared[strings.TrimPrefix(resp.Ref, "#/components/responses/")]; ok {
			return responseSchema(target, nil)
		}

		return nil
	}

	return resp.Content["application/json"].Schema
}

// Operations are every operation, in path order.
func (c *Contract) Operations() []*Operation { return c.ops }

// Operation is the one with this operationId.
func (c *Contract) Operation(id string) (*Operation, bool) {
	for _, op := range c.ops {
		if op.ID == id {
			return op, true
		}
	}

	return nil, false
}
