package dissect

import (
	"strings"

	"github.com/rs/zerolog/log"
)

func readSpawn(r *Reader) error {
	location, err := r.String()
	if err != nil {
		return err
	}
	if err = r.Skip(37); err != nil {
		return err
	}
	flag, err := r.Int()
	if err != nil {
		return err
	}
	if !strings.Contains(location, "<br/>") {
		return nil
	}
	log.Debug().
		Int("flag", flag).
		Str("site", location).
		Msg("site")
	if r.Header.Site == "" && (flag == 1 || flag == 164) {
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
