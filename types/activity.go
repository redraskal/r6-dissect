package types

import "encoding/json"

type ActivityType int

//go:generate stringer -type=ActivityType
const (
	KILL ActivityType = iota
	DEATH
	LOCATE_OBJECTIVE
)

type Activity struct {
	Type     ActivityType `json:"type"`
	Username string       `json:"username"`
	Target   string       `json:"target,omitempty"`
	Headshot *bool        `json:"headshot,omitempty"`
}

func (i ActivityType) MarshalJSON() (text []byte, err error) {
	return json.Marshal(i.String())
}
