package dissect

import (
	"github.com/rs/zerolog/log"
	"strings"
)

func (r *DissectReader) readDefuserTimer() error {
	timer, err := r.readString()
	if err != nil {
		return err
	}
	if _, err = r.read(34); err != nil {
		return err
	}
	id, err := r.read(4)
	if err != nil {
		return err
	}
	i := r.playerIndexById(id)
	a := DEFUSER_PLANT_START
	if r.planted {
		a = DEFUSER_DISABLE_START
	}
	if i > -1 {
		activity := Activity{
			Type:          a,
			Username:      r.Header.Players[i].Username,
			Time:          r.timeRaw,
			TimeInSeconds: r.time,
		}
		r.Activities = append(r.Activities, activity)
		log.Debug().Interface("activity", activity).Send()
		r.lastDefuserPlayerIndex = i
	}
	if !strings.HasPrefix(timer, "0.00") {
		return nil
	}
	a = DEFUSER_DISABLE_COMPLETE
	if !r.planted {
		a = DEFUSER_PLANT_COMPLETE
		r.planted = true
	}
	activity := Activity{
		Type:          a,
		Username:      r.Header.Players[r.lastDefuserPlayerIndex].Username,
		Time:          r.timeRaw,
		TimeInSeconds: r.time,
	}
	r.Activities = append(r.Activities, activity)
	log.Debug().Interface("activity", activity).Send()
	return nil
}
