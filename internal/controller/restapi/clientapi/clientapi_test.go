package clientapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/api"
	"github.com/evrone/go-clean-template/internal/controller/restapi/clientapi"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/pkg/contract"
	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	_desk   = "u-desk"   // a receptionist at CN1, who sees phones masked
	_owner  = "u-owner"  // every branch, every grant
	_viewer = "u-viewer" // signed in, no grants
)

var errDown = context.DeadlineExceeded

// people are the fixture's principals, by user id.
func people() map[string]entity.Principal {
	return map[string]entity.Principal{
		_desk:   {UserID: _desk, Name: "Lan", Email: "lan@acme.test", Role: "member", Grants: []string{"pat.patient.view", "pat.patient.create", "pat.patient.edit"}, Workspaces: []string{"b-cn1"}},
		_owner:  {UserID: _owner, Name: "Owner", Email: "owner@acme.test", Role: "owner", Grants: []string{"pat.patient.view", "pat.patient.create", "patient.phone.view_full", entity.ActionAllBranches}},
		_viewer: {UserID: _viewer, Name: "View", Email: "view@acme.test", Role: "viewer"},
	}
}

type fakeSession struct{}

func (fakeSession) SignIn(_ context.Context, email, password string) (entity.Principal, error) {
	if email == "lan@acme.test" && password == "right" {
		return people()[_desk], nil
	}

	return entity.Principal{}, entity.ErrUnauthorized
}

func (fakeSession) Principal(_ context.Context, id string) (entity.Principal, error) {
	p, ok := people()[id]
	if !ok {
		return entity.Principal{}, entity.ErrUnauthorized
	}

	return p, nil
}

type fakeAudit struct {
	mu    sync.Mutex
	lines []entity.AuditLine
	once  []time.Duration
	fail  bool
}

func (f *fakeAudit) Write(_ context.Context, line *entity.AuditLine) error {
	return f.add(line, 0)
}

func (f *fakeAudit) WriteOnce(_ context.Context, line *entity.AuditLine, within time.Duration) error {
	return f.add(line, within)
}

func (f *fakeAudit) add(line *entity.AuditLine, within time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.fail {
		return errDown
	}

	f.lines = append(f.lines, *line)
	f.once = append(f.once, within)

	return nil
}

type fakePatients struct{}

// shop is the one patient the fake knows.
func shop() entity.Patient {
	return entity.Patient{
		ID: "p-1", Code: "KH-000001", Workspace: "b-cn1", Name: "Hương", Gender: "female", Phone: "0399 991 152",
		CCCD: "001090000123", CreatedAt: time.Date(2026, 11, 1, 2, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 11, 1, 2, 0, 0, 0, time.UTC),
	}
}

func (fakePatients) List(_ context.Context, _ *entity.Principal, q *entity.PatientQuery) (entity.PatientPage, error) {
	return entity.PatientPage{Items: []entity.Patient{shop()}, Total: int(q.Offset) + 2, Female: 2}, nil //nolint:gosec // a test's small numbers
}

func (fakePatients) Get(_ context.Context, _ *entity.Principal, id string) (entity.Patient, error) {
	if id != shop().ID {
		return entity.Patient{}, entity.ErrNotFound
	}

	return shop(), nil
}

func (fakePatients) Create(_ context.Context, p *entity.Principal, in *entity.Patient) (entity.Patient, error) {
	if !p.Works(in.Workspace) {
		return entity.Patient{}, entity.ErrNotMember
	}

	made := shop()
	made.ID, made.Workspace, made.Name = "p-new", in.Workspace, in.Name

	return made, nil
}

func (fakePatients) Update(_ context.Context, _ *entity.Principal, _ string, patch *entity.PatientPatch) (entity.Patient, []entity.Change, error) {
	changed := shop()
	changed.Name = *patch.Name

	return changed, []entity.Change{{Field: "name", From: shop().Name, To: changed.Name}}, nil
}

type harness struct {
	app       *fiber.App
	api       *clientapi.API
	spec      *contract.Contract
	audit     *fakeAudit
	access    *jwt.Manager
	refresher *jwt.Manager
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	spec, err := contract.Load(api.Spec())
	require.NoError(t, err)

	h := &harness{app: fiber.New(), spec: spec, audit: &fakeAudit{}, access: jwt.New("s:session", time.Hour), refresher: jwt.New("s:refresh", 24*time.Hour)}

	h.api, err = clientapi.NewRoutes(h.app.Group("/api"), &clientapi.Deps{
		Contract: spec, Session: fakeSession{}, Audit: h.audit, Patient: fakePatients{},
		Access: h.access, Refresh: h.refresher,
		Cookies: clientapi.Cookies{Prefix: "/api", Secure: true, SessionLength: time.Hour, RefreshLength: 24 * time.Hour},
		Logger:  logger.New("error"),
	})
	require.NoError(t, err)

	return h
}

type call struct {
	method, path, body, as string
	header                 map[string]string
	// write: send X-Requested-With, as the client does on every write.
	write bool
}

type reply struct {
	status  int
	body    map[string]any
	raw     []byte
	cookies []*http.Cookie
}

func (h *harness) do(t *testing.T, c *call) reply {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), c.method, c.path, strings.NewReader(c.body))
	req.Header.Set("Content-Type", "application/json")

	if c.write {
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
	}

	for k, v := range c.header {
		req.Header.Set(k, v)
	}

	if c.as != "" {
		token, err := h.access.GenerateToken(c.as)
		require.NoError(t, err)
		req.AddCookie(&http.Cookie{Name: "session", Value: token, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	}

	resp, err := h.app.Test(req)
	require.NoError(t, err)

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	r := reply{status: resp.StatusCode, raw: raw, cookies: resp.Cookies()}
	if len(raw) > 0 {
		require.NoError(t, json.Unmarshal(raw, &r.body))
	}

	return r
}

// refusal holds a refusal to the contract's Error, and answers its code.
func (h *harness) refusal(t *testing.T, r reply, status int) string {
	t.Helper()

	assert.Equal(t, status, r.status, string(r.raw))
	require.NoError(t, h.spec.ValidateJSON(h.spec.Schema("Error"), r.raw), "a refusal is the contract's Error")

	code, ok := r.body["code"].(string)
	require.True(t, ok, "a refusal names its code")

	return code
}

func (h *harness) answers(t *testing.T, opID string, r reply) {
	t.Helper()

	op, ok := h.spec.Operation(opID)
	require.True(t, ok)
	require.Equal(t, http.StatusOK, r.status, string(r.raw))
	require.NoError(t, h.spec.ValidateJSON(op.Responses["200"], r.raw), "the answer is the contract's")
}

func TestRoutes_EveryOperationIsAnsweredAndOneNotBuiltSaysSo(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	assert.Contains(t, h.api.Built(), "patList")
	// /workspaces needs only a signed-in caller, and isn't built here.
	assert.Equal(t, "unavailable", h.refusal(t, h.do(t, &call{method: http.MethodGet, path: "/api/workspaces", as: _viewer}), http.StatusNotImplemented))
}

func TestRoutes_AHandlerForAnOperationTheContractLacksIsRefusedAtStart(t *testing.T) {
	t.Parallel()

	spec, err := contract.Load([]byte(`{"paths": {}}`))
	require.NoError(t, err)

	_, err = clientapi.NewRoutes(fiber.New(), &clientapi.Deps{Contract: spec})
	require.ErrorIs(t, err, clientapi.ErrUnknownOperation)
}

func TestGuard_AWriteWithoutTheAppsHeaderIsRefused(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	r := h.do(t, &call{method: http.MethodPost, path: "/api/patients", body: `{"name": "An", "phone": "0901234567"}`, as: _desk})

	assert.Equal(t, "forbidden", h.refusal(t, r, http.StatusForbidden))
	assert.Empty(t, h.audit.lines)
}

func TestGuard_NobodySignedInIsAskedToSignIn(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	assert.Equal(t, "unauthorized", h.refusal(t, h.do(t, &call{method: http.MethodGet, path: "/api/patients"}), http.StatusUnauthorized))
	assert.Equal(t, "unauthorized", h.refusal(t, h.do(t, &call{method: http.MethodGet, path: "/api/session", header: map[string]string{"Cookie": "session=forged"}}), http.StatusUnauthorized))
}

func TestGuard_AGroupWithoutTheGrantIsRefused(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	assert.Equal(t, "forbidden", h.refusal(t, h.do(t, &call{method: http.MethodGet, path: "/api/patients", as: _viewer}), http.StatusForbidden))
}

func TestGuard_AnotherBranchsWorkspaceIsNotTheirs(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	assert.Equal(t, "not_member", h.refusal(t, h.do(t, &call{method: http.MethodGet, path: "/api/patients?workspace=b-cn2", as: _desk}), http.StatusForbidden))
	assert.Equal(t, "not_member", h.refusal(t, h.do(t, &call{method: http.MethodGet, path: "/api/patients?workspace=all", as: _desk}), http.StatusForbidden), `"all" needs set.branch.all`)
	h.answers(t, "patList", h.do(t, &call{method: http.MethodGet, path: "/api/patients?workspace=all", as: _owner}))
}

func TestGuard_ABodyTheContractRefusesIsInvalidOnItsField(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	r := h.do(t, &call{method: http.MethodPost, path: "/api/patients", body: `{"name": "An"}`, as: _desk, write: true})
	assert.Equal(t, "invalid", h.refusal(t, r, http.StatusBadRequest))
	assert.Equal(t, "phone", r.body["field"])

	r = h.do(t, &call{method: http.MethodPost, path: "/api/patients", body: `{"name": "An", "phone": "0901234567", "gender": "other"}`, as: _desk, write: true})
	assert.Equal(t, "gender", r.body["field"])
}

func TestMasks_AReaderWithoutTheRightSeesThePhoneMasked(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	masked := h.do(t, &call{method: http.MethodGet, path: "/api/patients/p-1", as: _desk})
	h.answers(t, "patGet", masked)
	assert.Equal(t, "0399 *** 152", masked.body["phone"])
	assert.Equal(t, true, masked.body["phoneMasked"])
	assert.Equal(t, "*********123", masked.body["cccd"])

	full := h.do(t, &call{method: http.MethodGet, path: "/api/patients/p-1", as: _owner})
	assert.Equal(t, "0399 991 152", full.body["phone"])
	assert.NotContains(t, full.body, "phoneMasked")
}

func TestMasks_ApplyToEveryRowOfAList(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	r := h.do(t, &call{method: http.MethodGet, path: "/api/patients", as: _desk})

	h.answers(t, "patList", r)

	items, ok := r.body["items"].([]any)
	require.True(t, ok)

	row, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "0399 *** 152", row["phone"])
	assert.NotEmpty(t, r.body["next"], "a page short of the total says where the next starts")
}

func TestAudit_ALookAtARecordIsOneLineInTenMinutes(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.do(t, &call{method: http.MethodGet, path: "/api/patients/p-1", as: _desk, header: map[string]string{"X-Branch": "b-cn1"}})

	require.Len(t, h.audit.lines, 1, "the debt line is the handler's, for one who reads the debt alone")
	assert.Equal(t, entity.AuditLine{ActorID: _desk, ActorName: "Lan", Branch: "b-cn1", Action: "view", Resource: "record", Target: "p-1"}, h.audit.lines[0])
	assert.Equal(t, 10*time.Minute, h.audit.once[0])
}

func TestAudit_ABranchThatIsntTheirsIsNotWhereTheLineGoes(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.do(t, &call{method: http.MethodGet, path: "/api/patients/p-1", as: _desk, header: map[string]string{"X-Branch": "b-cn2"}})

	require.Len(t, h.audit.lines, 1)
	assert.Empty(t, h.audit.lines[0].Branch)
}

func TestAudit_AChangeKeepsEachFieldsOldAndNew(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.answers(t, "patUpdate", h.do(t, &call{method: http.MethodPatch, path: "/api/patients/p-1", body: `{"name": "Thanh Hương"}`, as: _desk, write: true}))

	require.Len(t, h.audit.lines, 1)
	assert.Equal(t, "update", h.audit.lines[0].Action)
	assert.Equal(t, []entity.Change{{Field: "name", From: "Hương", To: "Thanh Hương"}}, h.audit.lines[0].Changes)
}

func TestAudit_ARecordMadeIsTheLinesTarget(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.answers(t, "patCreate", h.do(t, &call{method: http.MethodPost, path: "/api/patients", body: `{"name": "An", "phone": "0901234567", "workspace": "b-cn1"}`, as: _desk, write: true}))

	require.Len(t, h.audit.lines, 1)
	assert.Equal(t, "p-new", h.audit.lines[0].Target)
	assert.Equal(t, "create", h.audit.lines[0].Action)
}

func TestAudit_ALookThatCantBeLoggedIsNotShown(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.audit.fail = true

	r := h.do(t, &call{method: http.MethodGet, path: "/api/patients/p-1", as: _owner})

	assert.Equal(t, http.StatusInternalServerError, r.status)
	assert.NotContains(t, string(r.raw), "0399", "nothing of the record went out")
}

func TestSession_SigningInSetsCookiesNoScriptCanRead(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	r := h.do(t, &call{method: http.MethodPost, path: "/api/session", body: `{"email": "lan@acme.test", "password": "right"}`, write: true})

	h.answers(t, "signIn", r)
	assert.NotEmpty(t, r.body["sessionEndsAt"])

	byName := map[string]*http.Cookie{}
	for _, c := range r.cookies {
		byName[c.Name] = c
	}

	require.Contains(t, byName, "session")
	require.Contains(t, byName, "refresh")

	for _, c := range byName {
		assert.True(t, c.HttpOnly, c.Name)
		assert.True(t, c.Secure, c.Name)
		assert.Equal(t, http.SameSiteLaxMode, c.SameSite, c.Name)
	}

	assert.Equal(t, "/api/session/refresh", byName["refresh"].Path, "the refresh cookie goes only to the refresh path")
	require.Len(t, h.audit.lines, 1)
	assert.Equal(t, entity.AuditLine{ActorID: _desk, ActorName: "Lan", Action: "signin", Resource: "session", Target: _desk}, h.audit.lines[0])
}

func TestSession_AWrongPasswordIsUnauthorized(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	r := h.do(t, &call{method: http.MethodPost, path: "/api/session", body: `{"email": "lan@acme.test", "password": "wrong"}`, write: true})

	assert.Equal(t, "unauthorized", h.refusal(t, r, http.StatusUnauthorized))
	assert.Empty(t, r.cookies)
}

func TestSession_MeSaysWhoAndWhenTheSessionEnds(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	r := h.do(t, &call{method: http.MethodGet, path: "/api/session", as: _desk})

	h.answers(t, "me", r)
	assert.Equal(t, _desk, r.body["id"])
	assert.NotEmpty(t, r.body["sessionEndsAt"])
}

func TestSession_RefreshTakesTheRefreshCookieNotTheSessionOne(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	r := h.do(t, &call{method: http.MethodPost, path: "/api/session/refresh", as: _desk, write: true})
	assert.Equal(t, "unauthorized", h.refusal(t, r, http.StatusUnauthorized), "a session cookie is no refresh cookie")

	token, err := h.refresher.GenerateToken(_desk)
	require.NoError(t, err)

	r = h.do(t, &call{method: http.MethodPost, path: "/api/session/refresh", write: true, header: map[string]string{"Cookie": "refresh=" + token}})
	h.answers(t, "refreshSession", r)
}

func TestSession_SigningOutEndsBothCookies(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	r := h.do(t, &call{method: http.MethodDelete, path: "/api/session", as: _desk, write: true})

	assert.Equal(t, http.StatusNoContent, r.status)

	for _, c := range r.cookies {
		assert.Empty(t, c.Value, c.Name)
		assert.True(t, c.Expires.Before(time.Now()), c.Name)
	}
}
