package reader

import (
	"bytes"
	"strconv"
	"time"

	"github.com/redraskal/r6-dissect/types"
	"github.com/rs/zerolog/log"
)

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

func (r *DissectReader) readHeader() (types.Header, error) {
	props := make(map[string]string)
	gmSettings := make([]int, 0)
	players := make([]types.Player, 0)
	// Loops until the last property is mapped.
	currentPlayer := types.Player{}
	playerData := false
	for lastProp := false; !lastProp; {
		k, err := r.readHeaderString()
		if err != nil {
			return types.Header{}, err
		}
		v, err := r.readHeaderString()
		if err != nil {
			return types.Header{}, err
		}
		if k == "playerid" {
			if playerData {
				players = append(players, currentPlayer)
			}
			playerData = true
			currentPlayer = types.Player{}
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
					return types.Header{}, err
				}
				gmSettings = append(gmSettings, n)
			}
		} else {
			switch k {
			case "playerid":
				currentPlayer.ID = v
			case "playername":
				currentPlayer.Username = v
			case "team":
				n, err := strconv.Atoi(v)
				if err != nil {
					return types.Header{}, err
				}
				currentPlayer.TeamIndex = n
			case "heroname":
				n, err := strconv.Atoi(v)
				if err != nil {
					return types.Header{}, err
				}
				currentPlayer.HeroName = n
			case "alliance":
				n, err := strconv.Atoi(v)
				if err != nil {
					return types.Header{}, err
				}
				currentPlayer.Alliance = n
			case "roleimage":
				n, err := strconv.Atoi(v)
				if err != nil {
					return types.Header{}, err
				}
				currentPlayer.RoleImage = n
			case "rolename":
				currentPlayer.RoleName = v
			case "roleportrait":
				n, err := strconv.Atoi(v)
				if err != nil {
					return types.Header{}, err
				}
				currentPlayer.RolePortrait = n
			}
		}
		_, lastProp = props["teamscore1"]
	}
	h := types.Header{
		Teams:      [2]types.Team{},
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
	h.MatchType = types.MatchType(n)
	// Parse map
	n, err = strconv.Atoi(props["worldid"])
	if err != nil {
		return h, err
	}
	h.Map = types.Map(n)
	// Add recording player id
	h.RecordingPlayerID = props["recordingplayerid"]
	h.RecordingProfileID = props["recordingprofileid"]
	// Add additional tags
	h.AdditionalTags = props["additionaltags"]
	// Parse game mode
	n, err = strconv.Atoi(props["gamemodeid"])
	if err != nil {
		return h, err
	}
	h.GameMode = types.GameMode(n)
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
	n, err = strconv.Atoi(props["playlistcategory"])
	if err != nil {
		log.Debug().Err(err).Msg("omitting playlistcategory")
	}
	h.PlaylistCategory = n
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
