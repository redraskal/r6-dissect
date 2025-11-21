package dissect

import (
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

func readDefuserTimer(r *Reader) error {
	timer, err := r.String()
	if err != nil {
		return err
	}
	prevTimer := r.lastDefuserTimer
	timerValue := -1.0
	if len(timer) > 0 {
		if v, parseErr := strconv.ParseFloat(timer, 64); parseErr == nil {
			timerValue = v
		}
	}
	if err = r.Skip(34); err != nil {
		return err
	}
	id, err := r.Bytes(4)
	if err != nil {
		return err
	}
	i := r.PlayerIndexByID(id)
	a := DefuserPlantStart
	recordStartEvent := true
	if r.planted {
		if timerValue >= 0 && prevTimer >= 0 && timerValue > prevTimer {
			a = DefuserDisableStart
			r.defuserDisabling = true
		} else {
			recordStartEvent = false
		}
	} else {
		r.defuserDisabling = false
	}
	if recordStartEvent && i > -1 {
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
		r.lastDefuserTimer = timerValue
		return nil
	}
	eventType := DefuserDisableComplete
	if !r.planted {
		eventType = DefuserPlantComplete
		r.planted = true
		r.defuserDisabling = false
	} else if r.defuserDisabling {
		eventType = DefuserDisableComplete
		r.defuserDisabling = false
		r.planted = false
	} else {
		r.lastDefuserTimer = timerValue
		return nil
	}
	u := MatchUpdate{
		Type:          eventType,
		Username:      r.Header.Players[r.lastDefuserPlayerIndex].Username,
		Time:          r.timeRaw,
		TimeInSeconds: r.time,
	}
	r.MatchFeedback = append(r.MatchFeedback, u)
	log.Debug().Interface("match_update", u).Send()
	r.lastDefuserTimer = timerValue
	return nil
}
