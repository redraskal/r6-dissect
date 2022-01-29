package reader

import (
	"bytes"
	"io"
	"strconv"
	"time"

	"github.com/redraskal/r6-dissect/types"
	"github.com/rs/zerolog/log"
)

// ReadHeaderStr reads the next string in the header of r.
func ReadHeaderStr(r io.Reader) (string, error) {
	b, err := readBytes(1, r)
	if err != nil {
		return "", err
	}
	len := int(b[0])
	b, err = readBytes(7, r)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(b, strSep) {
		return "", ErrInvalidStringSep
	}
	b, err = readBytes(len, r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ReadHeader reads the header of r.
func ReadHeader(r io.Reader) (types.Header, error) {
	props := make(map[string]string)
	gmSettings := make([]int, 0)
	players := make([]types.Player, 0)
	// Loops until the last property is mapped.
	currentPlayer := types.Player{}
	playerData := false
	for lastProp := false; !lastProp; {
		k, err := ReadHeaderStr(r)
		if err != nil {
			return types.Header{}, err
		}
		v, err := ReadHeaderStr(r)
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
			case "allilance":
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
	n, err := strconv.Atoi(props["version"])
	if err != nil {
		return h, err
	}
	h.GameVersion = n
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

func readBytes(n int, r io.Reader) ([]byte, error) {
	b := make([]byte, n)
	len, err := r.Read(b)
	if err != nil {
		return nil, err
	}
	if len != n {
		return nil, ErrInvalidLength
	}
	return b, nil
}
