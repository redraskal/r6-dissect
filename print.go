package main

import (
	"os"

	"github.com/redraskal/r6-dissect/reader"
	"github.com/redraskal/r6-dissect/types"
	"github.com/rs/zerolog/log"
)

func PrintHead(c reader.Container) {
	log.Info().Msgf("Version:          %s/%d", c.Header.GameVersion, c.Header.CodeVersion)
	log.Info().Msgf("Recording Player: %s [%s]", lookupUsername(c.Header.RecordingPlayerID, c.Header), c.Header.RecordingPlayerID)
	log.Info().Msgf("Match ID:         %s", c.Header.MatchID)
	log.Info().Msgf("Timestamp:        %s", c.Header.Timestamp.Local())
	log.Info().Msgf("Match Type:       %s", c.Header.MatchType)
	log.Info().Msgf("Game Mode:        %s", c.Header.GameMode)
	log.Info().Msgf("Map:              %s", c.Header.Map)
}

func DumpStatic(c reader.Container) error {
	static, err := c.ReadStatic()
	if err != nil {
		return err
	}
	if err := os.WriteFile("static.bin", static, os.ModePerm); err != nil {
		return err
	}
	log.Info().Msg("static data dumped to static.bin!")
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
