package dissect

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

func readTime(r *Reader) error {
	time, err := r.Uint32()
	if err != nil {
		return err
	}
	r.time = float64(time)
	r.timeRaw = fmt.Sprintf("%d:%02d", time/60, time%60)
	return nil
}

func readY7Time(r *Reader) error {
	time, err := r.String()
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

	if r.Header.CodeVersion >= Y9S4 {
		team0Won := r.Header.Teams[0].StartingScore < r.Header.Teams[0].Score
		r.Header.Teams[0].Won = team0Won
		r.Header.Teams[1].Won = !team0Won
	}

	for _, u := range r.MatchFeedback {
		switch u.Type {
		case Kill:
			i := r.Header.Players[r.PlayerIndexByUsername(u.Target)].TeamIndex
			deaths[i] = deaths[i] + 1
			// fix killer username
			if len(u.usernameFromScoreboard) > 0 {
				u.Username = u.usernameFromScoreboard
			}
			break
		case Death:
			i := r.Header.Players[r.PlayerIndexByUsername(u.Username)].TeamIndex
			deaths[i] = deaths[i] + 1
			break
		case DefuserPlantComplete:
			planter = r.PlayerIndexByUsername(u.Username)
			break
		case DefuserDisableComplete:
			i := r.Header.Players[r.PlayerIndexByUsername(u.Username)].TeamIndex
			winningTeam := i
			if r.Header.CodeVersion >= Y9S4 {
				winningTeam = 0
				if r.Header.Teams[1].Won {
					winningTeam = 1
				}
			}

			r.Header.Teams[winningTeam].Won = true
			if r.Header.CodeVersion >= Y9S4 {
				r.Header.Teams[winningTeam^1].Won = false
			}
			r.Header.Teams[winningTeam].WinCondition = DisabledDefuser
			return
		}
	}

	if planter > -1 {
		planterTeam := r.Header.Players[planter].TeamIndex
		if r.Header.CodeVersion >= Y9S4 {
			winningTeamIndex := 0
			if r.Header.Teams[1].Won {
				winningTeamIndex = 1
			}
			r.Header.Teams[winningTeamIndex].Won = true
			r.Header.Teams[winningTeamIndex^1].Won = false
			r.Header.Teams[winningTeamIndex].WinCondition = DefusedBomb
		} else {
			r.Header.Teams[planterTeam].Won = true
			r.Header.Teams[planterTeam].WinCondition = DefusedBomb
		}
		return
	}

	if r.Header.CodeVersion >= Y9S4 {
		winningTeamIndex := 0
		if r.Header.Teams[1].Won {
			winningTeamIndex = 1
		}

		if deaths[0] == sizes[0] && winningTeamIndex == 1 {
			r.Header.Teams[1].Won = true
			r.Header.Teams[0].Won = false
			r.Header.Teams[1].WinCondition = KilledOpponents
			return
		}
		if deaths[1] == sizes[1] && winningTeamIndex == 0 {
			r.Header.Teams[0].Won = true
			r.Header.Teams[1].Won = false
			r.Header.Teams[0].WinCondition = KilledOpponents
			return
		}

		defenseTeamIndex := 0
		if roles[1] == Defense {
			defenseTeamIndex = 1
		}

		r.Header.Teams[winningTeamIndex].Won = true
		r.Header.Teams[winningTeamIndex^1].Won = false
		if winningTeamIndex == defenseTeamIndex {
			r.Header.Teams[winningTeamIndex].WinCondition = Time
		} else {
			r.Header.Teams[winningTeamIndex].WinCondition = KilledOpponents
		}
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
