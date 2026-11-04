package contract_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/evrone/go-clean-template/pkg/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _spec = `{
  "paths": {
    "/people": {
      "get": {
        "operationId": "peopleList", "x-permission": "people.view", "x-branch-scoped": "filter",
        "x-masks": [{"path": "items[].phone", "unless": "people.phone", "as": "masked"},
                    {"path": "items[].salary", "unless": ["money.view", "money.own"], "as": "absent"}],
        "x-audit": [],
        "responses": {"200": {"content": {"application/json": {"schema": {"allOf": [
          {"$ref": "#/components/schemas/Page"},
          {"properties": {"items": {"type": "array", "items": {"$ref": "#/components/schemas/Person"}}}}]}}}},
          "401": {"$ref": "#/components/responses/Unauthorized"}}
      },
      "post": {
        "operationId": "peopleCreate", "x-permission": ["people.create", "people.admin"], "x-branch-scoped": true,
        "x-masks": [], "x-audit": ["create person"],
        "requestBody": {"content": {"application/json": {"schema": {"type": "object", "required": ["name"]}}}},
        "responses": {"200": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/Person"}}}}}
      }
    },
    "/people/{id}/log": {
      "get": {
        "operationId": "peopleLog", "x-permission": "signed-in", "x-branch-scoped": false,
        "x-masks": [{"path": "items[].changes[].to", "unless": "people.phone", "as": "masked", "where": {"resource": "person"}, "fields": ["phone"]}],
        "x-audit": [], "responses": {"204": {"description": "nothing"}}
      }
    },
    "/session": {"post": {"operationId": "signIn", "x-permission": "public", "x-branch-scoped": false, "x-masks": [], "x-audit": [], "responses": {}}}
  },
  "components": {
    "schemas": {
      "Page": {"type": "object", "required": ["items", "total"], "additionalProperties": false,
               "properties": {"items": {"type": "array"}, "next": {"type": "string"}, "total": {"type": "integer", "minimum": 0}}},
      "Person": {"type": "object", "required": ["id", "name"], "additionalProperties": false,
                 "properties": {"id": {"type": "string"}, "name": {"type": "string"}, "phone": {"type": "string"},
                                "phoneMasked": {"type": "boolean"}, "salary": {"type": "integer"},
                                "kind": {"type": "string", "enum": ["staff", "guest"]}, "born": {"type": "string", "format": "date"}}}
    },
    "responses": {"Unauthorized": {"content": {"application/json": {"schema": {"type": "object", "required": ["error"]}}}}}
  }
}`

func load(t *testing.T) *contract.Contract {
	t.Helper()

	c, err := contract.Load([]byte(_spec))
	require.NoError(t, err)

	return c
}

func TestLoad_EveryOperationWithItsRules(t *testing.T) {
	t.Parallel()

	c := load(t)
	ids := []string{}

	for _, op := range c.Operations() {
		ids = append(ids, op.ID)
	}

	assert.Equal(t, []string{"peopleList", "peopleCreate", "peopleLog", "signIn"}, ids)

	list, _ := c.Operation("peopleList")
	assert.Equal(t, []string{"people.view"}, list.Actions())
	assert.Equal(t, "filter", list.BranchScoped)
	assert.False(t, list.Write())

	create, _ := c.Operation("peopleCreate")
	assert.Equal(t, []string{"people.create", "people.admin"}, create.Actions(), "a list of permissions: any one will do")
	assert.Equal(t, "true", create.BranchScoped)
	assert.True(t, create.Write())
	assert.NotNil(t, create.Body)

	log, _ := c.Operation("peopleLog")
	assert.Empty(t, log.Actions(), "signed-in needs no action")
	assert.Equal(t, "/people/:id/log", log.FiberPath())
	assert.NotContains(t, log.Responses, "204", "an answer with no body has no schema")

	signIn, _ := c.Operation("signIn")
	assert.True(t, signIn.IsPublic())
}

func TestLoad_AnOperationWithoutAPermissionIsRefused(t *testing.T) {
	t.Parallel()

	_, err := contract.Load([]byte(`{"paths": {"/x": {"get": {"operationId": "x", "responses": {}}}}}`))
	require.ErrorIs(t, err, contract.ErrBadContract)
}

func TestValidate_HoldsAnAnswerToItsSchema(t *testing.T) {
	t.Parallel()

	c := load(t)
	list, _ := c.Operation("peopleList")
	schema := list.Responses["200"]

	cases := map[string]struct {
		body string
		ok   bool
	}{
		"a page":                       {`{"items": [{"id": "1", "name": "An", "kind": "staff", "born": "1990-01-02"}], "total": 1}`, true},
		"a required field missing":     {`{"items": [{"id": "1"}], "total": 1}`, false},
		"a field the schema hasn't":    {`{"items": [], "total": 0, "extra": 1}`, false},
		"a value outside its enum":     {`{"items": [{"id": "1", "name": "An", "kind": "boss"}], "total": 1}`, false},
		"a number under its minimum":   {`{"items": [], "total": -1}`, false},
		"a fraction for an integer":    {`{"items": [], "total": 1.5}`, false},
		"a date that isn't one":        {`{"items": [{"id": "1", "name": "An", "born": "02/01/1990"}], "total": 1}`, false},
		"a string for an array":        {`{"items": "none", "total": 0}`, false},
		"the shared 401 answer, empty": {`{}`, false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := schema
			if strings.Contains(name, "401") {
				s = list.Responses["401"]
			}

			err := c.ValidateJSON(s, []byte(tc.body))
			if tc.ok {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, contract.ErrSchema)
			}
		})
	}
}

func TestMask_HidesWhatTheReaderMayNotSee(t *testing.T) {
	t.Parallel()

	c := load(t)
	list, _ := c.Operation("peopleList")

	answer := map[string]any{"total": 1, "items": []any{map[string]any{"id": "1", "name": "An", "phone": "0901234567", "salary": 9}}}
	none := func(string) bool { return false }

	masked, ok := c.Mask(list, answer, none).(map[string]any)
	require.True(t, ok)

	row, ok := masked["items"].([]any)[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "0901 *** 567", row["phone"])
	assert.Equal(t, true, row["phoneMasked"], "the schema has phoneMasked, so the client hears why")
	assert.NotContains(t, row, "salary")
}

func TestMask_AnyOneOfUnlessLetsTheReaderSee(t *testing.T) {
	t.Parallel()

	c := load(t)
	list, _ := c.Operation("peopleList")

	answer := map[string]any{"items": []any{map[string]any{"phone": "0901234567", "salary": 9}}}
	owner := func(a string) bool { return a == "money.own" || a == "people.phone" }

	row, ok := c.Mask(list, answer, owner).(map[string]any)["items"].([]any)[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "0901234567", row["phone"])
	assert.InDelta(t, 9, row["salary"], 0)
	assert.NotContains(t, row, "phoneMasked")
}

func TestMask_WhereAndFieldsNarrowTheLinesItApplies(t *testing.T) {
	t.Parallel()

	c := load(t)
	logOp, _ := c.Operation("peopleLog")

	answer := map[string]any{"items": []any{
		map[string]any{"resource": "person", "changes": []any{
			map[string]any{"field": "phone", "to": "0901234567"},
			map[string]any{"field": "name", "to": "An"},
		}},
		map[string]any{"resource": "invoice", "changes": []any{map[string]any{"field": "phone", "to": "0907654321"}}},
	}}

	out, err := json.Marshal(c.Mask(logOp, answer, func(string) bool { return false }))
	require.NoError(t, err)

	assert.Contains(t, string(out), `"0901 *** 567"`, "the person's phone is masked")
	assert.Contains(t, string(out), `"An"`, "a change to another field is left")
	assert.Contains(t, string(out), `"0907654321"`, "a line about another resource is left")
}

func TestMasked_KeepsTheLastThree(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "0901 *** 567", contract.Masked("phone", "0901 234 567"))
	assert.Equal(t, "**45", contract.Masked("phone", "2345"))
	assert.Equal(t, "*********123", contract.Masked("cccd", "001090000123"))
	assert.Equal(t, "***ret", contract.Masked("note", "secret"), "text with no digits keeps its last three")
}

// The real contract, the client's (api/openapi.json): it loads, every
// operationId is used once, and every answer schema resolves.
func TestLoad_TheAppsContract(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../api/openapi.json")
	require.NoError(t, err)

	c, err := contract.Load(data)
	require.NoError(t, err)
	require.NotEmpty(t, c.Operations())

	seen := map[string]bool{}

	for _, op := range c.Operations() {
		assert.False(t, seen[op.ID], "operationId %s used twice", op.ID)
		seen[op.ID] = true

		for status, s := range op.Responses {
			assert.NotErrorIs(t, c.Validate(s, nil), contract.ErrBadContract, "%s %s", op.ID, status)
		}
	}
}
