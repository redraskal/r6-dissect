package dissect

import (
	"bytes"
	"encoding/json"
	"github.com/rs/zerolog/log"
	"strconv"
	"time"
)

type Header struct {
	GameVersion            string    `json:"gameVersion"`
	CodeVersion            int       `json:"codeVersion"`
	Timestamp              time.Time `json:"timestamp"`
	MatchType              MatchType `json:"matchType"`
	Map                    Map       `json:"map"`
	RecordingPlayerID      uint64    `json:"recordingPlayerID"`
	RecordingProfileID     string    `json:"recordingProfileID"`
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
	ID           uint64 `json:"id"`
	ProfileID    string `json:"profileID"` // Ubisoft stats identifier
	Username     string `json:"username"`
	TeamIndex    int    `json:"teamIndex"`
	HeroName     int    `json:"heroName"`
	Alliance     int    `json:"alliance"`
	RoleImage    int    `json:"roleImage"`
	RoleName     string `json:"roleName"`
	RolePortrait int    `json:"rolePortrait"`
	id           []byte // dissect player id at end of packet (4 bytes)
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
	EMERALD_PLAINS     Map = 365284490964
	STADIUM_BRAVO      Map = 270063334510
	NIGHTHAVEN_LABS    Map = 378595635123
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

func (h Header) RecordingPlayer() Player {
	for _, val := range h.Players {
		if val.ID == h.RecordingPlayerID {
			return val
		}
	}
	return Player{}
}

// readHeaderMagic reads the header magic of the reader
// and validates the dissect format.
// If there is an error, it will be of type *ErrInvalidFile.
func (r *DissectReader) readHeaderMagic() error {
	// Checks for the dissect header.
	b, err := r.read(7)
	if err != nil {
		return err
	}
	if string(b[:7]) != "dissect" {
		return ErrInvalidFile
	}
	// Skips to the end of the unknown dissect versioning scheme.
	// Probably will be replaced later when more info is uncovered.
	// We are skipping to the end of the second sequence of 7 0x00 bytes
	// where the string values are stored.
	b = make([]byte, 1)
	n := 0
	t := 0
	for t != 2 {
		len, err := r.compressed.Read(b)
		r.offset += len
		if err != nil {
			return err
		}
		if len != 1 {
			return ErrInvalidFile
		}
		if b[0] == 0x00 {
			if n != 6 {
				n++
			} else {
				n = 0
				t++
			}
		} else if n > 0 {
			n = 0
		}
	}
	return nil
}

func (r *DissectReader) readHeader() (Header, error) {
	props := make(map[string]string)
	gmSettings := make([]int, 0)
	players := make([]Player, 0)
	// Loops until the last property is mapped.
	currentPlayer := Player{}
	playerData := false
	for lastProp := false; !lastProp; {
		k, err := r.readHeaderString()
		if err != nil {
			return Header{}, err
		}
		v, err := r.readHeaderString()
		if err != nil {
			return Header{}, err
		}
		if k == "playerid" {
			if playerData {
				players = append(players, currentPlayer)
			}
			playerData = true
			currentPlayer = Player{}
		}
		if (k == "playlistcategory" || k == "id") && playerData {
			players = append(players, currentPlayer)
			playerData = false
		}
		if !playerData {
			if k != "gmsetting" {
				props[k] = v
			} else {
				n, err := strconv.Atoi(v)
				if err != nil {
					return Header{}, err
				}
				gmSettings = append(gmSettings, n)
			}
		} else {
			switch k {
			case "playerid":
				n, err := strconv.ParseUint(v, 10, 64)
				if err != nil {
					return Header{}, err
				}
				currentPlayer.ID = n
			case "playername":
				currentPlayer.Username = v
			case "team":
				n, err := strconv.Atoi(v)
				if err != nil {
					return Header{}, err
				}
				currentPlayer.TeamIndex = n
			case "heroname":
				n, err := strconv.Atoi(v)
				if err != nil {
					return Header{}, err
				}
				currentPlayer.HeroName = n
			case "alliance":
				n, err := strconv.Atoi(v)
				if err != nil {
					return Header{}, err
				}
				currentPlayer.Alliance = n
			case "roleimage":
				n, err := strconv.Atoi(v)
				if err != nil {
					return Header{}, err
				}
				currentPlayer.RoleImage = n
			case "rolename":
				currentPlayer.RoleName = v
			case "roleportrait":
				n, err := strconv.Atoi(v)
				if err != nil {
					return Header{}, err
				}
				currentPlayer.RolePortrait = n
			}
		}
		_, lastProp = props["teamscore1"]
	}
	h := Header{
		Teams:      [2]Team{},
		Players:    players,
		GMSettings: gmSettings,
	}
	// Parse game version
	h.GameVersion = props["version"]
	// Parse code version
	n, err := strconv.Atoi(props["code"])
	if err != nil {
		return h, err
	}
	h.CodeVersion = n
	// Parse timestamp
	t, err := time.Parse("2006-01-02-15-04-05", props["datetime"])
	if err != nil {
		return h, err
	}
	h.Timestamp = t
	// Parse match type
	n, err = strconv.Atoi(props["matchtype"])
	if err != nil {
		return h, err
	}
	h.MatchType = MatchType(n)
	// Parse map
	n, err = strconv.Atoi(props["worldid"])
	if err != nil {
		return h, err
	}
	h.Map = Map(n)
	// Add recording player id
	u, err := strconv.ParseUint(props["recordingplayerid"], 10, 64)
	if err != nil {
		return h, err
	}
	h.RecordingPlayerID = u
	h.RecordingProfileID = props["recordingprofileid"]
	// Add additional tags
	h.AdditionalTags = props["additionaltags"]
	// Parse game mode
	n, err = strconv.Atoi(props["gamemodeid"])
	if err != nil {
		return h, err
	}
	h.GameMode = GameMode(n)
	// Parse rounds per match
	n, err = strconv.Atoi(props["roundspermatch"])
	if err != nil {
		return h, err
	}
	h.RoundsPerMatch = n
	// Parse rounds per match overtime
	n, err = strconv.Atoi(props["roundspermatchovertime"])
	if err != nil {
		return h, err
	}
	h.RoundsPerMatchOvertime = n
	// Parse round number
	n, err = strconv.Atoi(props["roundnumber"])
	if err != nil {
		return h, err
	}
	h.RoundNumber = n
	// Parse overtime round number
	n, err = strconv.Atoi(props["overtimeroundnumber"])
	if err != nil {
		return h, err
	}
	h.OvertimeRoundNumber = n
	// Add team names
	h.Teams[0].Name = props["teamname0"]
	h.Teams[1].Name = props["teamname1"]
	// Add playlist category
	if len(props["playlistcategory"]) > 0 {
		n, err = strconv.Atoi(props["playlistcategory"])
		if err != nil {
			log.Debug().Err(err).Msg("omitting playlistcategory")
		}
		h.PlaylistCategory = n
	}
	// Add match id
	h.MatchID = props["id"]
	// Parse team scores
	n, err = strconv.Atoi(props["teamscore0"])
	if err != nil {
		return h, err
	}
	h.Teams[0].Score = n
	n, err = strconv.Atoi(props["teamscore1"])
	if err != nil {
		return h, err
	}
	h.Teams[1].Score = n
	return h, nil
}

func (r *DissectReader) readHeaderString() (string, error) {
	b, err := r.read(1)
	if err != nil {
		return "", err
	}
	len := int(b[0])
	b, err = r.read(7)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(b, strSep) {
		return "", ErrInvalidStringSep
	}
	b, err = r.read(len)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
