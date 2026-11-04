package clientapi

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/pkg/contract"
	"github.com/gofiber/fiber/v3"
)

const _auditWords = 2 // "<action> <resource>"

var (
	// everyMinutes reads how often a line is written: "one line per person per patient per 10 minutes".
	everyMinutes = regexp.MustCompile(`per (\d+) minutes?`)
	// aside is a note in brackets: "view record (the header)".
	aside = regexp.MustCompile(`\s*\([^)]*\)`)
)

// auditRule is one x-audit line, read: "view record: one line per person
// per patient per 10 minutes" is action view, resource record, at most once
// in ten minutes per person and record. A line that says for whom ("for one
// who reads …") depends on what the handler did, so the handler writes it.
// "the same" is as often as the line before it.
type auditRule struct {
	Action   string
	Resource string
	Within   time.Duration
	Handler  bool
	Same     bool
}

func readAudit(line string) (auditRule, bool) {
	plain := aside.ReplaceAllString(line, "")

	head, rest := plain, ""
	if i := strings.IndexAny(plain, ":,"); i >= 0 {
		head, rest = plain[:i], plain[i+1:]
	}

	words := strings.Fields(head)
	if len(words) != _auditWords {
		return auditRule{}, false
	}

	r := auditRule{Action: words[0], Resource: words[1], Handler: strings.Contains(rest, "for one who"), Same: strings.Contains(rest, "the same")}

	if m := everyMinutes.FindStringSubmatch(rest); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			r.Within = time.Duration(n) * time.Minute
		}
	}

	return r, true
}

// audit writes the operation's lines, by the caller, at the branch they
// act from, on the record the path names (or the one the answer made).
func (a *API) audit(c fiber.Ctx, op *contract.Operation) error {
	p := principalOf(c)
	branch := localString(c, _localBranch)

	var changes []entity.Change
	if ch, ok := c.Locals(_localChanges).([]entity.Change); ok {
		changes = ch
	}

	var last time.Duration

	for _, text := range op.Audit {
		rule, ok := readAudit(text)
		if !ok || rule.Handler {
			continue
		}

		if rule.Same {
			rule.Within = last
		}

		last = rule.Within

		line := &entity.AuditLine{Action: rule.Action, Resource: rule.Resource, Target: target(c), Branch: branch, Changes: changes}
		if p != nil {
			line.ActorID, line.ActorName = p.UserID, p.Name
		}

		var err error
		if rule.Within > 0 {
			err = a.d.Audit.WriteOnce(c.Context(), line, rule.Within)
		} else {
			err = a.d.Audit.Write(c.Context(), line)
		}

		if err != nil {
			return err //nolint:wrapcheck // the use case wraps it
		}
	}

	return nil
}

// target is the record acted on: the path's id, or the id of the one made.
func target(c fiber.Ctx) string {
	if id := c.Params("id"); id != "" {
		return id
	}

	var made struct {
		ID string `json:"id"`
	}

	if json.Unmarshal(c.Response().Body(), &made) != nil {
		return ""
	}

	return made.ID
}
