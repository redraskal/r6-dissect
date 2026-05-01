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
	disabler := -1
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

	if r.Header.CodeVersion >= Y10S4 {
		r.reconcileKillsFromScoreboard()
	}

	for i := range r.MatchFeedback {
		u := &r.MatchFeedback[i]
		switch u.Type {
		case Kill:
			ti := r.Header.Players[r.PlayerIndexByUsername(u.Target)].TeamIndex
			deaths[ti] = deaths[ti] + 1
			// fix killer username
			if len(u.usernameFromScoreboard) > 0 {
				log.Debug().
					Str("original", u.Username).
					Str("corrected", u.usernameFromScoreboard).
					Str("target", u.Target).
					Msg("scoreboard_kill_correction")
				u.Username = u.usernameFromScoreboard
			}
			break
		case Death:
			ti := r.Header.Players[r.PlayerIndexByUsername(u.Username)].TeamIndex
			deaths[ti] = deaths[ti] + 1
			break
		case DefuserPlantComplete:
			planter = r.PlayerIndexByUsername(u.Username)
			break
		case DefuserDisableStart:
			disabler = r.PlayerIndexByUsername(u.Username)
			break
		case DefuserDisableComplete:
			if r.Header.CodeVersion >= Y9S4 {
				winningTeam := 0
				if r.Header.Teams[1].Won {
					winningTeam = 1
				}
				r.Header.Teams[winningTeam].WinCondition = DisabledDefuser
			} else {
				playerIdx := r.PlayerIndexByUsername(u.Username)
				if playerIdx < 0 || playerIdx >= len(r.Header.Players) {
					log.Debug().Msg("warn: defuser disable player not found")
					return
				}
				ti := r.Header.Players[playerIdx].TeamIndex
				r.Header.Teams[ti].Won = true
				r.Header.Teams[ti].WinCondition = DisabledDefuser
			}
			return
		}
	}

	if r.Header.CodeVersion >= Y9S4 && planter > -1 {
		defenseTeamIndex := -1
		for i, team := range r.Header.Teams {
			if team.Role == Defense {
				defenseTeamIndex = i
				break
			}
		}
		if defenseTeamIndex >= 0 && r.Header.Teams[defenseTeamIndex].Won {
			username := ""
			if disabler >= 0 && disabler < len(r.Header.Players) {
				username = r.Header.Players[disabler].Username
			}
			u := MatchUpdate{
				Type:          DefuserDisableComplete,
				Username:      username,
				Time:          r.timeRaw,
				TimeInSeconds: r.time,
			}
			r.MatchFeedback = append(r.MatchFeedback, u)
			log.Debug().Interface("match_update", u).Msg("inferred DefuserDisableComplete")
			r.Header.Teams[defenseTeamIndex].WinCondition = DisabledDefuser
			return
		}
	}

	if planter > -1 {
		r.Header.Teams[r.Header.Players[planter].TeamIndex].Won = true
		r.Header.Teams[r.Header.Players[planter].TeamIndex].WinCondition = DefusedBomb
		return
	}

	if r.Header.CodeVersion >= Y10S4 {
		if deaths[0] == sizes[0] && sizes[0] > 0 {
			r.Header.Teams[1].Won = true
			r.Header.Teams[1].WinCondition = KilledOpponents
			return
		}
		if deaths[1] == sizes[1] && sizes[1] > 0 {
			r.Header.Teams[0].Won = true
			r.Header.Teams[0].WinCondition = KilledOpponents
			return
		}
		for i := range r.Header.Teams {
			if r.Header.Teams[i].Won && r.Header.Teams[i].WinCondition == "" {
				if r.Header.Teams[i].Role == Defense {
					r.Header.Teams[i].WinCondition = Time
				}
			}
		}
		return
	}

	if r.Header.CodeVersion >= Y9S4 {
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
