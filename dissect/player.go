package dissect

import (
	"bytes"
	"github.com/rs/zerolog/log"
)

func (r *DissectReader) readPlayer() error {
	spawnIndicator := []byte{0xAF, 0x98, 0x99, 0xCA}
	usernameIndicator := []byte{0x22, 0x85, 0xCF, 0x36, 0x3A}
	profileIDIndicator := []byte{0x8A, 0x50, 0x9B, 0xD0}
	//unknownIndicator := []byte{0x22, 0xEE, 0xD4, 0x45, 0xC8, 0x08} // maybe player appearance?
	id, err := r.read(4)
	if err != nil {
		return err
	}
	if err := r.seek(spawnIndicator); err != nil {
		return err
	}
	spawn, err := r.readString()
	if err != nil {
		return err
	}
	if spawn == "" {
		if _, err := r.read(10); err != nil {
			return err
		}
		valid, err := r.read(1)
		if err != nil {
			return err
		}
		if !bytes.Equal(valid, []byte{0x1B}) {
			return nil
		}
	}
	if err := r.seek(usernameIndicator); err != nil {
		return err
	}
	teamIndex := 0
	if r.playersRead > 4 {
		teamIndex = 1
	}
	username, err := r.readString()
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
	p := Player{
		ID:        unknownId,
		ProfileID: profileID,
		Username:  username,
		TeamIndex: teamIndex,
		Spawn:     spawn,
		id:        id,
	}
	if spawn == "" {
		p.Alliance = 4
		r.Header.Teams[teamIndex].Role = DEFENSE
	} else {
		r.Header.Teams[teamIndex].Role = ATTACK
	}
	log.Debug().Str("username", username).Int("teamIndex", teamIndex).Str("profileID", profileID).Hex("id", id).Send()
	found := false
	for i, existing := range r.Header.Players {
		if existing.Username == p.Username || existing.ID == p.ID {
			r.Header.Players[i].ID = p.ID
			r.Header.Players[i].ProfileID = p.ProfileID
			r.Header.Players[i].Username = p.Username
			r.Header.Players[i].TeamIndex = p.TeamIndex
			r.Header.Players[i].Alliance = p.Alliance
			r.Header.Players[i].Spawn = p.Spawn
			r.Header.Players[i].id = p.id
			found = true
			break
		}
	}
	if !found && len(username) > 0 {
		r.Header.Players = append(r.Header.Players, p)
	}
	//if err := r.seek(unknownIndicator); err != nil {
	//	return err
	//}
	//_, err = r.read(30) // unknown data, see above
	r.playersRead++
	return err
}
