package reader

import (
	"bytes"
	"strings"

	"github.com/redraskal/r6-dissect/types"
	"github.com/rs/zerolog/log"
)

var activity = []byte{0x59, 0x34, 0xe5, 0x8b, 0x04}
var activity2 = []byte{0x00, 0x00, 0x00, 0x22, 0xe3, 0x09, 0x00, 0x79}
var killIndicator = []byte{0x22, 0xd9, 0x13, 0x3c, 0xba}

func (r *DissectReader) ReadActivities() ([]types.Activity, error) {
	activities := make([]types.Activity, 0)
	for {
		err := r.Seek(activity)
		if err != nil {
			return activities, err
		}
		bombIndicator, err := r.Read(1)
		if err != nil {
			return activities, err
		}
		_ = bytes.Equal(bombIndicator, []byte{0x01}) // TODO, figure out meaning
		err = r.Seek(activity2)
		if err != nil {
			return activities, err
		}
		size, err := r.ReadInt()
		if err != nil {
			return activities, err
		}
		if size == 0 { // kill or an unknown indicator at start of match
			killTrace, err := r.Read(5)
			if err != nil {
				return activities, err
			}
			if !bytes.Equal(killTrace, killIndicator) {
				log.Debug().Hex("killTrace", killTrace).Send()
				continue
			}
			username, err := r.ReadString()
			if err != nil {
				return activities, err
			}
			if len(username) == 0 {
				log.Debug().Str("warn", "kill username empty").Send()
				continue
			}
			// No idea what these 15 bytes mean (kill type?)
			_, err = r.Read(15)
			if err != nil {
				return activities, err
			}
			target, err := r.ReadString()
			if err != nil {
				return activities, err
			}
			activity := types.Activity{
				Type:     types.KILL,
				Username: username,
				Target:   target,
			}
			_, err = r.Read(56)
			if err != nil {
				return activities, err
			}
			headshot, err := r.ReadInt()
			if err != nil {
				return activities, err
			}
			headshotPtr := new(bool)
			if headshot == 1 {
				*headshotPtr = true
			}
			activity.Headshot = headshotPtr
			found := false
			for _, val := range activities {
				if val.Type == types.KILL && val.Username == activity.Username && val.Target == activity.Target {
					found = true
					break
				}
			}
			if !found {
				activities = append(activities, activity)
				log.Debug().Interface("activity", activity).Send()
			}
			continue
		}
		b, err := r.Read(size)
		if err != nil {
			return activities, err
		}
		activityMessage := string(b)
		activityType := types.KILL
		if strings.HasPrefix(activityMessage, "Friendly Fire") {
			continue
		}
		if strings.Contains(activityMessage, "bombs") {
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
			continue
		}
		activity := types.Activity{
			Type:     activityType,
			Username: username,
			Target:   "",
		}
		activities = append(activities, activity)
		log.Debug().Interface("activity", activity).Send()
	}
}
