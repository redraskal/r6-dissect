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
	scorePattern    = []byte{0xEC, 0xDA, 0x4F, 0x80}
	timePattern     = []byte{0x1F, 0x07, 0xEF, 0xC9}
	positionPattern = []byte{0x60, 0x73, 0x85, 0xFE}
	feedbackPattern = []byte{0x59, 0x34, 0xE5, 0x8B, 0x04}
	playerPattern   = []byte{0x22, 0x07, 0x94, 0x9B, 0xDC}
)

type report struct {
	Root              string            `json:"root"`
	StartedAt         time.Time         `json:"startedAt"`
	RoundFiles        int               `json:"roundFiles"`
	ParsedFiles       int               `json:"parsedFiles"`
	ErroredFiles      int               `json:"erroredFiles"`
	PreEventBytes     int               `json:"preEventBytes"`
	GapBeforeBytes    int               `json:"gapBeforeBytes"`
	PostEventBytes    int               `json:"postEventBytes"`
	DeploymentWindows int               `json:"deploymentWindows"`
	GroupWindows      map[string]int    `json:"groupWindows"`
	Notes             []string          `json:"notes"`
	Candidates        []candidateOutput `json:"candidates"`
	Errors            []scanError       `json:"errors,omitempty"`
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

type tokenKey struct {
	Width int
	Hex   string
}

type tokenSample struct {
	File       string `json:"file"`
	Offset     int    `json:"offset"`
	Time       uint32 `json:"time"`
	Operator   string `json:"operator"`
	GadgetType string `json:"gadgetType"`
}

type tokenStats struct {
	Windows int           `json:"windows"`
	Samples []tokenSample `json:"samples,omitempty"`
}

type candidateOutput struct {
	Group        string        `json:"group"`
	Width        int           `json:"width"`
	Hex          string        `json:"hex"`
	Uint32LE     *uint32       `json:"uint32LE,omitempty"`
	Uint32BE     *uint32       `json:"uint32BE,omitempty"`
	TargetCount  int           `json:"targetCount"`
	OtherCount   int           `json:"otherCount"`
	Specificity  float64       `json:"specificity"`
	LikelyPacket string        `json:"likelyPacket,omitempty"`
	Samples      []tokenSample `json:"samples,omitempty"`
}

func main() {
	root := flag.String("root", "", "root folder containing .rec files")
	out := flag.String("out", filepath.Join("exports", "inventory_packet_candidates.json"), "JSON report output path")
	limit := flag.Int("limit", 0, "maximum number of .rec files to scan; 0 scans all")
	preEventBytes := flag.Int("pre", 8192, "bytes before a gadget deployment score packet to mine")
	gapBeforeBytes := flag.Int("gap", 512, "bytes immediately before a gadget deployment score packet to skip")
	postEventBytes := flag.Int("post", 256, "bytes after a gadget deployment score packet to mine")
	minCount := flag.Int("min-count", 12, "minimum target-window count to include a candidate")
	maxCandidates := flag.Int("max-candidates", 20, "maximum candidates per group")
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
		PreEventBytes:  *preEventBytes,
		GapBeforeBytes: *gapBeforeBytes,
		PostEventBytes: *postEventBytes,
		GroupWindows:   make(map[string]int),
		Notes: []string{
			"This scan looks for packet-like byte sequences before gadget deployment score events.",
			"A true held/inventory packet should appear before use/deploy and be enriched for a gadget/operator group, not just around every score packet.",
			"Candidates are comparative leads, not confirmed inventory fields, until a listener can decode player and item state from the bytes.",
			"The scanner masks known score packet neighborhoods and filters known time/position/feedback/player patterns.",
		},
	}

	groupTokens := make(map[string]map[tokenKey]*tokenStats)
	totalTokenWindows := make(map[tokenKey]int)

	for _, file := range files {
		if err := scanFile(file, *root, *preEventBytes, *gapBeforeBytes, *postEventBytes, &rep, groupTokens, totalTokenWindows); err != nil {
			rep.ErroredFiles++
			if len(rep.Errors) < 100 {
				rep.Errors = append(rep.Errors, scanError{File: file, Error: err.Error()})
			}
			continue
		}
		rep.ParsedFiles++
	}

	rep.Candidates = buildCandidates(groupTokens, totalTokenWindows, *minCount, *maxCandidates)
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

func scanFile(file string, root string, preBytes int, gapBeforeBytes int, postBytes int, rep *report, groupTokens map[string]map[tokenKey]*tokenStats, totalTokenWindows map[tokenKey]int) error {
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
	rawBytes := raw.Bytes()
	scores := extractScoreEvents(rawBytes)
	blockedScoreBytes := scorePacketMask(len(rawBytes), scores)
	deployScoreOffsets := scoreOffsetsByTime(scores, 20)

	if err := r.Read(); !dissect.Ok(err) {
		return err
	}

	for _, deployment := range r.GadgetDeployments {
		timeSeconds := uint32(deployment.TimeInSeconds)
		offset, ok := consumeScoreOffset(deployScoreOffsets, timeSeconds)
		if !ok {
			continue
		}

		groups := deploymentGroups(deployment)
		if len(groups) == 0 {
			continue
		}

		rel, err := filepath.Rel(root, file)
		if err != nil {
			rel = file
		}

		sample := tokenSample{
			File:       rel,
			Time:       timeSeconds,
			Operator:   deployment.Operator,
			GadgetType: deployment.GadgetType,
		}

		tokens := tokensInWindow(rawBytes, blockedScoreBytes, offset, preBytes, gapBeforeBytes, postBytes)
		if len(tokens) == 0 {
			continue
		}

		rep.DeploymentWindows++
		for key := range tokens {
			totalTokenWindows[key]++
		}
		for _, group := range groups {
			rep.GroupWindows[group]++
			bucket := groupTokens[group]
			if bucket == nil {
				bucket = make(map[tokenKey]*tokenStats)
				groupTokens[group] = bucket
			}
			for key, tokenOffset := range tokens {
				stat := bucket[key]
				if stat == nil {
					stat = &tokenStats{}
					bucket[key] = stat
				}
				stat.Windows++
				if len(stat.Samples) < 5 {
					s := sample
					s.Offset = tokenOffset
					stat.Samples = append(stat.Samples, s)
				}
			}
		}
	}

	return nil
}

func deploymentGroups(deployment dissect.GadgetDeployment) []string {
	groups := []string{"all_gadget_deployments"}
	if droneLikeGadget(deployment.GadgetType) {
		groups = append(groups, "drone_like_gadgets")
	}
	if deployment.GadgetType != "" {
		groups = append(groups, "gadget:"+deployment.GadgetType)
	}
	if deployment.Operator != "" {
		groups = append(groups, "operator:"+deployment.Operator)
	}
	return groups
}

func droneLikeGadget(gadget string) bool {
	return strings.Contains(gadget, "Drone") ||
		strings.Contains(gadget, "Yokai") ||
		strings.Contains(gadget, "Black Eye") ||
		strings.Contains(gadget, "ARGUS") ||
		strings.Contains(gadget, "Pest Launcher")
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

func tokensInWindow(raw []byte, blocked []bool, eventOffset int, preBytes int, gapBeforeBytes int, postBytes int) map[tokenKey]int {
	start := eventOffset - preBytes
	if start < 0 {
		start = 0
	}
	preEnd := eventOffset - gapBeforeBytes
	if preEnd < start {
		preEnd = start
	}
	postStart := eventOffset
	postEnd := eventOffset + postBytes
	if postEnd > len(raw) {
		postEnd = len(raw)
	}

	tokens := make(map[tokenKey]int)
	collect := func(rangeStart int, rangeEnd int, width int) {
		for i := rangeStart; i+width <= rangeEnd; i++ {
			if overlapsBlockedBytes(blocked, i, width) {
				continue
			}
			token := raw[i : i+width]
			if !interestingToken(token) {
				continue
			}
			key := tokenKey{Width: width, Hex: strings.ToUpper(hex.EncodeToString(token))}
			if _, exists := tokens[key]; !exists {
				tokens[key] = i
			}
		}
	}
	for _, width := range []int{4, 5} {
		collect(start, preEnd, width)
		if postBytes > 0 {
			collect(postStart, postEnd, width)
		}
	}
	return tokens
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
	if nonZero < len(token)-1 || len(unique) < 3 {
		return false
	}
	for _, known := range [][]byte{scorePattern, timePattern, positionPattern, feedbackPattern, playerPattern} {
		if bytes.Contains(token, known) || bytes.Contains(known, token) {
			return false
		}
	}
	return true
}

func buildCandidates(groupTokens map[string]map[tokenKey]*tokenStats, totalTokenWindows map[tokenKey]int, minCount int, maxPerGroup int) []candidateOutput {
	out := make([]candidateOutput, 0)
	for group, tokens := range groupTokens {
		if !includedGroup(group) {
			continue
		}
		groupCandidates := make([]candidateOutput, 0)
		for key, stat := range tokens {
			if stat.Windows < minCount {
				continue
			}
			other := totalTokenWindows[key] - stat.Windows
			if other < 0 {
				other = 0
			}
			candidate := candidateOutput{
				Group:        group,
				Width:        key.Width,
				Hex:          key.Hex,
				TargetCount:  stat.Windows,
				OtherCount:   other,
				Specificity:  float64(stat.Windows) / float64(other+1),
				LikelyPacket: likelyKnownPacket(key.Hex),
				Samples:      stat.Samples,
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
			groupCandidates = append(groupCandidates, candidate)
		}
		sort.Slice(groupCandidates, func(i, j int) bool {
			if groupCandidates[i].Specificity == groupCandidates[j].Specificity {
				return groupCandidates[i].TargetCount > groupCandidates[j].TargetCount
			}
			return groupCandidates[i].Specificity > groupCandidates[j].Specificity
		})
		if len(groupCandidates) > maxPerGroup {
			groupCandidates = groupCandidates[:maxPerGroup]
		}
		out = append(out, groupCandidates...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Group == out[j].Group {
			if out[i].Specificity == out[j].Specificity {
				return out[i].TargetCount > out[j].TargetCount
			}
			return out[i].Specificity > out[j].Specificity
		}
		return out[i].Group < out[j].Group
	})
	return out
}

func includedGroup(group string) bool {
	if group == "drone_like_gadgets" || group == "all_gadget_deployments" {
		return true
	}
	return strings.HasPrefix(group, "gadget:") && droneLikeGadget(strings.TrimPrefix(group, "gadget:"))
}

func likelyKnownPacket(tokenHex string) string {
	known := map[string][]byte{
		"score":          scorePattern,
		"time":           timePattern,
		"position":       positionPattern,
		"match_feedback": feedbackPattern,
		"player":         playerPattern,
	}
	decoded, err := hex.DecodeString(tokenHex)
	if err != nil {
		return ""
	}
	for name, pattern := range known {
		if bytes.Contains(decoded, pattern) || bytes.Contains(pattern, decoded) {
			return name
		}
	}
	return ""
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
	fmt.Printf("deployment windows: %d\n", rep.DeploymentWindows)
	fmt.Printf("groups: %s\n", compactMap(rep.GroupWindows))
	fmt.Printf("candidates: %d\n", len(rep.Candidates))
	for i, candidate := range rep.Candidates {
		if i >= 12 {
			break
		}
		fmt.Printf("group=%s width=%d hex=%s target=%d other=%d specificity=%.2f\n", candidate.Group, candidate.Width, candidate.Hex, candidate.TargetCount, candidate.OtherCount, candidate.Specificity)
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
