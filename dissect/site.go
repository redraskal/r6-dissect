package dissect

import (
	"bytes"
	"github.com/rs/zerolog/log"
	"strings"
)

func (r *DissectReader) readSpawn() error {
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
	if bytes.Equal(site, []byte{0x02}) {
		formatted := strings.Replace(location, "<br/>", ", ", 1)
		log.Debug().Str("site", formatted).Msg("defense site")
		for i, p := range r.Header.Players {
			if p.Alliance == 4 {
				r.Header.Players[i].Spawn = formatted
			}
		}
		r.Header.Site = formatted
		return nil
	}
	return nil
}
