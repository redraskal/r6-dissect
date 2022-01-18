package main

import (
	"io"
	"log"

	"github.com/redraskal/r6-dissect/reader"
	"github.com/redraskal/r6-dissect/types"
)

func PrintHead(r io.Reader) error {
	h, err := reader.ReadHeader(r)
	if err != nil {
		return err
	}
	log.Println("Game Version:     ", h.GameVersion)
	log.Println("Recording Player: ", lookupUsername(h.RecordingPlayerID, h), " [", h.RecordingPlayerID, "]")
	log.Println("Match ID:         ", h.MatchID)
	log.Println("Timestamp:        ", h.Timestamp.Local())
	log.Println("Match Type:       ", h.MatchType)
	log.Println("Game Mode:        ", h.GameMode)
	log.Println("Map:              ", h.Map)
	return nil
}

func lookupUsername(ID string, h types.Header) string {
	for _, val := range h.Players {
		if val.ID == ID {
			return val.Username
		}
	}
	return "UNKNOWN"
}
