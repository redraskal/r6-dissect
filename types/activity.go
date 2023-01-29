package types

import "encoding/json"

type ActivityType int

//go:generate stringer -type=ActivityType
const (
	KILL   ActivityType = iota
	DEATH               // TODO
	PLANT               // TODO
	DEFUSE              // TODO
	LOCATE_OBJECTIVE
	BATTLEYE
	PLAYER_LEAVE
)

type Activity struct {
	Type          ActivityType `json:"type"`
	Username      string       `json:"username,omitempty"`
	Target        string       `json:"target,omitempty"`
	Headshot      *bool        `json:"headshot,omitempty"`
	Time          string       `json:"time"`
	TimeInSeconds int          `json:"timeInSeconds"`
}

func (i ActivityType) MarshalJSON() (text []byte, err error) {
	return json.Marshal(i.String())
}
