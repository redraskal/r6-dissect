package dissect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/xuri/excelize/v2"
)

type MatchReader struct {
	Root   *os.File
	paths  []string
	rounds []*Reader

	queries   [][]byte
	listeners [][]func(r *Reader) error
}

func NewMatchReader(in *os.File) (m *MatchReader, err error) {
	paths, err := ListReplayFiles(in)
	if err != nil {
		return
	}
	m = &MatchReader{
		Root:   in,
		paths:  paths,
		rounds: make([]*Reader, len(paths)),
	}
	return
}

// Listen registers a callback to be run during round Read whenever
// the pattern is found.
func (m *MatchReader) Listen(pattern []byte, callback func(r *Reader) error) {
	var i int
	for i = 0; i < len(m.queries); i++ {
		if bytes.Equal(m.queries[i], pattern) {
			m.listeners[i] = append(m.listeners[i], callback)
			break
		}
	}
	m.queries = append(m.queries, pattern)
	m.listeners = append(m.listeners, []func(reader *Reader) error{callback})
}

func (m *MatchReader) read(i int) error {
	if i < 0 || i >= len(m.paths) {
		return ErrInvalidFile
	}
	if m.rounds[i] != nil {
		return nil
	}
	f, err := os.Open(m.paths[i])
	if err != nil {
		return err
	}
	defer f.Close()
	r, err := NewReader(f)
	if err != nil {
		return err
	}
	m.rounds[i] = r
	for i = 0; i < len(m.queries); i++ {
		for _, listener := range m.listeners[i] {
			r.Listen(m.queries[i], listener)
		}
	}
	return r.Read()
}

func (m *MatchReader) Read() error {
	for i := range m.paths {
		if err := m.read(i); err != nil {
			return err
		}
	}
	return nil
}

func (m *MatchReader) FirstRound() (r *Reader, err error) {
	return m.RoundAt(0)
}

func (m *MatchReader) LastRound() (r *Reader, err error) {
	return m.RoundAt(m.NumRounds() - 1)
}

func (m *MatchReader) RoundAt(i int) (r *Reader, err error) {
	if m.rounds[i] == nil {
		if err := m.read(i); err != nil {
			return nil, err
		}
	}
	return m.rounds[i], nil
}

func (m *MatchReader) NumRounds() int {
	return len(m.paths)
}

func (m *MatchReader) WriteExcel(out io.Writer) error {
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

		if openingDeath.Type == Kill {
			openingDeathUsername = openingDeath.Target
		}

		c.Heading("Statistics")
		c.Down(1).Str("Player")
		c.Right(1).Str("Team Index")
		c.Right(1).Str("Kills")
		c.Right(1).Str("Died")
		c.Right(1).Str("Assists")
		c.Right(1).Str("Hs%")
		c.Right(1).Str("Headshots")
		c.Right(1).Str("1vX")
		c.Right(1).Str("Operator")

		winningTeamIndex := 0
		if r.Header.Teams[1].Won {
			winningTeamIndex = 1
		}

		for _, s := range r.PlayerStats() {
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

		c.Down(2).Left(8).Heading("Round info")
		c.Down(1).Str("Name")
		c.Right(1).Str("Value")
		c.Right(1).Str("Time")
		c.Down(1).Left(2).Str("Site")
		c.Right(1).Str(r.Header.Site)
		c.Down(1).Left(1).Str("Winning team")
		c.Right(1).Str(fmt.Sprintf("%s [%d]", r.Header.Teams[winningTeamIndex].Name, winningTeamIndex))
		c.Down(1).Left(2).Str("Win condition")
		c.Right(1).Str(string(r.Header.Teams[winningTeamIndex].WinCondition))
		c.Down(1).Left(2).Str("Opening kill")
		c.Right(1).Str(openingKill.Username)
		c.Right(1).Str(openingKill.Time)
		c.Down(1).Left(3).Str("Opening death")
		c.Right(1).Str(openingDeathUsername)
		c.Right(1).Str(openingDeath.Time)

		if r.Header.GameMode == Bomb {
			var plant MatchUpdate
			var defuse MatchUpdate
			for _, update := range r.MatchFeedback {
				if update.Type == DefuserPlantComplete {
					plant = update
				} else if update.Type == DefuserDisableComplete {
					defuse = update
				}
			}
			c.Down(1).Left(2).Str("Planted at")
			if plant.Time != "" {
				c.Right(1).Str(plant.Time)
			} else {
				c.Right(1).Str("")
			}
			c.Down(1).Left(2).Str("Defused at")
			if defuse.Time != "" {
				c.Right(1).Str(defuse.Time)
			} else {
				c.Right(1).Str("")
			}
		}

		c.Down(2).Left(2).Heading("Kill/death feed")
		c.Down(1).Str("Player")
		c.Right(1).Str("Target")
		c.Right(1).Str("Time")
		c.Right(1).Str("Headshot")

		for _, a := range r.KillsAndDeaths() {
			c.Down(1).Left(3)
			if a.Type == Kill {
				c.Str(a.Username)
				c.Right(1).Str(a.Target)
			} else {
				c.Str(a.Username)
				c.Right(1).Str("")
			}
			c.Right(1).Str(a.Time)
			headshot := false
			if a.Type == Kill && *a.Headshot {
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
			c.Down(1).Left(2).Str(trade[0].Username)
			c.Right(1).Str(trade[0].Target)
			c.Right(1).Str(trade[1].Time)
		}
	}

	c.Sheet("Match")
	c.Heading("Statistics")
	c.Down(1).Str("Player")
	c.Right(1).Str("Team Index")
	c.Right(1).Str("Rounds")
	c.Right(1).Str("Kills")
	c.Right(1).Str("Deaths")
	c.Right(1).Str("Assists")
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

	return f.Write(out)
}

func (m *MatchReader) WriteJSON(out io.Writer) error {
	encoder := json.NewEncoder(out)
	return encoder.Encode(m.Data())
}

func (m *MatchReader) Data() any {
	type round struct {
		Header
		MatchFeedback []MatchUpdate      `json:"matchFeedback"`
		PlayerStats   []PlayerRoundStats `json:"stats"`
	}
	type output struct {
		Rounds      []round            `json:"rounds"`
		PlayerStats []PlayerMatchStats `json:"stats"`
	}
	rounds := make([]round, 0)
	for _, r := range m.rounds {
		rounds = append(rounds, round{
			Header:        r.Header,
			MatchFeedback: r.MatchFeedback,
			PlayerStats:   r.PlayerStats(),
		})
	}
	return output{
		Rounds:      rounds,
		PlayerStats: m.PlayerStats(),
	}
}

func ListReplayFiles(root *os.File) ([]string, error) {
	files, err := root.ReadDir(0)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0)
	for _, file := range files {
		name := file.Name()
		if file.Type().IsDir() || !strings.HasSuffix(name, ".rec") {
			continue
		}
		paths = append(paths, path.Join(root.Name(), name))
	}
	if len(paths) == 0 {
		return paths, ErrInvalidFolder
	}
	slices.Sort(paths)
	return paths, nil
}
