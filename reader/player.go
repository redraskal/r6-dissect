package reader

import (
	"github.com/redraskal/r6-dissect/types"
	"github.com/rs/zerolog/log"
)

var playerIndicator = []byte{0x22, 0x95, 0x1C, 0x16, 0x50, 0x08}

func (r *DissectReader) readPlayer() error {
	profileIDIndicator := []byte{0x8A, 0x50, 0x9B, 0xD0}
	unknownIndicator := []byte{0x22, 0xEE, 0xD4, 0x45, 0xC8, 0x08} // loadout & appearance... no data on operator
	teamIndicator, err := r.readInt()
	if err != nil {
		return err
	}
	teamIndex := 0
	if teamIndicator%2 != 0 {
		teamIndex = 1
	}
	if _, err := r.read(12); err != nil {
		return err
	}
	username, err := r.readString()
	if err != nil {
		return err
	}
	if err := r.seek(profileIDIndicator); err != nil {
		return err
	}
	profileID, err := r.readString()
	if err != nil {
		return err
	}
	player := types.Player{
		ProfileID: profileID,
		Username:  username,
		TeamIndex: teamIndex,
	}
	log.Debug().Str("username", username).Int("teamIndex", teamIndex).Str("profileID", profileID).Send()
	// Ignore duplicates
	for i, player := range r.Header.Players {
		if player.Username == username {
			r.Header.Players[i].ProfileID = profileID
			r.Header.Players[i].TeamIndex = teamIndex
			return nil
		}
	}
	r.Header.Players = append(r.Header.Players, player)
	if err := r.seek(unknownIndicator); err != nil {
		return err
	}
	_, err = r.read(30) // unknown data, see above
	return err
}
