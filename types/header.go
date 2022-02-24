package types

import (
	"encoding/json"
	"time"
)

type Header struct {
	GameVersion            string    `json:"gameVersion"`
	CodeVersion            int       `json:"codeVersion"`
	Timestamp              time.Time `json:"timestamp"`
	MatchType              MatchType `json:"matchType"`
	Map                    Map       `json:"map"`
	RecordingPlayerID      string    `json:"recordingPlayerID"`
	AdditionalTags         string    `json:"additionalTags"`
	GameMode               GameMode  `json:"gamemode"`
	RoundsPerMatch         int       `json:"roundsPerMatch"`
	RoundsPerMatchOvertime int       `json:"roundsPerMatchOvertime"`
	RoundNumber            int       `json:"roundNumber"`
	OvertimeRoundNumber    int       `json:"overtimeRoundNumber"`
	Teams                  [2]Team   `json:"teams"`
	Players                []Player  `json:"players"`
	GMSettings             []int     `json:"gmSettings"`
	PlaylistCategory       int       `json:"playlistCategory,omitempty"`
	MatchID                string    `json:"matchID"`
}

type Team struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
}

type Player struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	TeamIndex    int    `json:"teamIndex"`
	HeroName     int    `json:"heroName"`
	Alliance     int    `json:"alliance"`
	RoleImage    int    `json:"roleImage"`
	RoleName     string `json:"roleName"`
	RolePortrait int    `json:"rolePortrait"`
}

type stringerIntMarshal struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

type MatchType int
type GameMode int
type Map int

//go:generate stringer -type=MatchType
//go:generate stringer -type=GameMode
//go:generate stringer -type=Map
const (
	QUICK_MATCH        MatchType = 1
	RANKED             MatchType = 2
	CUSTOM_GAME_LOCAL  MatchType = 7
	CUSTOM_GAME_ONLINE MatchType = 8
	UNRANKED           MatchType = 12

	BOMB        GameMode = 327933806
	SECURE_AREA GameMode = 1983085217
	HOSTAGE     GameMode = 2838806006

	CLUB_HOUSE         Map = 837214085
	KAFE_DOSTOYEVSKY   Map = 1378191338
	KANAL              Map = 1460220617
	YACHT              Map = 1767965020
	PRESIDENTIAL_PLANE Map = 2609218856
	CONSULATE          Map = 2609221242
	BARTLETT_U         Map = 2697268122
	COASTLINE          Map = 42090092951
	TOWER              Map = 53627213396
	VILLA              Map = 88107330328
	FORTRESS           Map = 126196841359
	HEREFORD_BASE      Map = 127951053400
	THEME_PARK         Map = 199824623654
	OREGON             Map = 231702797556
	HOUSE              Map = 237873412352
	CHALET             Map = 259816839773
	SKYSCRAPER         Map = 276279025182
	BORDER             Map = 305979357167
	FAVELA             Map = 329867321446
	BANK               Map = 355496559878
	OUTBACK            Map = 362605108559
)

func (i MatchType) MarshalJSON() (text []byte, err error) {
	return json.Marshal(stringerIntMarshal{
		Name: i.String(),
		ID:   int(i),
	})
}

func (i GameMode) MarshalJSON() (text []byte, err error) {
	return json.Marshal(stringerIntMarshal{
		Name: i.String(),
		ID:   int(i),
	})
}

func (i Map) MarshalJSON() (text []byte, err error) {
	return json.Marshal(stringerIntMarshal{
		Name: i.String(),
		ID:   int(i),
	})
}
