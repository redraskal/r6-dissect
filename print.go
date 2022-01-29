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
	log.Info().Msgf("Game Version:     %d", h.GameVersion)
	log.Info().Msgf("Recording Player: %s [%s]", lookupUsername(h.RecordingPlayerID, h), h.RecordingPlayerID)
	log.Info().Msgf("Match ID:         %s", h.MatchID)
	log.Info().Msgf("Timestamp:        %s", h.Timestamp.Local())
	log.Info().Msgf("Match Type:       %s", h.MatchType)
	log.Info().Msgf("Game Mode:        %s", h.GameMode)
	log.Info().Msgf("Map:              %s", h.Map)
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
