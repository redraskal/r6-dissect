package dissect

import (
	"bytes"
	"strings"

	"github.com/rs/zerolog/log"
)

var currentSitePattern = []byte{0xFC, 0xC6, 0xA8, 0x60, 0x01}

func readSpawn(r *Reader) error {
	entryOffset := r.offset
	location, err := r.String()
	if err != nil {
		return err
	}
	// Detect room location packet: empty string followed by 3 zero bytes then a room name.
	// Packet structure: [magic] 00 00 00 00 [len] [room_name]
	// The entity byte identifying the player is at entryOffset - 26.
	if location == "" && r.offset+3 < len(r.b) &&
		r.b[r.offset] == 0 && r.b[r.offset+1] == 0 && r.b[r.offset+2] == 0 {
		if err = r.Skip(3); err != nil {
			return err
		}
		roomName, err := r.String()
		if err != nil {
			return err
		}
		if roomName != "" {
			var username string
			if entryOffset >= 27 && r.b[entryOffset-27] == 0x23 {
				entityByte := r.b[entryOffset-26]
				if _, ok := r.roomEntityToPlayer[entityByte]; !ok && len(r.roomEntityOrder) < len(r.Header.Players) {
					playerIdx := len(r.roomEntityOrder)
					r.roomEntityToPlayer[entityByte] = playerIdx
					r.roomEntityOrder = append(r.roomEntityOrder, entityByte)
				}
				if playerIdx, ok := r.roomEntityToPlayer[entityByte]; ok && playerIdx < len(r.Header.Players) {
					username = r.Header.Players[playerIdx].Username
				}
			}
			r.LocationEvents = append(r.LocationEvents, LocationEvent{
				Username:      username,
				Room:          roomName,
				TimeInSeconds: r.time,
			})
			log.Debug().
				Str("room", roomName).
				Str("player", username).
				Float64("time", r.time).
				Msg("room_location")
		}
		return nil
	}
	if err = r.Skip(150); err != nil {
		return err
	}
	pattern, err := r.Bytes(5)
	if err != nil {
		return err
	}
	if !strings.Contains(location, "<br/>") {
		return nil
	}
	log.Debug().
		Str("site", location).
		Send()
	if r.Header.Site == "" || bytes.Equal(pattern, currentSitePattern) {
		formatted := strings.Replace(location, "<br/>", ", ", 1)
		log.Debug().Str("site", formatted).Msg("defense site")
		for i, p := range r.Header.Players {
			defenseTeam := r.Header.Teams[p.TeamIndex].Role == Defense
			defenseRole := p.Operator != Recruit && p.Operator != 0 && p.Operator.Role() == Defense
			if defenseTeam || defenseRole {
				r.Header.Players[i].Spawn = formatted
			}
		}
		r.Header.Site = formatted
		return nil
	}
	return nil
}
