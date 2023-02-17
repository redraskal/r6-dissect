package dissect

import (
	"bytes"
	"encoding/json"
	"github.com/rs/zerolog/log"
	"strings"
)

type ActivityType int

//go:generate stringer -type=ActivityType
const (
	KILL ActivityType = iota
	DEATH
	DEFUSER_PLANT_START
	DEFUSER_PLANT_COMPLETE
	DEFUSER_DISABLE_START
	DEFUSER_DISABLE_COMPLETE
	LOCATE_OBJECTIVE
	BATTLEYE
	PLAYER_LEAVE
)

type Activity struct {
	Type          ActivityType `json:"type"`
	Username      string       `json:"username,omitempty"`
	Target        string       `json:"target,omitempty"`
	Headshot      *bool        `json:"headshot,omitempty"`
	Time          string       `json:"time"`
	TimeInSeconds float64      `json:"timeInSeconds"`
}

func (i ActivityType) MarshalJSON() (text []byte, err error) {
	return json.Marshal(i.String())
}

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
		empty := len(username) == 0
		if empty {
			log.Debug().Str("warn", "kill username empty").Send()
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
		if empty && len(target) > 0 {
			activity := Activity{
				Type:          DEATH,
				Username:      target,
				Time:          r.timeRaw,
				TimeInSeconds: r.time,
			}
			r.Activities = append(r.Activities, activity)
			log.Debug().Interface("activity", activity).Send()
			log.Debug().Msg("kill username empty because of death")
			return nil
		} else if empty {
			return nil
		}
		activity := Activity{
			Type:          KILL,
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
			if val.Type == KILL && val.Username == activity.Username && val.Target == activity.Target {
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
	activityType := KILL
	if strings.HasPrefix(activityMessage, "Friendly Fire") {
		return nil
	}
	if strings.Contains(activityMessage, "bombs") || strings.Contains(activityMessage, "objective") {
		activityType = LOCATE_OBJECTIVE
	}
	if strings.Contains(activityMessage, "BattlEye") {
		activityType = BATTLEYE
	}
	if strings.Contains(activityMessage, "left") {
		activityType = PLAYER_LEAVE
	}
	username := strings.Split(activityMessage, " ")[0]
	log.Debug().Str("activity_msg", activityMessage).Send()
	if activityType == KILL {
		return nil
	}
	activity := Activity{
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
