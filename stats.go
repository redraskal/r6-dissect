package main

type PlayerRoundStats struct {
	Username           string  `json:"username"`
	TeamIndex          int     `json:"teamIndex"`
	Kills              int     `json:"kills"`
	Died               bool    `json:"died"`
	Assists            int     `json:"assists"`
	Headshots          int     `json:"headshots"`
	HeadshotPercentage float64 `json:"headshotPercentage"`
	OneVx              int     `json:"1vX"`
}

// OpeningKill returns the first player to kill.
func (r *DissectReader) OpeningKill() Activity {
	for _, a := range r.Activities {
		if a.Type == KILL {
			return a
		}
	}
	return Activity{}
}

// OpeningDeath returns the first player to die (KILL or DEATH activity).
func (r *DissectReader) OpeningDeath() Activity {
	for _, a := range r.Activities {
		if a.Type == KILL || a.Type == DEATH {
			return a
		}
	}
	return Activity{}
}

// Trades returns KILL Activity pairs of trades.
func (r *DissectReader) Trades() [][]Activity {
	trades := make([][]Activity, 0)
	var previous = Activity{}
	for _, a := range r.Activities {
		if a.Type == KILL && previous.Target == a.Username {
			trades = append(trades, []Activity{previous, a})
		}
	}
	return trades
}

func (r *DissectReader) KillsAndDeaths() []Activity {
	activities := make([]Activity, 0)
	for _, a := range r.Activities {
		if a.Type == KILL || a.Type == DEATH {
			activities = append(activities, a)
		}
	}
	return activities
}

func (r *DissectReader) NumPlayers(team int) int {
	n := 0
	for _, p := range r.Header.Players {
		if p.TeamIndex == team {
			n++
		}
	}
	return n
}

func (r *DissectReader) PlayerStats(roundWinTeamIndex int) []PlayerRoundStats {
	stats := make([]PlayerRoundStats, 0)
	index := make(map[string]int)
	for i, p := range r.Header.Players {
		stats = append(stats, PlayerRoundStats{
			Username:  p.Username,
			TeamIndex: p.TeamIndex,
		})
		index[p.Username] = i
	}
	lastDeath := -1
	for _, a := range r.Activities {
		i := index[a.Username]
		if a.Type == KILL {
			stats[i].Kills += 1
			if *a.Headshot {
				stats[i].Headshots += 1
			}
			stats[i].HeadshotPercentage = (float64(stats[i].Headshots) / float64(stats[i].Kills)) * 100
			stats[index[a.Target]].Died = true
			lastDeath = index[a.Target]
		} else if a.Type == DEATH {
			stats[i].Died = true
			lastDeath = i
		}
	}
	// Calculates 1vX
	winnersLeftAlive := make([]int, 0)
	lastDeathWasWinner := false
	for i, p := range r.Header.Players {
		if p.TeamIndex != roundWinTeamIndex {
			continue
		}
		if !stats[i].Died {
			winnersLeftAlive = append(winnersLeftAlive, i)
		}
		if i == lastDeath {
			lastDeathWasWinner = true
		}
	}
	nWinnersLeftAlive := len(winnersLeftAlive)
	lastWinnerStanding := -1
	if nWinnersLeftAlive == 1 {
		lastWinnerStanding = winnersLeftAlive[0]
	} else if nWinnersLeftAlive == 0 && lastDeathWasWinner {
		lastWinnerStanding = lastDeath
	}
	if lastWinnerStanding > -1 {
		username := stats[lastWinnerStanding].Username
		teamLeft := r.NumPlayers(roundWinTeamIndex)
		oneVx := 0
		for _, a := range r.Activities {
			if a.Type == KILL && stats[index[a.Target]].TeamIndex == roundWinTeamIndex {
				teamLeft--
			} else if a.Type == DEATH && stats[index[a.Username]].TeamIndex == roundWinTeamIndex {
				teamLeft--
			} else if a.Type == PLAYER_LEAVE && stats[index[a.Username]].TeamIndex == roundWinTeamIndex {
				teamLeft--
			}
			if a.Username != username {
				continue
			}
			if a.Type == KILL && teamLeft < 2 {
				oneVx++
			}
		}
		for _, s := range stats {
			if s.TeamIndex != roundWinTeamIndex && !s.Died {
				oneVx++
			}
		}
		stats[lastWinnerStanding].OneVx = oneVx
	}
	return stats
}
