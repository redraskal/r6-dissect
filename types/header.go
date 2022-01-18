package types

import "time"

type Header struct {
	GameVersion            int
	Timestamp              time.Time
	MatchType              MatchType
	Map                    Map
	RecordingPlayerID      string
	AdditionalTags         string
	GameMode               GameMode
	RoundsPerMatch         int
	RoundsPerMatchOvertime int
	RoundNumber            int
	OvertimeRoundNumber    int
	Teams                  [2]Team
	Players                []Player
	GMSettings             []string
	PlaylistCategory       string
	MatchID                string
}

type Team struct {
	Name  string
	Score int
}

type Player struct {
	ID           string
	Username     string
	TeamIndex    int
	HeroName     string
	Alliance     int
	RoleImage    string
	RoleName     string
	RolePortrait string
}

type MatchType int
type GameMode int
type Map int

//go:generate stringer -type=MatchType
//go:generate stringer -type=GameMode
//go:generate stringer -type=Map
const (
	QUICK_MATCH MatchType = 1
	CUSTOM_GAME MatchType = 7
	UNRANKED    MatchType = 12

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
