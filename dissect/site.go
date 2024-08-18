package dissect

import (
	"bytes"
	"strings"

	"github.com/rs/zerolog/log"
)

var currentSitePattern = []byte{0xFC, 0xC6, 0xA8, 0x60, 0x01}

func readSpawn(r *Reader) error {
	location, err := r.String()
	if err != nil {
		return err
	}
	if err = r.Skip(150); err != nil {
		return err
	}
	pattern, err := r.Bytes(5)
	if err != nil {
		return err
	}
	if !strings.Contains(location, "<br/>") {
		return nil
	}
	log.Debug().
		Str("site", location).
		Send()
	if r.Header.Site == "" || bytes.Equal(pattern, currentSitePattern) {
		formatted := strings.Replace(location, "<br/>", ", ", 1)
		log.Debug().Str("site", formatted).Msg("defense site")
		for i, p := range r.Header.Players {
			defenseTeam := r.Header.Teams[p.TeamIndex].Role == Defense
			defenseRole := p.Operator != Recruit && p.Operator != 0 && p.Operator.Role() == Defense
			if defenseTeam || defenseRole {
				r.Header.Players[i].Spawn = formatted
			}
		}
		r.Header.Site = formatted
		return nil
	}
	return nil
}
