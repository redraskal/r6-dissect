package dissect

import (
	"bytes"
	"github.com/rs/zerolog/log"
	"strings"
)

func (r *Reader) readSpawn() error {
	log.Debug().Msg("site found")
	location, err := r.readString()
	if err != nil {
		return err
	}
	if _, err := r.read(6); err != nil {
		return err
	}
	site, err := r.read(1)
	if err != nil {
		return err
	}
	if r.Header.Site == "" && (bytes.Equal(site, []byte{0x02}) || bytes.Equal(site, []byte{0x04})) {
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
