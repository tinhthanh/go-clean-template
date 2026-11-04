package entity

// AuditLine is a line of the log, which is only ever added to: who did what
// to which record, at which branch (the X-Branch the call carried, when it
// is one of the person's).
type AuditLine struct {
	ActorID   string
	ActorName string
	Branch    string
	Action    string
	Resource  string
	Target    string
	Rows      int
	Changes   []Change
}

// Change is one field's old and new value, as a line of the log keeps it.
type Change struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}
