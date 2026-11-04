package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"testing"

	"github.com/evrone/go-clean-template/api"
	"github.com/evrone/go-clean-template/pkg/contract"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// The client's contract (api/openapi.json) over the real stack: Postgres,
// the app, the session cookie. People and branches are seeded straight into
// the database, as an operator would, since no operation makes a group.
const (
	basePathAPI  = httpURL + "/api"
	clientPass   = "acme-pass-123"
	deskEmail    = "desk@acme.test"
	ownerEmail   = "owner@acme.test"
	patientPhone = "0399 991 152"
)

// seed makes CN1 and CN2, a desk group (no full phones) and an owner group,
// and one person in each; each test calls it, and running again changes nothing.
func seed(t *testing.T) *pgx.Conn {
	t.Helper()

	conn, err := pgx.Connect(t.Context(), os.Getenv("PG_URL"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(context.Background()) })

	hash, err := bcrypt.GenerateFromPassword([]byte(clientPass), bcrypt.MinCost)
	require.NoError(t, err)

	_, err = conn.Exec(t.Context(), `
INSERT INTO workspaces (id, code, name) VALUES ('b-cn1', 'CN1', 'Chi nhánh 1'), ('b-cn2', 'CN2', 'Chi nhánh 2') ON CONFLICT DO NOTHING;
INSERT INTO permission_groups (id, name, grants) VALUES
  ('g-desk', 'Lễ tân', '{pat.patient.view,pat.patient.create,pat.patient.edit}'),
  ('g-owner', 'Chủ', '{pat.patient.view,pat.patient.create,pat.patient.edit,patient.phone.view_full,set.branch.all}')
ON CONFLICT DO NOTHING;
INSERT INTO users (id, username, email, password_hash, name, role, group_id) VALUES
  (gen_random_uuid(), 'acme-desk', '`+deskEmail+`', '`+string(hash)+`', 'Lan', 'member', 'g-desk'),
  (gen_random_uuid(), 'acme-owner', '`+ownerEmail+`', '`+string(hash)+`', 'Chủ', 'owner', 'g-owner')
ON CONFLICT DO NOTHING;
INSERT INTO user_workspaces (user_id, workspace_id) SELECT id, 'b-cn1' FROM users WHERE email = '`+deskEmail+`' ON CONFLICT DO NOTHING;`)
	require.NoError(t, err)

	return conn
}

// client is a browser: a cookie jar, and X-Requested-With on every write, as the app's client sends.
type client struct {
	http *http.Client
}

func signIn(t *testing.T, email string) *client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	c := &client{http: &http.Client{Jar: jar}}
	r := c.do(t, http.MethodPost, "/session", map[string]string{"email": email, "password": clientPass})
	require.Equal(t, http.StatusOK, r.status, string(r.body))

	return c
}

// response is a call's status and body.
type response struct {
	status int
	body   []byte
}

func (c *client) do(t *testing.T, method, path string, body any) response {
	t.Helper()

	var reader io.Reader = http.NoBody

	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)

		reader = bytes.NewReader(raw)
	}

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, basePathAPI+path, reader)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	if method != http.MethodGet {
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
	}

	resp, err := c.http.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return response{status: resp.StatusCode, body: raw}
}

func spec(t *testing.T) *contract.Contract {
	t.Helper()

	c, err := contract.Load(api.Spec())
	require.NoError(t, err)

	return c
}

// answers holds a 200 to the operation's own schema in the contract.
func answers(t *testing.T, opID string, r response) map[string]any {
	t.Helper()

	status, body := r.status, r.body
	require.Equal(t, http.StatusOK, status, string(body))

	s := spec(t)
	op, ok := s.Operation(opID)
	require.True(t, ok)
	require.NoError(t, s.ValidateJSON(op.Responses["200"], body), "%s answers what the contract says", opID)

	var v map[string]any
	require.NoError(t, json.Unmarshal(body, &v))

	return v
}

// newPatient makes a patient at CN1 with a name no other run has.
func newPatient(t *testing.T, desk *client) map[string]any {
	t.Helper()

	return answers(t, "patCreate", desk.do(t, http.MethodPost, "/patients", map[string]string{"name": "Hương " + uuid.NewString()[:8], "phone": patientPhone, "workspace": "b-cn1", "gender": "female"}))
}

func TestClientAPI_SigningInGivesTheSessionTheClientReads(t *testing.T) {
	seed(t)

	desk := signIn(t, deskEmail)
	me := answers(t, "me", desk.do(t, http.MethodGet, "/session", nil))

	assert.Equal(t, "Lan", me["name"])
	assert.Contains(t, me["grants"], "pat.patient.view")
	assert.NotEmpty(t, me["sessionEndsAt"])

	answers(t, "refreshSession", desk.do(t, http.MethodPost, "/session/refresh", nil))

	assert.Equal(t, http.StatusNoContent, desk.do(t, http.MethodDelete, "/session", nil).status)
	assert.Equal(t, http.StatusUnauthorized, desk.do(t, http.MethodGet, "/session", nil).status, "signed out is signed out")
}

func TestClientAPI_APatientIsIssuedACodeAndMaskedForTheDesk(t *testing.T) {
	seed(t)

	desk, owner := signIn(t, deskEmail), signIn(t, ownerEmail)
	made := newPatient(t, desk)

	assert.Regexp(t, `^KH-\d{6}$`, made["code"])
	assert.Equal(t, "0399 *** 152", made["phone"], "the desk sees what it made masked too")

	id, ok := made["id"].(string)
	require.True(t, ok)

	full := answers(t, "patGet", owner.do(t, http.MethodGet, "/patients/"+id, nil))
	assert.Equal(t, patientPhone, full["phone"])
}

func TestClientAPI_TheDeskFindsANumberOnlyByItsLastThree(t *testing.T) {
	seed(t)

	desk, owner := signIn(t, deskEmail), signIn(t, ownerEmail)
	newPatient(t, desk)

	total := func(c *client, q string) float64 {
		page := answers(t, "patList", c.do(t, http.MethodGet, "/patients?q="+q, nil))
		n, ok := page["total"].(float64)
		require.True(t, ok)

		return n
	}

	assert.Positive(t, total(desk, "152"), "the last three digits, as the mask shows them")
	assert.Zero(t, total(desk, "991"), "the middle digits are hidden, so they match nothing")
	assert.Positive(t, total(owner, "991"), "a reader with full phones finds them")
}

func TestClientAPI_AnotherBranchIsRefused(t *testing.T) {
	seed(t)

	desk := signIn(t, deskEmail)

	r := desk.do(t, http.MethodPost, "/patients", map[string]string{"name": "An", "phone": "0901234567", "workspace": "b-cn2"})
	assert.Equal(t, http.StatusForbidden, r.status)
	assert.Contains(t, string(r.body), `"not_member"`)

	assert.Equal(t, http.StatusForbidden, desk.do(t, http.MethodGet, "/patients?madeAt=b-cn2", nil).status)
}

func TestClientAPI_AWriteFromAnotherSiteIsRefused(t *testing.T) {
	seed(t)

	desk := signIn(t, deskEmail)

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, basePathAPI+"/patients", strings.NewReader(`{"name": "An", "phone": "0901234567"}`))
	require.NoError(t, err)

	resp, err := desk.http.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "the cookie went, the app's header didn't")
}

func TestClientAPI_ALookIsOneLineInTenMinutesAndAChangeKeepsOldAndNew(t *testing.T) {
	db := seed(t)

	desk := signIn(t, deskEmail)
	made := newPatient(t, desk)

	id, ok := made["id"].(string)
	require.True(t, ok)

	desk.do(t, http.MethodGet, "/patients/"+id, nil)
	desk.do(t, http.MethodGet, "/patients/"+id, nil)

	var looks int
	require.NoError(t, db.QueryRow(t.Context(), `SELECT count(*) FROM audit_log WHERE action = 'view' AND resource = 'record' AND target = $1`, id).Scan(&looks))
	assert.Equal(t, 1, looks, "two looks a moment apart are one line")

	answers(t, "patUpdate", desk.do(t, http.MethodPatch, "/patients/"+id, map[string]string{"note": "đau răng"}))

	var changes string
	require.NoError(t, db.QueryRow(t.Context(), `SELECT changes::text FROM audit_log WHERE action = 'update' AND resource = 'patient' AND target = $1`, id).Scan(&changes))
	assert.JSONEq(t, `[{"field": "note", "from": "", "to": "đau răng"}]`, changes)
}

func TestClientAPI_TheLogCannotBeChangedOrEmptied(t *testing.T) {
	db := seed(t)

	signIn(t, deskEmail) // a line to change

	_, err := db.Exec(t.Context(), `UPDATE audit_log SET actor_name = 'someone else'`)
	require.ErrorContains(t, err, "append-only")

	_, err = db.Exec(t.Context(), `DELETE FROM audit_log`)
	require.ErrorContains(t, err, "append-only")

	_, err = db.Exec(t.Context(), `TRUNCATE audit_log`)
	require.ErrorContains(t, err, "append-only")
}

func TestClientAPI_AnOperationNotBuiltSaysSo(t *testing.T) {
	seed(t)

	r := signIn(t, ownerEmail).do(t, http.MethodGet, "/workspaces", nil)

	assert.Equal(t, http.StatusNotImplemented, r.status)
	assert.Contains(t, string(r.body), `"unavailable"`)
}
