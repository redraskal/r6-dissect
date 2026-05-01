package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/redraskal/r6-dissect/dissect"
	"github.com/rs/zerolog"
)

var (
	scorePattern = []byte{0xEC, 0xDA, 0x4F, 0x80}
	timePattern  = []byte{0x1F, 0x07, 0xEF, 0xC9}
)

type report struct {
	Root             string              `json:"root"`
	StartedAt        time.Time           `json:"startedAt"`
	RoundFiles       int                 `json:"roundFiles"`
	ParsedFiles      int                 `json:"parsedFiles"`
	ErroredFiles     int                 `json:"erroredFiles"`
	DeploymentEvents int                 `json:"deploymentEvents"`
	NoDeployStates   int                 `json:"noDeployStates"`
	GadgetCounts     map[string]int      `json:"gadgetCounts"`
	OperatorCounts   map[string]int      `json:"operatorCounts"`
	Notes            []string            `json:"notes"`
	Deployments      []deploymentEvent   `json:"deployments"`
	PlayerStates     []playerGadgetState `json:"playerStates"`
	Errors           []scanError         `json:"errors,omitempty"`
}

type scanError struct {
	File  string `json:"file"`
	Error string `json:"error"`
}

type timeMark struct {
	Offset  int
	Seconds uint32
}

type scoreEvent struct {
	Offset int
	Delta  uint32
	Time   uint32
}

type deploymentEvent struct {
	File          string  `json:"file"`
	Round         int     `json:"round"`
	MatchID       string  `json:"matchID,omitempty"`
	Map           string  `json:"map,omitempty"`
	Site          string  `json:"site,omitempty"`
	Username      string  `json:"username"`
	Operator      string  `json:"operator"`
	GadgetType    string  `json:"gadgetType"`
	TimeInSeconds float64 `json:"time"`
	ScoreOffset   int     `json:"scoreOffset"`
}

type playerGadgetState struct {
	File                string   `json:"file"`
	Round               int      `json:"round"`
	MatchID             string   `json:"matchID,omitempty"`
	Map                 string   `json:"map,omitempty"`
	Site                string   `json:"site,omitempty"`
	Username            string   `json:"username"`
	Operator            string   `json:"operator"`
	GadgetType          string   `json:"gadgetType"`
	TeamRole            string   `json:"teamRole,omitempty"`
	DeploymentsObserved int      `json:"deploymentsObserved"`
	FirstDeploymentTime *float64 `json:"firstDeploymentTime,omitempty"`
	LastDeploymentTime  *float64 `json:"lastDeploymentTime,omitempty"`
	Inference           string   `json:"inference"`
}

func main() {
	root := flag.String("root", "", "root folder containing .rec files")
	out := flag.String("out", filepath.Join("exports", "deployment_timeline.json"), "JSON report output path")
	limit := flag.Int("limit", 0, "maximum number of .rec files to scan; 0 scans all")
	flag.Parse()
	zerolog.SetGlobalLevel(zerolog.Disabled)

	if *root == "" {
		fmt.Fprintln(os.Stderr, "-root is required")
		os.Exit(2)
	}

	files, err := replayFiles(*root, *limit)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rep := report{
		Root:           *root,
		StartedAt:      time.Now(),
		RoundFiles:     len(files),
		GadgetCounts:   make(map[string]int),
		OperatorCounts: make(map[string]int),
		Notes: []string{
			"Deployments are parsed from +20 score events that r6-dissect classifies as gadget deployments.",
			"ScoreOffset points to the raw decompressed scoreboard score packet used to anchor the deployment time.",
			"A no_deploy_observed state means the operator's primary gadget was not seen deploying in that round; this supports a held/pocket inference, not a confirmed active-hand packet.",
		},
	}

	for _, file := range files {
		if err := scanFile(file, *root, &rep); err != nil {
			rep.ErroredFiles++
			if len(rep.Errors) < 100 {
				rep.Errors = append(rep.Errors, scanError{File: file, Error: err.Error()})
			}
			continue
		}
		rep.ParsedFiles++
	}

	sort.Slice(rep.Deployments, func(i, j int) bool {
		if rep.Deployments[i].File == rep.Deployments[j].File {
			return rep.Deployments[i].TimeInSeconds > rep.Deployments[j].TimeInSeconds
		}
		return rep.Deployments[i].File < rep.Deployments[j].File
	})
	sort.Slice(rep.PlayerStates, func(i, j int) bool {
		if rep.PlayerStates[i].File == rep.PlayerStates[j].File {
			return rep.PlayerStates[i].Username < rep.PlayerStates[j].Username
		}
		return rep.PlayerStates[i].File < rep.PlayerStates[j].File
	})

	if err := writeReport(*out, rep); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printSummary(*out, rep)
}

func replayFiles(root string, limit int) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".rec") {
			return nil
		}
		files = append(files, path)
		if limit > 0 && len(files) >= limit {
			return filepath.SkipAll
		}
		return nil
	})
	return files, err
}

func scanFile(file string, root string, rep *report) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	r, err := dissect.NewReader(f)
	if err != nil {
		return err
	}

	var raw bytes.Buffer
	if _, err := r.Write(&raw); err != nil {
		return err
	}
	deployScoreOffsets := scoreOffsetsByTime(extractScoreEvents(raw.Bytes()), 20)

	if err := r.Read(); !dissect.Ok(err) {
		return err
	}

	rel, err := filepath.Rel(root, file)
	if err != nil {
		rel = file
	}
	round := roundFromPath(file)
	deploymentsByUser := make(map[string][]deploymentEvent)

	for _, deployment := range r.GadgetDeployments {
		timeSeconds := uint32(deployment.TimeInSeconds)
		offset, _ := consumeScoreOffset(deployScoreOffsets, timeSeconds)
		event := deploymentEvent{
			File:          rel,
			Round:         round,
			MatchID:       r.Header.MatchID,
			Map:           r.Header.Map.String(),
			Site:          r.Header.Site,
			Username:      deployment.Username,
			Operator:      deployment.Operator,
			GadgetType:    deployment.GadgetType,
			TimeInSeconds: deployment.TimeInSeconds,
			ScoreOffset:   offset,
		}
		rep.Deployments = append(rep.Deployments, event)
		rep.DeploymentEvents++
		rep.GadgetCounts[deployment.GadgetType]++
		rep.OperatorCounts[deployment.Operator]++
		deploymentsByUser[deployment.Username] = append(deploymentsByUser[deployment.Username], event)
	}

	for _, player := range r.Header.Players {
		operator := player.Operator.String()
		gadget := dissect.ClassifyGadgetByOperator(operator)
		if operator == "Recruit" || gadget == "" || gadget == "Recruit" {
			continue
		}

		state := playerGadgetState{
			File:       rel,
			Round:      round,
			MatchID:    r.Header.MatchID,
			Map:        r.Header.Map.String(),
			Site:       r.Header.Site,
			Username:   player.Username,
			Operator:   operator,
			GadgetType: gadget,
			TeamRole:   teamRoleForPlayer(r, player),
		}

		deployments := deploymentsByUser[player.Username]
		state.DeploymentsObserved = len(deployments)
		if len(deployments) == 0 {
			state.Inference = "no_deploy_observed_possible_held_or_pocket"
			rep.NoDeployStates++
		} else {
			state.Inference = "deployment_observed"
			first := deployments[0].TimeInSeconds
			last := deployments[0].TimeInSeconds
			for _, deployment := range deployments[1:] {
				if deployment.TimeInSeconds > first {
					first = deployment.TimeInSeconds
				}
				if deployment.TimeInSeconds < last {
					last = deployment.TimeInSeconds
				}
			}
			state.FirstDeploymentTime = &first
			state.LastDeploymentTime = &last
		}
		rep.PlayerStates = append(rep.PlayerStates, state)
	}

	return nil
}

func extractScoreEvents(raw []byte) []scoreEvent {
	times := extractTimeMarks(raw)
	events := make([]scoreEvent, 0)
	prevByID := make(map[string]uint32)
	timeIndex := 0

	for searchAt := 0; searchAt < len(raw); {
		rel := bytes.Index(raw[searchAt:], scorePattern)
		if rel < 0 {
			break
		}
		pos := searchAt + rel
		searchAt = pos + 1
		if pos+len(scorePattern)+5 > len(raw) {
			continue
		}

		for timeIndex+1 < len(times) && times[timeIndex+1].Offset < pos {
			timeIndex++
		}

		score := binary.LittleEndian.Uint32(raw[pos+len(scorePattern)+1 : pos+len(scorePattern)+5])
		if score == 0 || score > 20000 {
			continue
		}

		id := fmt.Sprintf("offset:%08x", pos)
		if pos >= 9 && raw[pos-9] == 0x23 {
			id = hex.EncodeToString(raw[pos-8 : pos-4])
		}

		prev := prevByID[id]
		prevByID[id] = score
		if prev == 0 || score <= prev {
			continue
		}

		eventTime := uint32(0)
		if len(times) > 0 && times[timeIndex].Offset < pos {
			eventTime = times[timeIndex].Seconds
		}
		events = append(events, scoreEvent{Offset: pos, Delta: score - prev, Time: eventTime})
	}

	return events
}

func extractTimeMarks(raw []byte) []timeMark {
	marks := make([]timeMark, 0)
	for searchAt := 0; searchAt < len(raw); {
		rel := bytes.Index(raw[searchAt:], timePattern)
		if rel < 0 {
			break
		}
		pos := searchAt + rel
		searchAt = pos + 1
		if pos+len(timePattern)+5 > len(raw) {
			continue
		}
		seconds := binary.LittleEndian.Uint32(raw[pos+len(timePattern)+1 : pos+len(timePattern)+5])
		if seconds > 900 {
			continue
		}
		marks = append(marks, timeMark{Offset: pos, Seconds: seconds})
	}
	return marks
}

func scoreOffsetsByTime(scores []scoreEvent, delta uint32) map[uint32][]int {
	out := make(map[uint32][]int)
	for _, score := range scores {
		if score.Delta == delta {
			out[score.Time] = append(out[score.Time], score.Offset)
		}
	}
	for timeSeconds := range out {
		sort.Ints(out[timeSeconds])
	}
	return out
}

func consumeScoreOffset(offsets map[uint32][]int, timeSeconds uint32) (int, bool) {
	list := offsets[timeSeconds]
	if len(list) == 0 {
		return 0, false
	}
	offset := list[0]
	offsets[timeSeconds] = list[1:]
	return offset, true
}

func teamRoleForPlayer(r *dissect.Reader, player dissect.Player) string {
	if player.TeamIndex < 0 || player.TeamIndex >= len(r.Header.Teams) {
		return ""
	}
	return string(r.Header.Teams[player.TeamIndex].Role)
}

func roundFromPath(file string) int {
	base := filepath.Base(file)
	marker := "-R"
	idx := strings.LastIndex(base, marker)
	if idx < 0 || idx+len(marker)+2 > len(base) {
		return 0
	}
	var round int
	_, _ = fmt.Sscanf(base[idx+len(marker):], "%02d", &round)
	return round
}

func writeReport(out string, rep report) error {
	if err := os.MkdirAll(filepath.Dir(out), os.ModePerm); err != nil {
		return err
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	return encoder.Encode(rep)
}

func printSummary(out string, rep report) {
	fmt.Printf("wrote %s\n", out)
	fmt.Printf("round files: %d, parsed: %d, errors: %d\n", rep.RoundFiles, rep.ParsedFiles, rep.ErroredFiles)
	fmt.Printf("deployment events: %d\n", rep.DeploymentEvents)
	fmt.Printf("no-deploy states: %d\n", rep.NoDeployStates)
	fmt.Printf("top gadgets: %s\n", topMap(rep.GadgetCounts, 10))
	for i, deployment := range rep.Deployments {
		if i >= 8 {
			break
		}
		fmt.Printf("deploy time=%.0f file=%s user=%s op=%s gadget=%s offset=%d\n", deployment.TimeInSeconds, deployment.File, deployment.Username, deployment.Operator, deployment.GadgetType, deployment.ScoreOffset)
	}
}

func topMap(m map[string]int, max int) string {
	type pair struct {
		key   string
		value int
	}
	pairs := make([]pair, 0, len(m))
	for key, value := range m {
		pairs = append(pairs, pair{key: key, value: value})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].value == pairs[j].value {
			return pairs[i].key < pairs[j].key
		}
		return pairs[i].value > pairs[j].value
	})
	if len(pairs) > max {
		pairs = pairs[:max]
	}
	parts := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		parts = append(parts, fmt.Sprintf("%s=%d", pair.key, pair.value))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
