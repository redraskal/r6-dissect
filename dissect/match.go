package dissect

import (
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/xuri/excelize/v2"
	"os"
	"path"
	"strings"
)

type MatchReader struct {
	Root   string
	files  []*os.File
	rounds []*DissectReader
	read   bool
}

func NewMatchReader(root string) (m *MatchReader, err error) {
	paths, err := listReplayFiles(root)
	if err != nil {
		return
	}
	m = &MatchReader{
		Root: root,
	}
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			return m, err
		}
		r, err := NewReader(f)
		if err != nil {
			return m, err
		}
		if err := r.ReadPartial(); !Ok(err) {
			return m, err
		}
		m.rounds = append(m.rounds, r)
	}
	return
}

func (m *MatchReader) Read() error {
	total := m.NumRounds()
	for i, r := range m.rounds {
		log.Info().Msgf("Reading round %d/%d...", i+1, total)
		if err := r.Read(); !Ok(err) {
			return err
		}
	}
	m.read = true
	return nil
}

func (m *MatchReader) FirstRound() *DissectReader {
	return m.RoundAt(0)
}

func (m *MatchReader) LastRound() *DissectReader {
	return m.RoundAt(m.NumRounds() - 1)
}

func (m *MatchReader) RoundAt(i int) *DissectReader {
	return m.rounds[i]
}

func (m *MatchReader) NumRounds() int {
	return len(m.rounds)
}

func (m *MatchReader) WinningTeamIndex(round int) int {
	if m.NumRounds() == 0 || !m.read {
		return -1
	}
	teams := m.RoundAt(round).Header.Teams
	if round == 0 {
		if teams[0].Score > teams[1].Score {
			return 0
		}
		return 1
	}
	previous := m.RoundAt(round - 1).Header.Teams
	if teams[0].Score > previous[0].Score {
		return 0
	}
	return 1
}

func (m *MatchReader) Export(path string) error {
	f := excelize.NewFile()
	defer f.Close()
	first, err := f.NewSheet("Match")
	if err := f.DeleteSheet("Sheet1"); err != nil {
		return err
	}
	if err != nil {
		return err
	}
	c := newExcelCompass(f, "Match")
	for i, r := range m.rounds {
		sheet := fmt.Sprintf("Round %d", i+1)
		_, err := f.NewSheet(sheet)
		if err != nil {
			return err
		}
		c.Sheet(sheet)
		// Conditional stats
		openingKill := r.OpeningKill()
		openingDeath := r.OpeningDeath()
		openingDeathUsername := openingDeath.Username
		if openingDeath.Type == KILL {
			openingDeathUsername = openingDeath.Target
		}
		c.Heading("Statistics")
		c.Down(1).Str("Player")
		c.Right(1).Str("Team Index")
		c.Right(1).Str("Kills")
		c.Right(1).Str("Died")
		c.Right(1).Str("Assists (TODO)")
		c.Right(1).Str("Hs%")
		c.Right(1).Str("Headshots")
		c.Right(1).Str("1vX")
		c.Right(1).Str("Operator")
		winningTeamIndex := m.WinningTeamIndex(i)
		for _, s := range r.PlayerStats(winningTeamIndex) {
			c.Down(1).Left(8).Str(s.Username)
			c.Right(1).Int(s.TeamIndex)
			c.Right(1).Int(s.Kills)
			c.Right(1).Bool(s.Died)
			c.Right(1).Int(s.Assists)
			c.Right(1).Float(s.HeadshotPercentage, 3)
			c.Right(1).Int(s.Headshots)
			c.Right(1).Int(s.OneVx)
			c.Right(1).Str(s.Operator)
			log.Debug().Interface("round_player_stats", s).Send()
		}
		c.Down(2).Left(8).Heading("Other statistics")
		c.Down(1).Str("Name")
		c.Right(1).Str("Value")
		c.Right(1).Str("Time")
		c.Down(1).Left(3).Str("Winning team")
		c.Right(1).Str(fmt.Sprintf("%s [%d]", r.Header.Teams[winningTeamIndex].Name, winningTeamIndex))
		c.Down(1).Left(2).Str("Opening kill")
		c.Right(1).Str(openingKill.Username)
		c.Right(1).Str(openingKill.Time)
		c.Down(1).Left(3).Str("Opening death")
		c.Right(1).Str(openingDeathUsername)
		c.Right(1).Str(openingDeath.Time)
		c.Down(2).Left(2).Heading("Kill/death feed")
		c.Down(1).Str("Player")
		c.Right(1).Str("Target")
		c.Right(1).Str("Time")
		c.Right(1).Str("Headshot")
		for _, a := range r.KillsAndDeaths() {
			c.Down(1).Left(3)
			if a.Type == KILL {
				c.Str(a.Username)
				c.Right(1).Str(a.Target)
			} else {
				c.Str(a.Username)
				c.Right(1).Str("")
			}
			c.Right(1).Str(a.Time)
			headshot := false
			if a.Type == KILL && *a.Headshot {
				headshot = true
			}
			c.Right(1).Bool(headshot)
		}
		c.Reset().Right(10).Heading("Trades")
		c.Down(1).Str("Player 1")
		c.Right(1).Str("Player 2")
		c.Right(1).Str("Time")
		trades := r.Trades()
		for _, trade := range trades {
			c.Down(1).Left(3).Str(trade[0].Username)
			c.Right(1).Str(trade[0].Target)
			c.Right(1).Str(trade[0].Time)
		}
	}
	c.Sheet("Match")
	c.Heading("Statistics")
	c.Down(1).Str("Player")
	c.Right(1).Str("Team Index")
	c.Right(1).Str("Rounds")
	c.Right(1).Str("Kills")
	c.Right(1).Str("Deaths")
	c.Right(1).Str("Assists (TODO)")
	c.Right(1).Str("Hs%")
	c.Right(1).Str("Headshots")
	for _, s := range m.PlayerStats() {
		c.Down(1).Left(8).Str(s.Username)
		c.Right(1).Int(s.TeamIndex)
		c.Right(1).Int(s.Rounds)
		c.Right(1).Int(s.Kills)
		c.Right(1).Int(s.Deaths)
		c.Right(1).Int(s.Assists)
		c.Right(1).Float(s.HeadshotPercentage, 3)
		c.Right(1).Int(s.Headshots)
		log.Debug().Interface("match_player_stats", s).Send()
	}
	f.SetActiveSheet(first)
	return f.SaveAs(path)
}

func (m *MatchReader) ExportJSON(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return err
	}
	defer f.Close()
	type round struct {
		Header       Header     `json:"header"`
		ActivityFeed []Activity `json:"activityFeed"`
	}
	type output struct {
		Rounds []round `json:"rounds"`
	}
	rounds := make([]round, 0)
	for _, r := range m.rounds {
		rounds = append(rounds, round{
			Header:       r.Header,
			ActivityFeed: r.Activities,
		})
	}
	encoder := json.NewEncoder(f)
	return encoder.Encode(output{
		Rounds: rounds,
	})
}

func (m *MatchReader) Close() error {
	for _, f := range m.files {
		if err := f.Close(); err != nil {
			return err
		}
	}
	return nil
}

func listReplayFiles(root string) ([]string, error) {
	files, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0)
	for _, file := range files {
		name := file.Name()
		if file.Type().IsDir() || !strings.HasSuffix(name, ".rec") {
			continue
		}
		paths = append(paths, path.Join(root, name))
	}
	if len(paths) == 0 {
		return paths, ErrInvalidFolder
	}
	return paths, nil
}
