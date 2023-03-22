package dissect

import (
	"github.com/rs/zerolog/log"
	"strings"
)

func (r *Reader) readDefuserTimer() error {
	timer, err := r.readString()
	if err != nil {
		return err
	}
	if err = r.discard(34); err != nil {
		return err
	}
	id, err := r.read(4)
	if err != nil {
		return err
	}
	i := r.playerIndexById(id)
	a := DefuserPlantStart
	if r.planted {
		a = DefuserDisableStart
	}
	if i > -1 {
		u := MatchUpdate{
			Type:          a,
			Username:      r.Header.Players[i].Username,
			Time:          r.timeRaw,
			TimeInSeconds: r.time,
		}
		r.MatchFeedback = append(r.MatchFeedback, u)
		log.Debug().Interface("match_update", u).Send()
		r.lastDefuserPlayerIndex = i
	}
	// TODO: 0.00 can be present even if defuser was not disabled.
	if !strings.HasPrefix(timer, "0.00") {
		return nil
	}
	a = DefuserDisableComplete
	if !r.planted {
		a = DefuserPlantComplete
		r.planted = true
	}
	u := MatchUpdate{
		Type:          a,
		Username:      r.Header.Players[r.lastDefuserPlayerIndex].Username,
		Time:          r.timeRaw,
		TimeInSeconds: r.time,
	}
	r.MatchFeedback = append(r.MatchFeedback, u)
	log.Debug().Interface("match_update", u).Send()
	return nil
}
