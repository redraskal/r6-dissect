package main

import (
	"github.com/redraskal/r6-dissect/reader"
	"github.com/rs/zerolog/log"
)

func PrintHead(r *reader.DissectReader) {
	log.Info().Msgf("Version:          %s/%d", r.Header.GameVersion, r.Header.CodeVersion)
	log.Info().Msgf("Recording Player: %s", r.Header.RecordingProfileID)
	log.Info().Msgf("Match ID:         %s", r.Header.MatchID)
	log.Info().Msgf("Timestamp:        %s", r.Header.Timestamp.Local())
	log.Info().Msgf("Match Type:       %s", r.Header.MatchType)
	log.Info().Msgf("Game Mode:        %s", r.Header.GameMode)
	log.Info().Msgf("Map:              %s", r.Header.Map)
}
