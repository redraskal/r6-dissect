package dissect

import (
	"github.com/rs/zerolog/log"
)

func (r *DissectReader) readPlayer() error {
	profileIDIndicator := []byte{0x8A, 0x50, 0x9B, 0xD0}
	//unknownIndicator := []byte{0x22, 0xEE, 0xD4, 0x45, 0xC8, 0x08} // maybe player appearance?
	teamIndicator, err := r.readInt()
	if err != nil {
		return err
	}
	teamIndex := 0
	if teamIndicator%2 != 0 {
		teamIndex = 1
	}
	if _, err = r.read(12); err != nil {
		return err
	}
	username, err := r.readString()
	if err != nil {
		return err
	}
	if err = r.seek([]byte{0x00, 0x1A}); err != nil {
		return err
	}
	if _, err = r.read(4); err != nil {
		return err
	}
	id, err := r.read(4)
	if err != nil {
		return err
	}
	// Older versions of siege did not include profile ids
	profileID := ""
	var unknownId uint64
	if len(r.Header.RecordingProfileID) > 0 {
		if err = r.seek(profileIDIndicator); err != nil {
			return err
		}
		profileID, err = r.readString()
		if err != nil {
			return err
		}
		_, err := r.read(5) // 22eed445c8
		if err != nil {
			return err
		}
		unknownId, err = r.readUint64()
		if err != nil {
			return err
		}
	} else {
		log.Debug().Str("warn", "profileID not found, skipping").Send()
	}
	player := Player{
		ID:        unknownId,
		ProfileID: profileID,
		Username:  username,
		TeamIndex: teamIndex,
		id:        id,
	}
	log.Debug().Str("username", username).Int("teamIndex", teamIndex).Str("profileID", profileID).Hex("id", id).Send()
	found := false
	for i, player := range r.Header.Players {
		if player.Username == username {
			r.Header.Players[i].ID = unknownId
			r.Header.Players[i].ProfileID = profileID
			r.Header.Players[i].TeamIndex = teamIndex
			found = true
			break
		}
	}
	if !found && len(username) > 0 {
		r.Header.Players = append(r.Header.Players, player)
	}
	//if err := r.seek(unknownIndicator); err != nil {
	//	return err
	//}
	//_, err = r.read(30) // unknown data, see above
	return err
}
