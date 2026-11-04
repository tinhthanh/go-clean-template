package entity

import (
	"errors"
	"fmt"
)

// The refusals the client's contract names (its Error.code): each
// controller maps them onto a status and the code, never the text.
var (
	ErrUnauthorized = errors.New("not signed in")
	ErrForbidden    = errors.New("the person's group may not do this")
	ErrNotMember    = errors.New("a branch the person does not work in")
	ErrNotFound     = errors.New("no such record")
	ErrInvalid      = errors.New("invalid")
	ErrNotBuilt     = errors.New("not built yet")
)

// FieldError is ErrInvalid on one field: the client puts Message on it.
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }

// Unwrap makes errors.Is(err, ErrInvalid) true.
func (e *FieldError) Unwrap() error { return ErrInvalid }

// Invalid is a refusal of one field's value, in words for people.
func Invalid(field, message string) error { return &FieldError{Field: field, Message: message} }
