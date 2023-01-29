package reader

import (
	"bytes"
	"strings"

	"github.com/redraskal/r6-dissect/types"
	"github.com/rs/zerolog/log"
)

var activityIndicator = []byte{0x59, 0x34, 0xe5, 0x8b, 0x04}

var activity2 = []byte{0x00, 0x00, 0x00, 0x22, 0xe3, 0x09, 0x00, 0x79}
var killIndicator = []byte{0x22, 0xd9, 0x13, 0x3c, 0xba}

func (r *DissectReader) readActivity() error {
	bombIndicator, err := r.read(1)
	if err != nil {
		return err
	}
	_ = bytes.Equal(bombIndicator, []byte{0x01}) // TODO, figure out meaning
	err = r.seek(activity2)
	if err != nil {
		return err
	}
	size, err := r.readInt()
	if err != nil {
		return err
	}
	if size == 0 { // kill or an unknown indicator at start of match
		killTrace, err := r.read(5)
		if err != nil {
			return err
		}
		if !bytes.Equal(killTrace, killIndicator) {
			log.Debug().Hex("killTrace", killTrace).Send()
			return nil
		}
		username, err := r.readString()
		if err != nil {
			return err
		}
		if len(username) == 0 {
			log.Debug().Str("warn", "kill username empty").Send()
			return nil
		}
		// No idea what these 15 bytes mean (kill type?)
		_, err = r.read(15)
		if err != nil {
			return err
		}
		target, err := r.readString()
		if err != nil {
			return err
		}
		activity := types.Activity{
			Type:          types.KILL,
			Username:      username,
			Target:        target,
			Time:          r.timeRaw,
			TimeInSeconds: r.time,
		}
		_, err = r.read(56)
		if err != nil {
			return err
		}
		headshot, err := r.readInt()
		if err != nil {
			return err
		}
		headshotPtr := new(bool)
		if headshot == 1 {
			*headshotPtr = true
		}
		activity.Headshot = headshotPtr
		// Ignore duplicates
		for _, val := range r.Activities {
			if val.Type == types.KILL && val.Username == activity.Username && val.Target == activity.Target {
				return nil
			}
		}
		r.Activities = append(r.Activities, activity)
		log.Debug().Interface("activity", activity).Send()
		return nil
	}
	b, err := r.read(size)
	if err != nil {
		return err
	}
	activityMessage := string(b)
	activityType := types.KILL
	if strings.HasPrefix(activityMessage, "Friendly Fire") {
		return nil
	}
	if strings.Contains(activityMessage, "bombs") || strings.Contains(activityMessage, "objective") {
		activityType = types.LOCATE_OBJECTIVE
	}
	if strings.Contains(activityMessage, "BattlEye") {
		activityType = types.BATTLEYE
	}
	if strings.Contains(activityMessage, "left") {
		activityType = types.PLAYER_LEAVE
	}
	username := strings.Split(activityMessage, " ")[0]
	log.Debug().Str("activity_msg", activityMessage).Send()
	if activityType == types.KILL {
		return nil
	}
	activity := types.Activity{
		Type:          activityType,
		Username:      username,
		Target:        "",
		Time:          r.timeRaw,
		TimeInSeconds: r.time,
	}
	r.Activities = append(r.Activities, activity)
	log.Debug().Interface("activity", activity).Send()
	return nil
}
