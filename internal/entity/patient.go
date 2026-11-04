package entity

import "time"

// Patient is a clinic's record of a person: the clinic's, not a branch's
// (Workspace is where it was made). Money and history fields are the
// modules' that keep them; this one has none.
type Patient struct {
	ID         string
	Code       string
	Workspace  string
	Name       string
	Gender     string
	Birth      string
	Phone      string
	CCCD       string
	Email      string
	Address    string
	Occupation string
	Note       string
	Tags       []string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// PatientQuery is what a list asks: the filters the client sends, a cursor and a size.
type PatientQuery struct {
	// Workspaces limit the rows to those made at these; nil is every one.
	MadeAt []string
	Q      string
	Tag    string
	// Created is a day at the clinic (Asia/Ho_Chi_Minh), "2006-01-02".
	Created string
	Sort    string
	Desc    bool
	// PhoneDigitsOnly: the reader sees phones masked, so digits match only a phone's last three.
	PhoneDigitsOnly bool
	Offset          uint64
	Limit           uint64
}

// PatientPage is one page and what the server worked out over every match.
type PatientPage struct {
	Items  []Patient
	Total  int
	Male   int
	Female int
}

// PatientPatch is the fields a change sends; nil is left as it is.
type PatientPatch struct {
	Name       *string
	Phone      *string
	Gender     *string
	Birth      *string
	CCCD       *string
	Email      *string
	Address    *string
	Occupation *string
	Note       *string
}
