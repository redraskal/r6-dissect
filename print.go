package main

import (
	"io"

	"github.com/redraskal/r6-dissect/reader"
	"github.com/redraskal/r6-dissect/types"
	"github.com/rs/zerolog/log"
)

func PrintHead(r io.Reader) error {
	h, err := reader.ReadHeader(r)
	if err != nil {
		return err
	}
	log.Info().Msgf("Game Version:     ", h.GameVersion)
	log.Info().Msgf("Recording Player: ", lookupUsername(h.RecordingPlayerID, h), " [", h.RecordingPlayerID, "]")
	log.Info().Msgf("Match ID:         ", h.MatchID)
	log.Info().Msgf("Timestamp:        ", h.Timestamp.Local())
	log.Info().Msgf("Match Type:       ", h.MatchType)
	log.Info().Msgf("Game Mode:        ", h.GameMode)
	log.Info().Msgf("Map:              ", h.Map)
	return nil
}

func lookupUsername(id string, h types.Header) string {
	for _, val := range h.Players {
		if val.ID == id {
			return val.Username
		}
	}
	return "UNKNOWN"
}
