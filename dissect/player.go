package dissect

import (
	"bytes"

	"github.com/rs/zerolog/log"
)

func (r *DissectReader) readPlayer() error {
	idIndicator := []byte{0x33, 0xD8, 0x3D, 0x4F, 0x23}
	spawnIndicator := []byte{0xAF, 0x98, 0x99, 0xCA}
	usernameIndicator := []byte{0x22, 0x85, 0xCF, 0x36, 0x3A}
	profileIDIndicator := []byte{0x8A, 0x50, 0x9B, 0xD0}
	//unknownIndicator := []byte{0x22, 0xEE, 0xD4, 0x45, 0xC8, 0x08} // maybe player appearance?
	r.playersRead++
	if _, err := r.read(8); err != nil {
		return err
	}
	swap, err := r.read(1)
	if err != nil {
		return err
	}
	// Sometimes, 0x40, 0xF2, 0x15, 0x04 is sent twice.
	// Does not seem to be linked to role swap.
	if swap[0] == 0x9D {
		return nil
	}
	op, err := r.readUint64() // Op before atk role swaps
	if err != nil {
		return err
	}
	if op == 0 { // Empty player slot
		return nil
	}
	if err := r.seek(idIndicator); err != nil {
		return err
	}
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
	if r.playersRead > 5 {
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
		Operator:  Operator(op),
		Spawn:     spawn,
		id:        id,
	}
	log.Debug().Str("username", username).Int("teamIndex", teamIndex).Interface("op", p.Operator).Str("profileID", profileID).Hex("id", id).Send()
	found := false
	for i, existing := range r.Header.Players {
		if existing.Username == p.Username || existing.ID == p.ID {
			r.Header.Players[i].ID = p.ID
			r.Header.Players[i].ProfileID = p.ProfileID
			r.Header.Players[i].Username = p.Username
			r.Header.Players[i].TeamIndex = p.TeamIndex
			r.Header.Players[i].Operator = p.Operator
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
	return err
}

func (r *DissectReader) readAtkOpSwap() error {
	op, err := r.readUint64()
	if err != nil {
		return err
	}
	if _, err := r.read(5); err != nil {
		return err
	}
	id, err := r.read(4)
	if err != nil {
		return err
	}
	i := r.playerIndexById(id)
	o := Operator(op)
	if i > -1 {
		r.Header.Players[i].Operator = o
		u := MatchUpdate{
			Type:          OperatorSwap,
			Username:      r.Header.Players[i].Username,
			Time:          r.timeRaw,
			TimeInSeconds: r.time,
			Operator:      o,
		}
		r.MatchFeedback = append(r.MatchFeedback, u)
		log.Debug().Interface("match_update", u).Send()
	}
	log.Debug().Hex("id", id).Interface("op", op).Msg("atk_op_swap")
	return nil
}
