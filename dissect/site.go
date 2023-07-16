package dissect

import (
	"github.com/rs/zerolog/log"
	"strings"
)

func (r *Reader) readSpawn() error {
	log.Debug().Msg("site found")
	location, err := r.readString()
	if err != nil {
		return err
	}
	if err = r.skip(6); err != nil {
		return err
	}
	if !strings.Contains(location, "<br/>") {
		return nil
	}
	if r.Header.Site == "" {
		formatted := strings.Replace(location, "<br/>", ", ", 1)
		log.Debug().Str("site", formatted).Msg("defense site")
		for i, p := range r.Header.Players {
			if r.Header.Teams[p.TeamIndex].Role == Defense {
				r.Header.Players[i].Spawn = formatted
			}
		}
		r.Header.Site = formatted
		return nil
	}
	return nil
}
