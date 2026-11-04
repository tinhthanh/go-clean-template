package clientapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/pkg/contract"
	"github.com/gofiber/fiber/v3"
)

const (
	_sessionCookie = "session"
	_refreshCookie = "refresh"
	// The operation whose caller is named by the refresh cookie, not the session cookie.
	_refreshOp = "refreshSession"

	_localPrincipal = "clientapi.principal"
	_localBranch    = "clientapi.branch"
	_localChanges   = "clientapi.changes"
)

// guard is what the contract writes beside an operation, in order: a write
// carries X-Requested-With (CSRF); the session cookie names the caller;
// x-permission; the workspace asked is one of theirs; the body is what the
// operation's schema says. On a 2xx it hides what x-masks says the caller
// may not see, then writes the x-audit lines. A handler copies none of it.
func (a *API) guard(op *contract.Operation) fiber.Handler {
	return func(c fiber.Ctx) error {
		if err := a.before(c, op); err != nil {
			return a.refuse(c, err)
		}

		if err := c.Next(); err != nil {
			return err //nolint:wrapcheck // Fiber's own, for its error handler
		}

		if status := c.Response().StatusCode(); status < http.StatusOK || status >= http.StatusMultipleChoices {
			return nil
		}

		if err := a.mask(c, op); err != nil {
			return a.refuse(c, err)
		}

		// A look or a change that can't be logged isn't answered.
		if err := a.audit(c, op); err != nil {
			c.Response().ResetBody()

			return a.refuse(c, err)
		}

		return nil
	}
}

// before is every check made ahead of the handler.
func (a *API) before(c fiber.Ctx, op *contract.Operation) error {
	if op.Write() && c.Get("X-Requested-With") != "XMLHttpRequest" {
		return errCSRF
	}

	p, err := a.caller(c, op)
	if err != nil {
		return err
	}

	if err := allowed(c, op, p); err != nil {
		return err
	}

	return a.validBody(c, op)
}

// caller is the person the session cookie names, or nil when there is none;
// a public operation is answered either way.
func (a *API) caller(c fiber.Ctx, op *contract.Operation) (*entity.Principal, error) {
	name, manager := _sessionCookie, a.d.Access
	if op.ID == _refreshOp {
		name, manager = _refreshCookie, a.d.Refresh
	}

	token := c.Cookies(name)
	if token == "" {
		return nil, nil //nolint:nilnil // no cookie is nobody, which a public operation allows
	}

	userID, ends, err := manager.ParseExpiry(token)
	if err != nil {
		if op.IsPublic() {
			return nil, nil //nolint:nilnil // an ended session on a public operation is nobody
		}

		return nil, entity.ErrUnauthorized
	}

	p, err := a.d.Session.Principal(c.Context(), userID)
	if err != nil {
		return nil, err
	}

	if name == _sessionCookie {
		p.SessionEndsAt = ends
	}

	c.Locals(_localPrincipal, &p)

	return &p, nil
}

// allowed holds the caller to x-permission and to the branches they work
// in: a workspace asked that isn't theirs is not_member, and "all" needs
// set.branch.all. X-Branch, when it is theirs, is where the log's lines go.
func allowed(c fiber.Ctx, op *contract.Operation, p *entity.Principal) error {
	if op.IsPublic() {
		return nil
	}

	if p == nil {
		return entity.ErrUnauthorized
	}

	if actions := op.Actions(); len(actions) > 0 && !p.CanAny(actions) {
		return entity.ErrForbidden
	}

	if op.BranchScoped != "false" && !mayRead(p, c.Query("workspace")) {
		return entity.ErrNotMember
	}

	if b := c.Get("X-Branch"); b != "" && p.Works(b) {
		c.Locals(_localBranch, b)
	}

	return nil
}

// mayRead says whether the person may read the workspace asked: none, one of theirs, or "all" with set.branch.all.
func mayRead(p *entity.Principal, ws string) bool {
	switch ws {
	case "":
		return true
	case "all":
		return p.Can(entity.ActionAllBranches)
	default:
		return p.Works(ws)
	}
}

// validBody refuses a body its schema doesn't allow, on the field it names.
func (a *API) validBody(c fiber.Ctx, op *contract.Operation) error {
	if op.Body == nil {
		return nil
	}

	body := c.Body()
	if len(body) == 0 {
		body = []byte("{}")
	}

	err := a.d.Contract.ValidateJSON(op.Body, body)

	if schemaErr, ok := errors.AsType[*contract.SchemaError](err); ok {
		if schemaErr.Reason == "is required" {
			return entity.Invalid(schemaErr.Field(), "This is needed.")
		}

		return entity.Invalid(schemaErr.Field(), "This value isn't accepted here.")
	}

	return err
}

// mask hides what x-masks says the caller may not see, in the answer itself.
func (a *API) mask(c fiber.Ctx, op *contract.Operation) error {
	body := c.Response().Body()
	if len(op.Masks) == 0 || len(body) == 0 {
		return nil
	}

	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return err //nolint:wrapcheck // the answer is this package's own, so a 500 is right
	}

	p := principalOf(c)
	has := func(action string) bool { return p != nil && p.Can(action) }

	out, err := json.Marshal(a.d.Contract.Mask(op, v, has))
	if err != nil {
		return err //nolint:wrapcheck // as above
	}

	c.Response().SetBody(out)

	return nil
}

// principalOf is the caller the guard found, or one a handler named (signIn).
func principalOf(c fiber.Ctx) *entity.Principal {
	if p, ok := c.Locals(_localPrincipal).(*entity.Principal); ok {
		return p
	}

	return nil
}

// localString is a string a guard or handler left for later, or "".
func localString(c fiber.Ctx, key string) string {
	if s, ok := c.Locals(key).(string); ok {
		return s
	}

	return ""
}
