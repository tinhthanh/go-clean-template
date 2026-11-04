package entity

import (
	"slices"
	"time"
)

// The action that lets a person read every branch at once ("all").
const ActionAllBranches = "set.branch.all"

// Principal is who is signed in, as every guard and use case reads them:
// their group's grants (action keys, the contract's x-permission words) and
// the workspaces (branches) they work in. Workspaces nil is every one, as
// the owner's.
type Principal struct {
	UserID        string
	Name          string
	Email         string
	Role          string
	Group         string
	GroupName     string
	Grants        []string
	Doctor        bool
	Workspaces    []string
	SessionEndsAt time.Time
}

// Can says whether the person's group grants the action.
func (p *Principal) Can(action string) bool { return slices.Contains(p.Grants, action) }

// CanAny says whether it grants any one of the actions.
func (p *Principal) CanAny(actions []string) bool { return slices.ContainsFunc(actions, p.Can) }

// Works says whether the person works in the workspace.
func (p *Principal) Works(workspace string) bool {
	return p.Workspaces == nil || slices.Contains(p.Workspaces, workspace)
}
