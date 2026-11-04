package clientapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/gofiber/fiber/v3"
)

// user is the contract's User: who is signed in and what their group grants.
type user struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Email         string   `json:"email"`
	Role          string   `json:"role"`
	SessionEndsAt string   `json:"sessionEndsAt,omitempty"`
	Group         string   `json:"group,omitempty"`
	GroupName     string   `json:"groupName,omitempty"`
	Grants        []string `json:"grants"`
	Doctor        bool     `json:"doctor"`
}

func userOf(p *entity.Principal) user {
	u := user{ID: p.UserID, Name: p.Name, Email: p.Email, Role: p.Role, Group: p.Group, GroupName: p.GroupName, Grants: p.Grants, Doctor: p.Doctor}
	if u.Grants == nil {
		u.Grants = []string{}
	}

	if !p.SessionEndsAt.IsZero() {
		u.SessionEndsAt = p.SessionEndsAt.UTC().Format(time.RFC3339)
	}

	return u
}

func (a *API) me(c fiber.Ctx) error {
	return c.JSON(userOf(principalOf(c)))
}

func (a *API) signIn(c fiber.Ctx) error {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return a.refuse(c, entity.ErrInvalid)
	}

	p, err := a.d.Session.SignIn(c.Context(), body.Email, body.Password)
	if err != nil {
		return a.refuse(c, err)
	}

	return a.startSession(c, &p)
}

// refresh is a longer session for the person the refresh cookie names (the guard read it).
func (a *API) refresh(c fiber.Ctx) error {
	return a.startSession(c, principalOf(c))
}

// startSession sets the session cookie and a new refresh cookie, which
// only the refresh path is sent; both httpOnly, so no script reads them.
func (a *API) startSession(c fiber.Ctx, p *entity.Principal) error {
	now := time.Now()

	access, err := a.d.Access.GenerateToken(p.UserID)
	if err != nil {
		return a.refuse(c, fmt.Errorf("clientapi - startSession - Access: %w", err))
	}

	refresh, err := a.d.Refresh.GenerateToken(p.UserID)
	if err != nil {
		return a.refuse(c, fmt.Errorf("clientapi - startSession - Refresh: %w", err))
	}

	p.SessionEndsAt = now.Add(a.d.Cookies.SessionLength)
	a.cookie(c, _sessionCookie, access, "/", p.SessionEndsAt)
	a.cookie(c, _refreshCookie, refresh, a.refreshPath(), now.Add(a.d.Cookies.RefreshLength))
	c.Locals(_localPrincipal, p)

	return c.JSON(userOf(p))
}

func (a *API) signOut(c fiber.Ctx) error {
	gone := time.Unix(0, 0)
	a.cookie(c, _sessionCookie, "", "/", gone)
	a.cookie(c, _refreshCookie, "", a.refreshPath(), gone)

	return c.SendStatus(http.StatusNoContent)
}

func (a *API) refreshPath() string { return a.d.Cookies.Prefix + "/session/refresh" }

func (a *API) cookie(c fiber.Ctx, name, value, path string, expires time.Time) {
	c.Cookie(&fiber.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		Expires:  expires,
		HTTPOnly: true,
		Secure:   a.d.Cookies.Secure,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}
