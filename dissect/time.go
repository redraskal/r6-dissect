package dissect

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"strconv"
	"strings"
)

func (r *Reader) readTime() error {
	time, err := r.readUint32()
	if err != nil {
		return err
	}
	r.time = float64(time)
	r.timeRaw = fmt.Sprintf("%d:%02d", time/60, time%60)
	return nil
}

func (r *Reader) readY7Time() error {
	time, err := r.readString()
	parts := strings.Split(time, ":")
	if len(parts) == 1 {
		seconds, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return err
		}
		r.time = seconds
		r.timeRaw = parts[0]
		return nil
	}
	minutes, err := strconv.Atoi(parts[0])
	if err != nil {
		return err
	}
	seconds, err := strconv.Atoi(parts[1])
	if err != nil {
		return err
	}
	r.time = float64((minutes * 60) + seconds)
	r.timeRaw = time
	return nil
}

func (r *Reader) roundEnd() {
	log.Debug().Msg("round_end")
	planter := -1
	deaths := make(map[int]int)
	sizes := make(map[int]int)
	roles := make(map[int]TeamRole)
	for _, p := range r.Header.Players {
		sizes[p.TeamIndex] += 1
		roles[p.TeamIndex] = r.Header.Teams[p.TeamIndex].Role
	}
	for _, u := range r.MatchFeedback {
		switch u.Type {
		case Kill:
			i := r.Header.Players[r.playerIndexByUsername(u.Target)].TeamIndex
			deaths[i] = deaths[i] + 1
			break
		case Death:
			i := r.Header.Players[r.playerIndexByUsername(u.Username)].TeamIndex
			deaths[i] = deaths[i] + 1
			break
		case DefuserPlantComplete:
			planter = r.playerIndexByUsername(u.Username)
			break
		case DefuserDisableComplete:
			i := r.Header.Players[r.playerIndexByUsername(u.Username)].TeamIndex
			r.Header.Teams[i].Won = true
			r.Header.Teams[i].WinCondition = DisabledDefuser
			return
		}
	}
	if planter > -1 {
		r.Header.Teams[r.Header.Players[planter].TeamIndex].Won = true
		r.Header.Teams[r.Header.Players[planter].TeamIndex].WinCondition = DefusedBomb
		return
	}
	if deaths[0] == sizes[0] {
		if planter > -1 && roles[0] == Attack { // ignore attackers killed post-plant
			return
		}
		r.Header.Teams[1].Won = true
		r.Header.Teams[1].WinCondition = KilledOpponents
		return
	}
	if deaths[1] == sizes[1] {
		if planter > -1 && roles[1] == Attack { // ignore attackers killed post-plant
			return
		}
		r.Header.Teams[0].Won = true
		r.Header.Teams[0].WinCondition = KilledOpponents
		return
	}
	i := 0
	if roles[1] == Defense {
		i = 1
	}
	r.Header.Teams[i].Won = true
	r.Header.Teams[i].WinCondition = Time
}
