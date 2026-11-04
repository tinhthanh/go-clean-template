package contract

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"unicode"
)

// The ways x-masks hides a field.
const (
	MaskMasked = "masked"
	MaskAbsent = "absent"
	MaskEquals = "equals"
)

const (
	_shownDigits      = 3 // a masked number keeps its last three, as the client's mask does
	_phoneShownFirst  = 4 // and a phone its first four
	_shortPhoneDigits = 7
	_shortPhoneShown  = 2
)

// Mask applies the operation's x-masks to a decoded answer, in place, for a
// reader whose grants are has: each mask whose unless the reader has none of.
// A masked field's object gets <field>Masked: true where its schema has one
// (Patient.phoneMasked), so the client says why it can't read it.
func (c *Contract) Mask(op *Operation, answer any, has func(action string) bool) any {
	root := op.Responses["200"]

	for i := range op.Masks {
		m := &op.Masks[i]
		if slices.ContainsFunc(m.Unless, has) {
			continue
		}

		segments := strings.Split(m.Path, ".")
		answer = c.walk(answer, root, segments, m)
	}

	return answer
}

// walk follows the path's segments ("items[]", "changes[]", "phone"), the
// schema beside it, and hides the last one.
func (c *Contract) walk(v any, schema json.RawMessage, segments []string, m *Mask) any {
	name, array := strings.CutSuffix(segments[0], "[]")
	rest := segments[1:]

	if name == "" && array {
		return c.each(v, c.items(schema), rest, m)
	}

	obj, ok := v.(map[string]any)
	if !ok || !matches(obj, m) {
		return v
	}

	child := c.property(schema, name)

	switch {
	case array:
		if inner, ok := obj[name]; ok {
			obj[name] = c.each(inner, c.items(child), rest, m)
		}
	case len(rest) > 0:
		if inner, ok := obj[name]; ok {
			obj[name] = c.walk(inner, child, rest, m)
		}
	default:
		c.hide(obj, name, schema, m)
	}

	return obj
}

func (c *Contract) each(v any, item json.RawMessage, rest []string, m *Mask) any {
	list, ok := v.([]any)
	if !ok {
		return v
	}

	for i, el := range list {
		if len(rest) == 0 {
			continue
		}

		list[i] = c.walk(el, item, rest, m)
	}

	return list
}

// matches says whether an object on the path is one the mask is for: its
// where fields, and for a change line its field in Fields.
func matches(obj map[string]any, m *Mask) bool {
	for k, want := range m.Where {
		if got, ok := obj[k]; ok && got != want {
			return false
		}
	}

	if f, ok := obj["field"].(string); ok && len(m.Fields) > 0 {
		return slices.Contains(m.Fields, f)
	}

	return true
}

func (c *Contract) hide(obj map[string]any, name string, schema json.RawMessage, m *Mask) {
	value, ok := obj[name]
	if !ok {
		return
	}

	switch m.As {
	case MaskAbsent:
		delete(obj, name)
	case MaskEquals:
		obj[name] = m.Value
	default:
		s, isString := value.(string)
		if !isString || s == "" {
			return
		}

		// A change line ({field, from, to}) is masked as the field it is about.
		field := name
		if f, ok := obj["field"].(string); ok {
			field = f
		}

		obj[name] = Masked(field, s)

		if c.property(schema, name+"Masked") != nil {
			obj[name+"Masked"] = true
		}
	}
}

// Masked is a value as a reader without the right sees it: a phone as
// "0901 *** 567", any other number or text all but its last three.
func Masked(field, s string) string {
	digits := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}

		return -1
	}, s)

	if field == "phone" || strings.HasSuffix(field, "Phone") {
		if len(digits) < _shortPhoneDigits {
			return strings.Repeat("*", max(0, len(digits)-_shortPhoneShown)) + digits[max(0, len(digits)-_shortPhoneShown):]
		}

		return digits[:_phoneShownFirst] + " *** " + digits[len(digits)-_shownDigits:]
	}

	if digits == "" {
		digits = s
	}

	runes := []rune(digits)
	shown := max(0, len(runes)-_shownDigits)

	return strings.Repeat("*", shown) + string(runes[shown:])
}

// resolved is a schema through its $ref and allOf: its properties and items.
type resolved struct {
	props map[string]json.RawMessage
	items json.RawMessage
}

func (c *Contract) resolve(raw json.RawMessage) resolved {
	out := resolved{props: map[string]json.RawMessage{}}

	var n node
	if raw == nil || json.Unmarshal(raw, &n) != nil {
		return out
	}

	if n.Ref != "" {
		return c.resolve(c.schemas[strings.TrimPrefix(n.Ref, _refPrefix)])
	}

	for _, part := range n.AllOf {
		r := c.resolve(part)
		maps.Copy(out.props, r.props)

		if r.items != nil {
			out.items = r.items
		}
	}

	maps.Copy(out.props, n.Properties)

	if n.Items != nil {
		out.items = n.Items
	}

	return out
}

func (c *Contract) property(schema json.RawMessage, name string) json.RawMessage {
	return c.resolve(schema).props[name]
}

func (c *Contract) items(schema json.RawMessage) json.RawMessage {
	return c.resolve(schema).items
}
