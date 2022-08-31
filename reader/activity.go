package reader

import (
	"bytes"
	"github.com/klauspost/compress/zstd"
	"github.com/redraskal/r6-dissect/types"
	"github.com/rs/zerolog/log"
	"io"
)

var activity = []byte{0x2b, 0x9d, 0x69, 0x47, 0x23}
var someOtherActivityIndicator = []byte{0x0b, 0xf0}
var killIndicator = []byte{0x00, 0x22, 0xd9, 0x13, 0x3c, 0xba}

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
		// No idea what the these 18 bytes mean
		secondIndicator, err := readBytes(24, c.compressed)
		if err != nil {
			return activities, err
		}
		log.Debug().Hex("second_indicator", secondIndicator).Send()
		// Objective found indicator seems to happen when the last 6 bytes are missing
		// Kills seem to happen when these last 6 bytes are present
		if !bytes.Equal(secondIndicator[18:], killIndicator) {
			continue
		}
		playerName, err := readString(c.compressed)
		if err != nil {
			return activities, err
		}
		// No idea what these 15 bytes mean (kill type?)
		_, err = readBytes(15, c.compressed)
		if err != nil {
			return activities, err
		}
		targetPlayerName, err := readString(c.compressed)
		if err != nil {
			return activities, err
		}
		activity := types.Activity{
			Type:   types.KILL,
			Player: playerName,
			Target: targetPlayerName,
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

// readString reads one byte for string length and returns the resulting string.
func readString(r io.Reader) (string, error) {
	b, err := readBytes(1, r)
	if err != nil {
		return "", err
	}
	size := int(b[0])
	b, err = readBytes(size, r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
