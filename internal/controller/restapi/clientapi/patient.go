package clientapi

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/gofiber/fiber/v3"
)

// patient is the contract's Patient. Money, history and the alert line are
// the modules' that keep them; flags is always there, empty until one does.
type patient struct {
	ID         string   `json:"id"`
	Code       string   `json:"code"`
	Workspace  string   `json:"workspace"`
	Name       string   `json:"name"`
	Gender     string   `json:"gender,omitempty"`
	Birth      string   `json:"birth,omitempty"`
	Phone      string   `json:"phone"`
	CCCD       string   `json:"cccd,omitempty"`
	Email      string   `json:"email,omitempty"`
	Address    string   `json:"address,omitempty"`
	Occupation string   `json:"occupation,omitempty"`
	Note       string   `json:"note,omitempty"`
	Tags       []string `json:"tags"`
	Flags      []string `json:"flags"`
	CreatedAt  string   `json:"createdAt"`
	UpdatedAt  string   `json:"updatedAt"`
}

func patientOf(p *entity.Patient) patient {
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}

	return patient{
		ID: p.ID, Code: p.Code, Workspace: p.Workspace, Name: p.Name, Gender: p.Gender, Birth: p.Birth, Phone: p.Phone,
		CCCD: p.CCCD, Email: p.Email, Address: p.Address, Occupation: p.Occupation, Note: p.Note, Tags: tags, Flags: []string{},
		CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: p.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// patientPage is the contract's Page of patients, with its summary over every match.
type patientPage struct {
	Items   []patient `json:"items"`
	Next    string    `json:"next,omitempty"`
	Total   int       `json:"total"`
	Summary struct {
		Male   int `json:"male"`
		Female int `json:"female"`
	} `json:"summary"`
}

func (a *API) patList(c fiber.Ctx) error {
	p := principalOf(c)

	q, err := patientQuery(c, p)
	if err != nil {
		return a.refuse(c, err)
	}

	page, err := a.d.Patient.List(c.Context(), p, &q)
	if err != nil {
		return a.refuse(c, err)
	}

	out := patientPage{Items: make([]patient, 0, len(page.Items)), Total: page.Total}
	out.Summary.Male, out.Summary.Female = page.Male, page.Female

	for i := range page.Items {
		out.Items = append(out.Items, patientOf(&page.Items[i]))
	}

	if end := q.Offset + uint64(len(page.Items)); end < uint64(page.Total) { //nolint:gosec // counts, never negative
		out.Next = cursorOf(end)
	}

	return c.JSON(out)
}

// patientQuery reads the list's query: madeAt is one of the person's
// branches, or all of theirs.
func patientQuery(c fiber.Ctx, p *entity.Principal) (entity.PatientQuery, error) {
	q := entity.PatientQuery{Q: strings.TrimSpace(c.Query("q")), Tag: c.Query("tag"), Created: c.Query("created")}

	switch made := c.Query("madeAt"); made {
	case "":
	case "all":
		q.MadeAt = p.Workspaces
	default:
		q.MadeAt = []string{made}
	}

	if q.Created != "" {
		if _, err := time.Parse(time.DateOnly, q.Created); err != nil {
			return q, entity.Invalid("created", "A day is written 2006-01-02.")
		}
	}

	q.Sort, q.Desc = sortOf(c.Query("sort"))

	offset, err := offsetOf(c.Query("cursor"))
	if err != nil {
		return q, entity.Invalid("cursor", "Start the list again.")
	}

	q.Offset = offset

	if l := c.Query("limit"); l != "" {
		n, convErr := strconv.ParseUint(l, 10, 32)
		if convErr != nil || n == 0 {
			return q, entity.Invalid("limit", "A page holds at least one.")
		}

		q.Limit = n
	}

	return q, nil
}

func sortOf(s string) (string, bool) {
	field, dir, _ := strings.Cut(s, ":")

	return field, dir == "desc"
}

// A cursor is opaque to the client: where the next page starts.
func cursorOf(offset uint64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatUint(offset, 10)))
}

func offsetOf(cursor string) (uint64, error) {
	if cursor == "" {
		return 0, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, err //nolint:wrapcheck // turned into an invalid field by the caller
	}

	return strconv.ParseUint(string(raw), 10, 64) //nolint:wrapcheck // as above
}

func (a *API) patGet(c fiber.Ctx) error {
	pat, err := a.d.Patient.Get(c.Context(), principalOf(c), c.Params("id"))
	if err != nil {
		return a.refuse(c, err)
	}

	return c.JSON(patientOf(&pat))
}

// patientInput is the create and change bodies (the guard held them to the contract).
type patientInput struct {
	Name       *string `json:"name"`
	Phone      *string `json:"phone"`
	Gender     *string `json:"gender"`
	Birth      *string `json:"birth"`
	CCCD       *string `json:"cccd"`
	Email      *string `json:"email"`
	Address    *string `json:"address"`
	Occupation *string `json:"occupation"`
	Note       *string `json:"note"`
	Workspace  *string `json:"workspace"`
}

func (in *patientInput) patch() *entity.PatientPatch {
	return &entity.PatientPatch{
		Name: in.Name, Phone: in.Phone, Gender: in.Gender, Birth: in.Birth, CCCD: in.CCCD,
		Email: in.Email, Address: in.Address, Occupation: in.Occupation, Note: in.Note,
	}
}

func (a *API) patCreate(c fiber.Ctx) error {
	var in patientInput
	if err := json.Unmarshal(c.Body(), &in); err != nil {
		return a.refuse(c, entity.ErrInvalid)
	}

	// Made at the branch the body names, else the one the person acts from.
	var pat entity.Patient
	if in.Workspace != nil {
		pat.Workspace = *in.Workspace
	} else {
		pat.Workspace = localString(c, _localBranch)
	}

	applyInput(&pat, in.patch())

	made, err := a.d.Patient.Create(c.Context(), principalOf(c), &pat)
	if err != nil {
		return a.refuse(c, err)
	}

	return c.JSON(patientOf(&made))
}

func (a *API) patUpdate(c fiber.Ctx) error {
	var in patientInput
	if err := json.Unmarshal(c.Body(), &in); err != nil {
		return a.refuse(c, entity.ErrInvalid)
	}

	pat, changes, err := a.d.Patient.Update(c.Context(), principalOf(c), c.Params("id"), in.patch())
	if err != nil {
		return a.refuse(c, err)
	}

	c.Locals(_localChanges, changes)

	return c.JSON(patientOf(&pat))
}

func applyInput(pat *entity.Patient, in *entity.PatientPatch) {
	for _, f := range []struct {
		from *string
		to   *string
	}{
		{in.Name, &pat.Name},
		{in.Phone, &pat.Phone},
		{in.Gender, &pat.Gender},
		{in.Birth, &pat.Birth},
		{in.CCCD, &pat.CCCD},
		{in.Email, &pat.Email},
		{in.Address, &pat.Address},
		{in.Occupation, &pat.Occupation},
		{in.Note, &pat.Note},
	} {
		if f.from != nil {
			*f.to = *f.from
		}
	}
}
