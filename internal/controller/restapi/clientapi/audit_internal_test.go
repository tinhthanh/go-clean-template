package clientapi

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/api"
	"github.com/evrone/go-clean-template/pkg/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadAudit_ALineSaysWhatHowOftenAndWhoseItIs(t *testing.T) {
	t.Parallel()

	r, ok := readAudit("view record: one line per person per patient per 10 minutes")
	require.True(t, ok)
	assert.Equal(t, auditRule{Action: "view", Resource: "record", Within: 10 * time.Minute}, r)

	r, _ = readAudit("update patient, each field's old and new value (a phone change needs patient.phone.view_full)")
	assert.Equal(t, auditRule{Action: "update", Resource: "patient"}, r)

	r, _ = readAudit("view debt, for one who reads the open patient's debt alone (fin.debt.view_open): every time, at most 20 a minute")
	assert.True(t, r.Handler, "a line for some readers only is the handler's to write")

	r, _ = readAudit("view record (the header): one line per person per patient per 10 minutes")
	assert.Equal(t, auditRule{Action: "view", Resource: "record", Within: 10 * time.Minute}, r, "a note in brackets is left out")

	r, _ = readAudit("view exam (the clinical content), apart: the same")
	assert.Equal(t, auditRule{Action: "view", Resource: "exam", Same: true}, r, `"the same" is as often as the line before`)

	_, ok = readAudit("nonsense")
	assert.False(t, ok)
}

// Every x-audit line of the client's contract reads into an action and a
// resource the log's schema names, so a new line can't be written wrong.
func TestReadAudit_EveryLineOfTheContractReads(t *testing.T) {
	t.Parallel()

	spec, err := contract.Load(api.Spec())
	require.NoError(t, err)

	enum := func(name string) []string {
		var s struct {
			Enum []string `json:"enum"`
		}
		require.NoError(t, json.Unmarshal(spec.Schema(name), &s))

		return s.Enum
	}
	actions, resources := enum("AuditAction"), enum("AuditResource")

	for _, op := range spec.Operations() {
		for _, line := range op.Audit {
			r, ok := readAudit(line)
			if !assert.True(t, ok, "%s: %q", op.ID, line) {
				continue
			}

			assert.True(t, slices.Contains(actions, r.Action), "%s: action %q of %q", op.ID, r.Action, line)
			assert.True(t, slices.Contains(resources, r.Resource), "%s: resource %q of %q", op.ID, r.Resource, line)
		}
	}
}
