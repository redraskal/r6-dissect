package reader

import (
	"bytes"
	"github.com/klauspost/compress/zstd"
	"github.com/redraskal/r6-dissect/types"
	"github.com/rs/zerolog/log"
	"io"
	"strings"
)

var activity = []byte{0x2b, 0x9d, 0x69, 0x47, 0x23}
var someOtherActivityIndicator = []byte{0x0b, 0xf0}
var killIndicator = []byte{0x22, 0xd9, 0x13, 0x3c, 0xba}

func (c *Container) ReadActivities() ([]types.Activity, error) {
	activities := make([]types.Activity, 0)
	for {
		err := locate(activity, c.compressed)
		if err == io.EOF || err == zstd.ErrMagicMismatch {
			return activities, nil
		}
		if err != nil {
			return activities, err
		}
		// No idea what these 2 bytes mean
		_, err = readBytes(2, c.compressed)
		if err != nil {
			return activities, err
		}
		indicator, err := readBytes(2, c.compressed)
		if err != nil {
			return activities, err
		}
		log.Debug().Hex("indicator", indicator).Send()
		if !bytes.Equal(indicator, someOtherActivityIndicator) {
			continue
		}
		// No idea what the first 18 bytes mean
		_, err = readBytes(18, c.compressed)
		if err != nil {
			return activities, err
		}
		size, err := readByteAsInt(c.compressed)
		if err != nil {
			return activities, err
		}
		// Activity is finding objective if size is not 0
		if size != 0 {
			b, err := readBytes(size, c.compressed)
			if err != nil {
				return activities, err
			}
			username := strings.Split(string(b), " ")[0]
			activity := types.Activity{
				Type:     types.LOCATE_OBJECTIVE,
				Username: username,
				Target:   "",
			}
			activities = append(activities, activity)
			log.Debug().Interface("activity", activity).Send()
			continue
		}
		secondIndicator, err := readBytes(5, c.compressed)
		if err != nil {
			return activities, err
		}
		log.Debug().Hex("second_indicator", secondIndicator).Send()
		// Objective found indicator seems to happen when the last 6 bytes are missing
		// Kills seem to happen when these last 6 bytes are present
		if !bytes.Equal(secondIndicator, killIndicator) {
			continue
		}
		username, err := readString(c.compressed)
		if err != nil {
			return activities, err
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
