package main

import (
	"github.com/rs/zerolog/log"
)

func (r *DissectReader) head() {
	username := "N/A"
	for _, p := range r.Header.Players {
		if p.ProfileID == r.Header.RecordingProfileID {
			username = p.Username
		}
	}
	log.Info().Msgf("Version:          %s/%d", r.Header.GameVersion, r.Header.CodeVersion)
	log.Info().Msgf("Recording Player: %s [%s]", username, r.Header.RecordingProfileID)
	log.Info().Msgf("Match ID:         %s", r.Header.MatchID)
	log.Info().Msgf("Timestamp:        %s", r.Header.Timestamp.Local())
	log.Info().Msgf("Match Type:       %s", r.Header.MatchType)
	log.Info().Msgf("Game Mode:        %s", r.Header.GameMode)
	log.Info().Msgf("Map:              %s", r.Header.Map)
}
