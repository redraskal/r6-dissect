package reader

import (
	"bytes"
	"io"
	"strings"

	"github.com/redraskal/r6-dissect/types"
	"github.com/rs/zerolog/log"
)

var activity = []byte{0x59, 0x34, 0xe5, 0x8b, 0x04}
var activity2 = []byte{0x00, 0x00, 0x00, 0x22, 0xe3, 0x09, 0x00, 0x79}
var killIndicator = []byte{0x22, 0xd9, 0x13, 0x3c, 0xba}

func (c *Container) ReadActivities() ([]types.Activity, error) {
	activities := make([]types.Activity, 0)
	for {
		err := locate(activity, c.compressed)
		if err != nil {
			return activities, err
		}
		bombIndicator, err := readBytes(1, c.compressed)
		if err != nil {
			return activities, err
		}
		_ = bytes.Equal(bombIndicator, []byte{0x01}) // TODO, figure out meaning
		err = locate(activity2, c.compressed)
		if err != nil {
			return activities, err
		}
		size, err := readByteAsInt(c.compressed)
		if err != nil {
			return activities, err
		}
		if size == 0 { // kill or an unknown indicator at start of match
			killTrace, err := readBytes(5, c.compressed)
			if err != nil {
				return activities, err
			}
			if !bytes.Equal(killTrace, killIndicator) {
				log.Debug().Hex("killTrace", killTrace).Send()
				continue
			}
			username, err := readString(c.compressed)
			if err != nil {
				return activities, err
			}
			if len(username) == 0 {
				log.Debug().Str("warn", "kill username empty").Send()
				continue
			}
			// No idea what these 15 bytes mean (kill type?)
			_, err = readBytes(15, c.compressed)
			if err != nil {
				return activities, err
			}
			target, err := readString(c.compressed)
			if err != nil {
				return activities, err
			}
			activity := types.Activity{
				Type:     types.KILL,
				Username: username,
				Target:   target,
			}
			_, err = readBytes(56, c.compressed)
			if err != nil {
				return activities, err
			}
			headshot, err := readByteAsInt(c.compressed)
			if err != nil {
				return activities, err
			}
			headshotPtr := new(bool)
			if headshot == 1 {
				*headshotPtr = true
			}
			activity.Headshot = headshotPtr
			activities = append(activities, activity)
			log.Debug().Interface("activity", activity).Send()
			continue
		}
		b, err := readBytes(size, c.compressed)
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

func locate(search []byte, r io.Reader) error {
	b := make([]byte, 1)
	i := 0
	for {
		_, err := r.Read(b)
		if err != nil {
			return err
		}
		if b[0] != search[i] {
			i = 0
			continue
		}
		i++
		if i == len(search) {
			return nil
		}
	}
}

func readByteAsInt(r io.Reader) (int, error) {
	b, err := readBytes(1, r)
	if err != nil {
		return -1, err
	}
	return int(b[0]), nil
}

// readString reads one byte for string length and returns the resulting string.
func readString(r io.Reader) (string, error) {
	size, err := readByteAsInt(r)
	if err != nil {
		return "", err
	}
	b, err := readBytes(size, r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
