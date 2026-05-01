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

	tacticalDeltas = map[uint32]string{
		10: "drone_destroy_defender_score",
		15: "camera_destroy_score",
		20: "gadget_deploy_score",
		25: "utility_destroy_score",
	}
)

type report struct {
	Root           string            `json:"root"`
	StartedAt      time.Time         `json:"startedAt"`
	RoundFiles     int               `json:"roundFiles"`
	ParsedFiles    int               `json:"parsedFiles"`
	ErroredFiles   int               `json:"erroredFiles"`
	EventCounts    map[string]int    `json:"eventCounts"`
	GadgetCounts   map[string]int    `json:"gadgetCounts,omitempty"`
	OperatorCounts map[string]int    `json:"operatorCounts,omitempty"`
	ScoreDeltas    map[string]int    `json:"scoreDeltas"`
	CandidateNotes []string          `json:"candidateNotes"`
	Candidates     []candidateOutput `json:"candidates"`
	Events         []parsedEvent     `json:"events"`
	Errors         []scanError       `json:"errors,omitempty"`
}

type scanError struct {
	File  string `json:"file"`
	Error string `json:"error"`
}

type parsedEvent struct {
	Kind          string  `json:"kind"`
	File          string  `json:"file"`
	Round         int     `json:"round"`
	Username      string  `json:"username,omitempty"`
	Operator      string  `json:"operator,omitempty"`
	GadgetType    string  `json:"gadgetType,omitempty"`
	TimeInSeconds float64 `json:"time"`
}

type timeMark struct {
	Offset  int
	Seconds uint32
}

type scoreEvent struct {
	Offset    int
	Score     uint32
	PrevScore uint32
	Delta     uint32
	Time      uint32
	SBID      string
	Marker    bool
}

type tokenKey struct {
	Delta uint32
	Width int
	Hex   string
}

type tokenSample struct {
	File   string `json:"file"`
	Offset int    `json:"offset"`
	Time   uint32 `json:"time"`
}

type tokenStats struct {
	Count   int           `json:"count"`
	Samples []tokenSample `json:"samples,omitempty"`
}

type candidateOutput struct {
	Delta       uint32        `json:"delta"`
	Signal      string        `json:"signal"`
	Width       int           `json:"width"`
	Hex         string        `json:"hex"`
	Uint32LE    *uint32       `json:"uint32LE,omitempty"`
	Uint32BE    *uint32       `json:"uint32BE,omitempty"`
	Count       int           `json:"count"`
	OtherCount  int           `json:"otherCount"`
	Specificity float64       `json:"specificity"`
	Samples     []tokenSample `json:"samples,omitempty"`
}

func main() {
	root := flag.String("root", "", "root folder containing .rec files")
	out := flag.String("out", filepath.Join("exports", "gadget_hash_candidates.json"), "JSON report output path")
	limit := flag.Int("limit", 0, "maximum number of .rec files to scan; 0 scans all")
	window := flag.Int("window", 96, "bytes before and after score events to mine for candidate tokens")
	minCount := flag.Int("min-count", 8, "minimum token count to include as a candidate")
	maxCandidates := flag.Int("max-candidates", 25, "maximum candidates per score delta and token width")
	maxEvents := flag.Int("max-events", 500, "maximum parsed tactical events to include in the report")
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
		EventCounts:    make(map[string]int),
		GadgetCounts:   make(map[string]int),
		OperatorCounts: make(map[string]int),
		ScoreDeltas:    make(map[string]int),
		CandidateNotes: []string{
			"Candidate hashes are mined from raw windows around scoreboard score deltas.",
			"Candidate mining only uses score deltas whose replay time matches a parsed tactical event of the same type.",
			"Bytes belonging to the scoreboard score packet itself are skipped so player scoreboard IDs do not dominate the output.",
			"The parser currently infers drone/gadget events from score changes, not from a confirmed held-item or inventory packet.",
			"Use repeated, high-specificity candidates as leads for deeper reverse engineering.",
		},
	}

	tokens := make(map[tokenKey]*tokenStats)
	totalByToken := make(map[string]int)

	for _, file := range files {
		if err := scanFile(file, *root, *window, *maxEvents, &rep, tokens, totalByToken); err != nil {
			rep.ErroredFiles++
			if len(rep.Errors) < 100 {
				rep.Errors = append(rep.Errors, scanError{File: file, Error: err.Error()})
			}
			continue
		}
		rep.ParsedFiles++
	}

	rep.Candidates = buildCandidates(tokens, totalByToken, *minCount, *maxCandidates)
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

func scanFile(file string, root string, window int, maxEvents int, rep *report, tokens map[tokenKey]*tokenStats, totalByToken map[string]int) error {
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

	scores := extractScoreEvents(raw.Bytes())
	blockedScoreBytes := scorePacketMask(len(raw.Bytes()), scores)
	if err := r.Read(); !dissect.Ok(err) {
		return err
	}

	allowedEventTimes := tacticalEventTimes(r)
	for _, score := range scores {
		if score.Delta == 0 {
			continue
		}
		rep.ScoreDeltas[fmt.Sprint(score.Delta)]++
		if _, ok := tacticalDeltas[score.Delta]; ok && consumeEventTime(allowedEventTimes, score.Delta, score.Time) {
			addCandidateTokens(file, root, raw.Bytes(), blockedScoreBytes, score, window, tokens, totalByToken)
		}
	}

	recordParsedEvents(file, root, r, maxEvents, rep)
	return nil
}

func scorePacketMask(rawLen int, scores []scoreEvent) []bool {
	blocked := make([]bool, rawLen)
	for _, score := range scores {
		start := score.Offset - 24
		if start < 0 {
			start = 0
		}
		end := score.Offset + len(scorePattern) + 16
		if end > rawLen {
			end = rawLen
		}
		for i := start; i < end; i++ {
			blocked[i] = true
		}
	}
	return blocked
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

		marker := pos >= 9 && raw[pos-9] == 0x23
		sbID := fmt.Sprintf("offset:%08x", pos)
		if marker {
			sbID = hex.EncodeToString(raw[pos-8 : pos-4])
		}

		prev := prevByID[sbID]
		delta := uint32(0)
		if prev > 0 && score > prev {
			delta = score - prev
		}
		prevByID[sbID] = score

		eventTime := uint32(0)
		if len(times) > 0 && times[timeIndex].Offset < pos {
			eventTime = times[timeIndex].Seconds
		}

		events = append(events, scoreEvent{
			Offset:    pos,
			Score:     score,
			PrevScore: prev,
			Delta:     delta,
			Time:      eventTime,
			SBID:      sbID,
			Marker:    marker,
		})
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

func addCandidateTokens(file string, root string, raw []byte, blocked []bool, score scoreEvent, window int, tokens map[tokenKey]*tokenStats, totalByToken map[string]int) {
	start := score.Offset - window
	if start < 0 {
		start = 0
	}
	end := score.Offset + len(scorePattern) + window
	if end > len(raw) {
		end = len(raw)
	}

	rel, err := filepath.Rel(root, file)
	if err != nil {
		rel = file
	}

	for _, width := range []int{4, 5} {
		for i := start; i+width <= end; i++ {
			if overlapsBlockedBytes(blocked, i, width) {
				continue
			}
			token := raw[i : i+width]
			if !interestingToken(token) {
				continue
			}
			tokenHex := strings.ToUpper(hex.EncodeToString(token))
			key := tokenKey{Delta: score.Delta, Width: width, Hex: tokenHex}
			stat := tokens[key]
			if stat == nil {
				stat = &tokenStats{}
				tokens[key] = stat
			}
			stat.Count++
			if len(stat.Samples) < 5 {
				stat.Samples = append(stat.Samples, tokenSample{File: rel, Offset: i, Time: score.Time})
			}
			windowTokenKey := fmt.Sprintf("%d:%s", width, tokenHex)
			totalByToken[windowTokenKey]++
		}
	}
}

func overlapsBlockedBytes(blocked []bool, offset int, width int) bool {
	end := offset + width
	if offset < 0 || end > len(blocked) {
		return true
	}
	for i := offset; i < end; i++ {
		if blocked[i] {
			return true
		}
	}
	return false
}

func interestingToken(token []byte) bool {
	if len(token) == 0 {
		return false
	}
	nonZero := 0
	unique := make(map[byte]bool)
	for _, b := range token {
		if b != 0 {
			nonZero++
		}
		unique[b] = true
	}
	if nonZero < 2 || len(unique) < 3 {
		return false
	}
	if nonZero < len(token)-1 {
		return false
	}
	if bytes.Contains(token, []byte{0x23}) {
		return false
	}
	if bytes.Contains(token, scorePattern) || bytes.Contains(token, timePattern) {
		return false
	}
	return true
}

func buildCandidates(tokens map[tokenKey]*tokenStats, totalByToken map[string]int, minCount int, maxPerGroup int) []candidateOutput {
	byGroup := make(map[string][]candidateOutput)
	for key, stat := range tokens {
		if stat.Count < minCount {
			continue
		}
		totalKey := fmt.Sprintf("%d:%s", key.Width, key.Hex)
		otherCount := totalByToken[totalKey] - stat.Count
		if otherCount < 0 {
			otherCount = 0
		}
		candidate := candidateOutput{
			Delta:       key.Delta,
			Signal:      tacticalDeltas[key.Delta],
			Width:       key.Width,
			Hex:         key.Hex,
			Count:       stat.Count,
			OtherCount:  otherCount,
			Specificity: float64(stat.Count) / float64(otherCount+1),
			Samples:     stat.Samples,
		}
		if key.Width == 4 {
			decoded, err := hex.DecodeString(key.Hex)
			if err == nil && len(decoded) == 4 {
				le := binary.LittleEndian.Uint32(decoded)
				be := binary.BigEndian.Uint32(decoded)
				candidate.Uint32LE = &le
				candidate.Uint32BE = &be
			}
		}
		group := fmt.Sprintf("%d:%d", key.Delta, key.Width)
		byGroup[group] = append(byGroup[group], candidate)
	}

	out := make([]candidateOutput, 0)
	for _, group := range byGroup {
		sort.Slice(group, func(i, j int) bool {
			if group[i].Specificity == group[j].Specificity {
				return group[i].Count > group[j].Count
			}
			return group[i].Specificity > group[j].Specificity
		})
		if len(group) > maxPerGroup {
			group = group[:maxPerGroup]
		}
		out = append(out, group...)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Delta == out[j].Delta {
			if out[i].Width == out[j].Width {
				if out[i].Specificity == out[j].Specificity {
					return out[i].Count > out[j].Count
				}
				return out[i].Specificity > out[j].Specificity
			}
			return out[i].Width < out[j].Width
		}
		return out[i].Delta < out[j].Delta
	})
	return out
}

func recordParsedEvents(file string, root string, r *dissect.Reader, maxEvents int, rep *report) {
	for _, event := range r.DroneDestructions {
		rep.EventCounts["droneDestructions"]++
		appendParsedEvent(file, root, maxEvents, rep, parsedEvent{
			Kind:          "droneDestruction",
			Username:      event.DestroyedBy,
			Operator:      operatorForUser(r, event.DestroyedBy),
			TimeInSeconds: event.TimeInSeconds,
		})
	}
	for _, event := range r.GadgetDeployments {
		rep.EventCounts["gadgetDeployments"]++
		rep.GadgetCounts[event.GadgetType]++
		rep.OperatorCounts[event.Operator]++
		appendParsedEvent(file, root, maxEvents, rep, parsedEvent{
			Kind:          "gadgetDeployment",
			Username:      event.Username,
			Operator:      event.Operator,
			GadgetType:    event.GadgetType,
			TimeInSeconds: event.TimeInSeconds,
		})
	}
	for _, event := range r.UtilityEvents {
		rep.EventCounts["utilityEvents"]++
		appendParsedEvent(file, root, maxEvents, rep, parsedEvent{
			Kind:          "utilityEvent",
			Username:      event.Username,
			Operator:      event.Operator,
			GadgetType:    event.UtilityType,
			TimeInSeconds: event.TimeInSeconds,
		})
	}
	rep.EventCounts["cameraDestructions"] += len(r.CameraDestructions)
	rep.EventCounts["reinforcements"] += len(r.Reinforcements)
	rep.EventCounts["barricades"] += len(r.Barricades)
}

func tacticalEventTimes(r *dissect.Reader) map[uint32]map[uint32]int {
	out := map[uint32]map[uint32]int{
		10: make(map[uint32]int),
		15: make(map[uint32]int),
		20: make(map[uint32]int),
		25: make(map[uint32]int),
	}
	for _, event := range r.DroneDestructions {
		out[10][uint32(event.TimeInSeconds)]++
	}
	for _, event := range r.CameraDestructions {
		out[15][uint32(event.TimeInSeconds)]++
	}
	for _, event := range r.GadgetDeployments {
		out[20][uint32(event.TimeInSeconds)]++
	}
	for _, event := range r.UtilityEvents {
		out[25][uint32(event.TimeInSeconds)]++
	}
	return out
}

func consumeEventTime(times map[uint32]map[uint32]int, delta uint32, seconds uint32) bool {
	byTime := times[delta]
	if len(byTime) == 0 {
		return false
	}
	if byTime[seconds] > 0 {
		byTime[seconds]--
		return true
	}
	return false
}

func appendParsedEvent(file string, root string, maxEvents int, rep *report, event parsedEvent) {
	if len(rep.Events) >= maxEvents {
		return
	}
	rel, err := filepath.Rel(root, file)
	if err != nil {
		rel = file
	}
	event.File = rel
	event.Round = roundFromPath(file)
	rep.Events = append(rep.Events, event)
}

func operatorForUser(r *dissect.Reader, username string) string {
	for _, player := range r.Header.Players {
		if player.Username == username {
			return player.Operator.String()
		}
	}
	return ""
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
	fmt.Printf("events: %s\n", compactMap(rep.EventCounts))
	fmt.Printf("score deltas: %s\n", compactMap(rep.ScoreDeltas))
	fmt.Printf("candidates: %d\n", len(rep.Candidates))
	for i, candidate := range rep.Candidates {
		if i >= 12 {
			break
		}
		fmt.Printf("delta=%d width=%d hex=%s count=%d other=%d specificity=%.2f\n", candidate.Delta, candidate.Width, candidate.Hex, candidate.Count, candidate.OtherCount, candidate.Specificity)
	}
}

func compactMap[K ~string, V ~int](m map[K]V) string {
	if len(m) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, string(key))
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, m[K(key)]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
