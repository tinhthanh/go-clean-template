// Package clientapi serves the client's contract (api/openapi.json): a
// route for every operation in it, each behind the guard the contract
// writes beside it (guard.go: the session cookie, CSRF, x-permission, the
// branch, the request body's schema, then x-masks and x-audit on the
// answer). An operation with no handler here answers 501 unavailable, so
// the client sees what isn't built yet rather than a 404.
package clientapi

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/contract"
	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/gofiber/fiber/v3"
)

// ErrUnknownOperation is a handler for an operationId the contract doesn't have.
var ErrUnknownOperation = errors.New("clientapi: a handler for an operation the contract doesn't have")

// Cookies are the session cookies' settings.
type Cookies struct {
	// Prefix is where the API is mounted ("/api"): the refresh cookie goes only to its refresh path.
	Prefix        string
	Secure        bool
	SessionLength time.Duration
	RefreshLength time.Duration
}

// Deps -.
type Deps struct {
	Contract *contract.Contract
	Session  usecase.Session
	Audit    usecase.Audit
	Patient  usecase.Patient
	// Access signs the session cookie, Refresh the refresh cookie, each with its own secret.
	Access  *jwt.Manager
	Refresh *jwt.Manager
	Cookies Cookies
	Logger  logger.Interface
}

// API -.
type API struct {
	d     *Deps
	built map[string]fiber.Handler
}

// NewRoutes routes every operation of the contract on r.
func NewRoutes(r fiber.Router, d *Deps) (*API, error) {
	a := &API{d: d}
	a.built = map[string]fiber.Handler{
		"me":             a.me,
		"signIn":         a.signIn,
		"signOut":        a.signOut,
		"refreshSession": a.refresh,
		"patList":        a.patList,
		"patGet":         a.patGet,
		"patCreate":      a.patCreate,
		"patUpdate":      a.patUpdate,
	}

	for id := range a.built {
		if _, ok := d.Contract.Operation(id); !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownOperation, id)
		}
	}

	for _, op := range d.Contract.Operations() {
		h, ok := a.built[op.ID]
		if !ok {
			h = a.notBuilt
		}

		r.Add([]string{op.Method}, op.FiberPath(), a.guard(op), h)
	}

	return a, nil
}

// Built is the operationIds this server answers, sorted; the rest answer 501.
func (a *API) Built() []string {
	ids := make([]string, 0, len(a.built))
	for id := range a.built {
		ids = append(ids, id)
	}

	slices.Sort(ids)

	return ids
}
