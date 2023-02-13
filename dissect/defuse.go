package dissect

import (
	"github.com/rs/zerolog/log"
)

func (r *DissectReader) readDefusePlantTimer() error {
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
	if i > -1 && timer == "6.967" {
		activity := Activity{
			Type:          DEFUSE_PLANT_START,
			Username:      r.Header.Players[i].Username,
			Time:          r.timeRaw,
			TimeInSeconds: r.time,
		}
		r.Activities = append(r.Activities, activity)
		log.Debug().Interface("activity", activity).Send()
		r.lastDefusePlanterIndex = i
	}
	if timer == "0.000" {
		activity := Activity{
			Type:          DEFUSE_PLANT_COMPLETE,
			Username:      r.Header.Players[r.lastDefusePlanterIndex].Username,
			Time:          r.timeRaw,
			TimeInSeconds: r.time,
		}
		r.Activities = append(r.Activities, activity)
		log.Debug().Interface("activity", activity).Send()
	}
	return nil
}
