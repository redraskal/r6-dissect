package main

import (
	"os"

	"github.com/redraskal/r6-dissect/reader"
	"github.com/rs/zerolog/log"
)

func PrintHead(r reader.DissectReader) {
	player := r.Header.RecordingPlayer()
	if player.Username == "" {
		player.Username = "UNKNOWN"
	}
	if player.ID == "" {
		player.ID = "--"
	}
	log.Info().Msgf("Version:          %s/%d", r.Header.GameVersion, r.Header.CodeVersion)
	log.Info().Msgf("Recording Player: %s [%s]", player.Username, player.ID)
	log.Info().Msgf("Match ID:         %s", r.Header.MatchID)
	log.Info().Msgf("Timestamp:        %s", r.Header.Timestamp.Local())
	log.Info().Msgf("Match Type:       %s", r.Header.MatchType)
	log.Info().Msgf("Game Mode:        %s", r.Header.GameMode)
	log.Info().Msgf("Map:              %s", r.Header.Map)
}

func DumpStatic(r reader.DissectReader) error {
	static, err := r.ReadStatic()
	if err != nil {
		return err
	}
	if err := os.WriteFile("static.bin", static, os.ModePerm); err != nil {
		return err
	}
	log.Info().Msg("static data dumped to static.bin!")
	return nil
}
