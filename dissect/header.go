package dissect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

type Header struct {
	GameVersion            string    `json:"gameVersion"`
	CodeVersion            int       `json:"codeVersion"`
	Timestamp              time.Time `json:"timestamp"`
	MatchType              MatchType `json:"matchType"`
	Map                    Map       `json:"map"`
	Site                   string    `json:"site,omitempty"`
	RecordingPlayerID      uint64    `json:"recordingPlayerID"`
	RecordingProfileID     string    `json:"recordingProfileID,omitempty"`
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
	Name         string       `json:"name"`
	Score        int          `json:"score"`
	Won          bool         `json:"won"`
	WinCondition WinCondition `json:"winCondition,omitempty"`
	Role         TeamRole     `json:"role,omitempty"`
}

type Player struct {
	ID           uint64   `json:"id,omitempty"`
	ProfileID    string   `json:"profileID,omitempty"` // Ubisoft stats identifier
	Username     string   `json:"username"`
	TeamIndex    int      `json:"teamIndex"`
	Operator     Operator `json:"operator"`
	HeroName     int      `json:"heroName,omitempty"`
	Alliance     int      `json:"alliance"`
	RoleImage    int      `json:"roleImage,omitempty"`
	RoleName     string   `json:"roleName,omitempty"`
	RolePortrait int      `json:"rolePortrait,omitempty"`
	Spawn        string   `json:"spawn,omitempty"`
	id           []byte   // dissect player id at end of packet (4 bytes)
}

type stringerIntMarshal struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

type MatchType int
type GameMode int
type Map int
type WinCondition string
type TeamRole string
type Operator uint64

//go:generate stringer -type=MatchType
//go:generate stringer -type=GameMode
//go:generate stringer -type=Map
//go:generate stringer -type=Operator
//go:generate go run ./genops.go -type=Operator -atkval=Attack -defval=Defense
const (
	QuickMatch       MatchType = 1
	Ranked           MatchType = 2
	CustomGameLocal  MatchType = 7
	CustomGameOnline MatchType = 8
	Unranked         MatchType = 12

	Bomb       GameMode = 327933806
	SecureArea GameMode = 1983085217
	Hostage    GameMode = 2838806006

	ClubHouse         Map = 837214085
	KafeDostoyevsky   Map = 1378191338
	Kanal             Map = 1460220617
	Yacht             Map = 1767965020
	PresidentialPlane Map = 2609218856
	Consulate         Map = 2609221242
	BartlettU         Map = 2697268122
	Coastline         Map = 42090092951
	Tower             Map = 53627213396
	Villa             Map = 88107330328
	Fortress          Map = 126196841359
	HerefordBase      Map = 127951053400
	ThemePark         Map = 199824623654
	Oregon            Map = 231702797556
	House             Map = 237873412352
	Chalet            Map = 259816839773
	Skyscraper        Map = 276279025182
	Border            Map = 305979357167
	Favela            Map = 329867321446
	Bank              Map = 355496559878
	Outback           Map = 362605108559
	EmeraldPlains     Map = 365284490964
	StadiumBravo      Map = 270063334510
	NighthavenLabs    Map = 378595635123

	KilledOpponents  WinCondition = "KilledOpponents"
	SecuredArea      WinCondition = "SecuredArea" // TODO
	DisabledDefuser  WinCondition = "DisabledDefuser"
	DefusedBomb      WinCondition = "DefusedBomb"
	ExtractedHostage WinCondition = "ExtractedHostage" // TODO
	Time             WinCondition = "Time"

	Attack  TeamRole = "Attack"
	Defense TeamRole = "Defense"

	Castle      Operator = 92270642682 // May technically refer to the op icon?
	Aruni       Operator = 104189664704
	Kaid        Operator = 161289666230
	Mozzie      Operator = 174977508820
	Pulse       Operator = 92270642708
	Ace         Operator = 104189664390
	Echo        Operator = 92270642214
	Azami       Operator = 378305069945
	Solis       Operator = 391752120891
	Capitao     Operator = 92270644215
	Zofia       Operator = 92270644189
	Dokkaebi    Operator = 92270644267
	Warden      Operator = 104189662920
	Mira        Operator = 92270644319
	Sledge      Operator = 92270642344
	Melusi      Operator = 104189664273
	Bandit      Operator = 92270642526
	Valkyrie    Operator = 92270642188
	Rook        Operator = 92270644059
	Kapkan      Operator = 92270641980
	Zero        Operator = 291191151607
	Iana        Operator = 104189664038
	Ash         Operator = 92270642656
	Blackbeard  Operator = 92270642136
	Osa         Operator = 288200867444
	Thorn       Operator = 373711624351
	Jager       Operator = 92270642604
	Kali        Operator = 104189663920
	Thermite    Operator = 92270642760
	Brava       Operator = 288200866821
	Amaru       Operator = 104189663607
	Ying        Operator = 92270642292
	Lesion      Operator = 92270642266
	Doc         Operator = 92270644007
	Lion        Operator = 104189661861
	Fuze        Operator = 92270642032
	Smoke       Operator = 92270642396
	Vigil       Operator = 92270644293
	Mute        Operator = 92270642318
	Goyo        Operator = 104189663698
	Wamai       Operator = 104189663803
	Ela         Operator = 92270644163
	Montagne    Operator = 92270644033
	Nokk        Operator = 104189663024
	Alibi       Operator = 104189662071
	Finka       Operator = 104189661965
	Caveira     Operator = 92270644241
	Nomad       Operator = 161289666248
	Thunderbird Operator = 288200867351
	Sens        Operator = 384797789346
	IQ          Operator = 92270642578
	Blitz       Operator = 92270642539
	Hibana      Operator = 92270642240
	Maverick    Operator = 104189662384
	Flores      Operator = 328397386974
	Buck        Operator = 92270642474
	Twitch      Operator = 92270644111
	Gridlock    Operator = 174977508808
	Thatcher    Operator = 92270642422
	Glaz        Operator = 92270642084
	Jackal      Operator = 92270644345
	Grim        Operator = 374667788042
	Tachanka    Operator = 291437347686
	Oryx        Operator = 104189664155
	Frost       Operator = 92270642500
	Maestro     Operator = 104189662175
	Clash       Operator = 104189662280
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

func (i Operator) MarshalJSON() (text []byte, err error) {
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

// deriveTeamRoles uses the operators chosen by the players to
// determine the team roles
func (r *DissectReader) deriveTeamRoles() error {
	for _, p := range r.Header.Players {
		if role, err := p.Operator.Role(); err == nil {
			teamIndex := p.TeamIndex
			oppositeTeamIndex := teamIndex ^ 1
			if role == Attack {
				r.Header.Teams[teamIndex].Role = Attack
				r.Header.Teams[oppositeTeamIndex].Role = Defense
			} else {
				r.Header.Teams[teamIndex].Role = Defense
				r.Header.Teams[oppositeTeamIndex].Role = Attack
			}
			return nil
		}
	}
	return fmt.Errorf("could not determine team roles (have %d players)", len(r.Header.Players))
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
