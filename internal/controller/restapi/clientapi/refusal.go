package clientapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/gofiber/fiber/v3"
)

// errCSRF is a write without the client's X-Requested-With: a form or a
// script on another site can't send it without CORS allowing that.
var errCSRF = errors.New("a write without X-Requested-With")

// apiError is the contract's Error: words for people, a code for the app,
// and with invalid the field the form puts the words on.
type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
	Field string `json:"field,omitempty"`
}

// answer is a refusal's status and body. The raw error never reaches the
// client: it is logged, for a 500.
func answer(err error) (int, apiError) {
	var field *entity.FieldError

	switch {
	case errors.As(err, &field):
		return http.StatusBadRequest, apiError{Error: field.Message, Code: "invalid", Field: field.Field}
	case errors.Is(err, entity.ErrInvalid):
		return http.StatusBadRequest, apiError{Error: "Check what was entered.", Code: "invalid"}
	case errors.Is(err, entity.ErrUnauthorized):
		return http.StatusUnauthorized, apiError{Error: "Sign in to go on.", Code: "unauthorized"}
	case errors.Is(err, entity.ErrNotMember):
		return http.StatusForbidden, apiError{Error: "This belongs to a branch you don't work in.", Code: "not_member"}
	case errors.Is(err, entity.ErrForbidden):
		return http.StatusForbidden, apiError{Error: "You don't have permission to do this.", Code: "forbidden"}
	case errors.Is(err, errCSRF):
		return http.StatusForbidden, apiError{Error: "Refused: this has to come from the app.", Code: "forbidden"}
	case errors.Is(err, entity.ErrNotFound):
		return http.StatusNotFound, apiError{Error: "It isn't there any more.", Code: "not_found"}
	case errors.Is(err, entity.ErrNotBuilt):
		return http.StatusNotImplemented, apiError{Error: "This isn't built on the server yet.", Code: "unavailable"}
	default:
		return http.StatusInternalServerError, apiError{Error: "Something went wrong on the server.", Code: "internal"}
	}
}

func (a *API) refuse(c fiber.Ctx, err error) error {
	status, body := answer(err)
	if status >= http.StatusInternalServerError {
		a.d.Logger.Error(fmt.Errorf("clientapi - %s %s: %w", c.Method(), c.Path(), err))
	}

	return c.Status(status).JSON(body)
}

func (a *API) notBuilt(c fiber.Ctx) error { return a.refuse(c, entity.ErrNotBuilt) }
