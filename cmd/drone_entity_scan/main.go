package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/redraskal/r6-dissect/dissect"
	"github.com/rs/zerolog"
)

var (
	timePattern     = []byte{0x1F, 0x07, 0xEF, 0xC9}
	positionPattern = []byte{0x60, 0x73, 0x85, 0xFE}
)

type report struct {
	Root                  string             `json:"root"`
	StartedAt             time.Time          `json:"startedAt"`
	RoundFiles            int                `json:"roundFiles"`
	ParsedFiles           int                `json:"parsedFiles"`
	ErroredFiles          int                `json:"erroredFiles"`
	MinSamples            int                `json:"minSamples"`
	DestructionWindowSec  float64            `json:"destructionWindowSec"`
	PickupDistance        float64            `json:"pickupDistance"`
	NonPlayerTracks       int                `json:"nonPlayerTracks"`
	DestructionEvents     int                `json:"destructionEvents"`
	DestructionMatches    int                `json:"destructionMatches"`
	PickupCandidates      int                `json:"pickupCandidates"`
	PrepPickupCandidates  int                `json:"prepPickupCandidates"`
	PlayerDroneStates     int                `json:"playerDroneStates"`
	DirectOwnerHints      int                `json:"directOwnerHints"`
	EntityEndTimeCounts   map[string]int     `json:"entityEndTimeCounts"`
	PickupTimeCounts      map[string]int     `json:"pickupTimeCounts"`
	PickupEntityCounts    map[string]int     `json:"pickupEntityCounts"`
	DestructionTimeCounts map[string]int     `json:"destructionTimeCounts"`
	Notes                 []string           `json:"notes"`
	DroneInventory        []droneInventory   `json:"droneInventory,omitempty"`
	RoundSamples          []roundSample      `json:"roundSamples,omitempty"`
	EntitySamples         []entitySummary    `json:"entitySamples,omitempty"`
	DestructionEvidence   []destructionMatch `json:"destructionEvidence,omitempty"`
	PickupEvidence        []pickupCandidate  `json:"pickupEvidence,omitempty"`
	OwnerHintEvidence     []ownerHint        `json:"ownerHintEvidence,omitempty"`
	Errors                []scanError        `json:"errors,omitempty"`
}

type scanError struct {
	File  string `json:"file"`
	Error string `json:"error"`
}

type timeMark struct {
	Offset  int
	Seconds uint32
}

type positionSample struct {
	Offset int
	Time   uint32
	X      float64
	Y      float64
	Z      float64
}

type entityTrack struct {
	EntityID byte
	Samples  []positionSample
	Distance float64
}

type playerTrack struct {
	Username  string
	Operator  string
	TeamRole  string
	EntityID  byte
	DissectID []byte
	Track     *entityTrack
}

type roundSample struct {
	File              string `json:"file"`
	Round             int    `json:"round"`
	MatchID           string `json:"matchID,omitempty"`
	Players           int    `json:"players"`
	PositionEntities  int    `json:"positionEntities"`
	NonPlayerEntities int    `json:"nonPlayerEntities"`
	DroneDestructions int    `json:"droneDestructions"`
}

type entitySummary struct {
	File                 string   `json:"file"`
	Round                int      `json:"round"`
	EntityID             int      `json:"entityId"`
	Samples              int      `json:"samples"`
	FirstTime            uint32   `json:"firstTime"`
	LastTime             uint32   `json:"lastTime"`
	FirstOffset          int      `json:"firstOffset"`
	LastOffset           int      `json:"lastOffset"`
	Distance             float64  `json:"distance"`
	NearestEndPlayer     string   `json:"nearestEndPlayer,omitempty"`
	NearestEndTeamRole   string   `json:"nearestEndTeamRole,omitempty"`
	NearestEndDistance   *float64 `json:"nearestEndDistance,omitempty"`
	NearDroneDestruction bool     `json:"nearDroneDestruction"`
}

type destructionMatch struct {
	File                string   `json:"file"`
	Round               int      `json:"round"`
	DestroyedBy         string   `json:"destroyedBy"`
	Time                float64  `json:"time"`
	EntityID            int      `json:"entityId"`
	EntityLastTime      uint32   `json:"entityLastTime"`
	TimeDelta           float64  `json:"timeDelta"`
	LastOffset          int      `json:"lastOffset"`
	Samples             int      `json:"samples"`
	Distance            float64  `json:"distance"`
	DistanceToDestroyer *float64 `json:"distanceToDestroyer,omitempty"`
}

type pickupCandidate struct {
	File             string  `json:"file"`
	Round            int     `json:"round"`
	EntityID         int     `json:"entityId"`
	LastTime         uint32  `json:"lastTime"`
	LastOffset       int     `json:"lastOffset"`
	Samples          int     `json:"samples"`
	Distance         float64 `json:"distance"`
	NearestAttacker  string  `json:"nearestAttacker"`
	AttackerOperator string  `json:"attackerOperator"`
	DistanceToPlayer float64 `json:"distanceToPlayer"`
}

type droneInventory struct {
	File                          string   `json:"file"`
	Round                         int      `json:"round"`
	MatchID                       string   `json:"matchID,omitempty"`
	Username                      string   `json:"username"`
	Operator                      string   `json:"operator"`
	PrepPickupCandidateCount      int      `json:"prepPickupCandidateCount"`
	PrepPickupEntityIDs           []int    `json:"prepPickupEntityIds,omitempty"`
	BestPickupDistance            *float64 `json:"bestPickupDistance,omitempty"`
	PocketDronesAfterPrepEstimate int      `json:"pocketDronesAfterPrepEstimate"`
	Inference                     string   `json:"inference"`
	Confidence                    string   `json:"confidence"`
}

type ownerHint struct {
	File         string `json:"file"`
	Round        int    `json:"round"`
	EntityID     int    `json:"entityId"`
	Username     string `json:"username"`
	Anchor       string `json:"anchor"`
	AnchorTime   uint32 `json:"anchorTime"`
	AnchorOffset int    `json:"anchorOffset"`
	HintOffset   int    `json:"hintOffset"`
	Pattern      string `json:"pattern"`
}

func main() {
	root := flag.String("root", "", "root folder containing .rec files")
	out := flag.String("out", filepath.Join("exports", "drone_entity_candidates.json"), "JSON report output path")
	limit := flag.Int("limit", 0, "maximum number of .rec files to scan; 0 scans all")
	minSamples := flag.Int("min-samples", 8, "minimum position samples for a non-player entity candidate")
	destructionWindow := flag.Float64("destruction-window", 2.5, "seconds around a drone destruction where an entity ending is treated as correlated")
	pickupDistance := flag.Float64("pickup-distance", 4.0, "maximum distance between an ending entity and attacker for pickup candidate output")
	maxSamples := flag.Int("max-samples", 400, "maximum evidence rows to retain for each evidence list")
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
		Root:                  *root,
		StartedAt:             time.Now(),
		RoundFiles:            len(files),
		MinSamples:            *minSamples,
		DestructionWindowSec:  *destructionWindow,
		PickupDistance:        *pickupDistance,
		EntityEndTimeCounts:   make(map[string]int),
		PickupTimeCounts:      make(map[string]int),
		PickupEntityCounts:    make(map[string]int),
		DestructionTimeCounts: make(map[string]int),
		Notes: []string{
			"This is an experimental raw position-entity probe, not a confirmed drone inventory decoder.",
			"Player entity IDs are inferred the same way the existing movement parser does: lowest valid position entity IDs map to header player order.",
			"A destruction match means a non-player position entity stopped updating near a parsed +10 defender drone-destruction score event.",
			"A pickup candidate means a non-player entity stopped updating near an attacker and not near a parsed drone destruction; this can suggest pickup/despawn but does not prove ownership.",
			"DroneInventory estimates pocket drones after prep phase: base 1, plus 1 when a prep-transition entity ending at 44s is close to that attacker.",
			"DroneInventory does not prove total drone ownership in the world; world drone ownership still requires a direct entity-owner packet.",
			"OwnerHintEvidence records player DissectID bytes found near candidate entity position packets; absence of these hints means ownership is proximity-inferred, not packet-decoded.",
		},
	}

	for _, file := range files {
		if err := scanFile(file, *root, *minSamples, *destructionWindow, *pickupDistance, *maxSamples, &rep); err != nil {
			rep.ErroredFiles++
			if len(rep.Errors) < 100 {
				rep.Errors = append(rep.Errors, scanError{File: file, Error: err.Error()})
			}
			continue
		}
		rep.ParsedFiles++
	}

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

func scanFile(file string, root string, minSamples int, destructionWindow float64, pickupDistance float64, maxSamples int, rep *report) error {
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
	tracks := extractPositionTracks(rawBytes)

	if err := r.Read(); !dissect.Ok(err) {
		return err
	}

	rel, err := filepath.Rel(root, file)
	if err != nil {
		rel = file
	}
	round := roundFromPath(file)
	players := inferPlayerTracks(r, tracks)
	playerEntities := make(map[byte]bool)
	for _, player := range players {
		playerEntities[player.EntityID] = true
	}

	nonPlayers := nonPlayerTracks(tracks, playerEntities, minSamples)
	rep.NonPlayerTracks += len(nonPlayers)
	rep.DestructionEvents += len(r.DroneDestructions)

	if len(rep.RoundSamples) < maxSamples {
		rep.RoundSamples = append(rep.RoundSamples, roundSample{
			File:              rel,
			Round:             round,
			MatchID:           r.Header.MatchID,
			Players:           len(r.Header.Players),
			PositionEntities:  len(tracks),
			NonPlayerEntities: len(nonPlayers),
			DroneDestructions: len(r.DroneDestructions),
		})
	}

	for _, track := range nonPlayers {
		last := lastSample(track)
		rep.EntityEndTimeCounts[fmt.Sprintf("%d", last.Time)]++
		nearestPlayer, nearestDistance := nearestPlayerAtTrackEnd(track, players, "")
		nearDestruction := nearDroneDestruction(track, r.DroneDestructions, destructionWindow)
		if len(rep.EntitySamples) < maxSamples {
			summary := entitySummary{
				File:                 rel,
				Round:                round,
				EntityID:             int(track.EntityID),
				Samples:              len(track.Samples),
				FirstTime:            track.Samples[0].Time,
				LastTime:             last.Time,
				FirstOffset:          track.Samples[0].Offset,
				LastOffset:           last.Offset,
				Distance:             track.Distance,
				NearDroneDestruction: nearDestruction,
			}
			if nearestPlayer != nil {
				summary.NearestEndPlayer = nearestPlayer.Username
				summary.NearestEndTeamRole = nearestPlayer.TeamRole
				summary.NearestEndDistance = &nearestDistance
			}
			rep.EntitySamples = append(rep.EntitySamples, summary)
		}

		if nearDestruction {
			matches := destructionMatchesForTrack(rel, round, track, players, r.DroneDestructions, destructionWindow)
			rep.DestructionMatches += len(matches)
			for _, match := range matches {
				rep.DestructionTimeCounts[fmt.Sprintf("%d", match.EntityLastTime)]++
				if len(rep.DestructionEvidence) < maxSamples {
					rep.DestructionEvidence = append(rep.DestructionEvidence, match)
				}
			}
			continue
		}

		attacker, attackerDistance := nearestPlayerAtTrackEnd(track, players, string(dissect.Attack))
		if attacker != nil && attackerDistance <= pickupDistance {
			rep.PickupCandidates++
			rep.PickupTimeCounts[fmt.Sprintf("%d", last.Time)]++
			rep.PickupEntityCounts[fmt.Sprintf("%d", track.EntityID)]++
			if len(rep.PickupEvidence) < maxSamples {
				rep.PickupEvidence = append(rep.PickupEvidence, pickupCandidate{
					File:             rel,
					Round:            round,
					EntityID:         int(track.EntityID),
					LastTime:         last.Time,
					LastOffset:       last.Offset,
					Samples:          len(track.Samples),
					Distance:         track.Distance,
					NearestAttacker:  attacker.Username,
					AttackerOperator: attacker.Operator,
					DistanceToPlayer: attackerDistance,
				})
			}
		}
	}

	ownerHintCount, ownerHintSamples := scanOwnerHints(rawBytes, rel, round, nonPlayers, players, maxSamples-len(rep.OwnerHintEvidence))
	rep.DirectOwnerHints += ownerHintCount
	for _, hint := range ownerHintSamples {
		if len(rep.OwnerHintEvidence) < maxSamples {
			rep.OwnerHintEvidence = append(rep.OwnerHintEvidence, hint)
		}
	}

	appendDroneInventory(rel, round, r, players, nonPlayers, destructionWindow, pickupDistance, rep)

	return nil
}

func appendDroneInventory(file string, round int, r *dissect.Reader, players []playerTrack, nonPlayers []*entityTrack, destructionWindow float64, pickupDistance float64, rep *report) {
	pickupsByUser := make(map[string][]pickupCandidate)
	for _, track := range nonPlayers {
		last := lastSample(track)
		if last.Time != 44 || nearDroneDestruction(track, r.DroneDestructions, destructionWindow) {
			continue
		}
		attacker, attackerDistance := nearestPlayerAtTrackEnd(track, players, string(dissect.Attack))
		if attacker == nil || attackerDistance > pickupDistance {
			continue
		}
		candidate := pickupCandidate{
			File:             file,
			Round:            round,
			EntityID:         int(track.EntityID),
			LastTime:         last.Time,
			LastOffset:       last.Offset,
			Samples:          len(track.Samples),
			Distance:         track.Distance,
			NearestAttacker:  attacker.Username,
			AttackerOperator: attacker.Operator,
			DistanceToPlayer: attackerDistance,
		}
		pickupsByUser[attacker.Username] = append(pickupsByUser[attacker.Username], candidate)
		rep.PrepPickupCandidates++
	}

	for _, player := range players {
		if player.TeamRole != string(dissect.Attack) {
			continue
		}
		pickups := pickupsByUser[player.Username]
		state := droneInventory{
			File:                          file,
			Round:                         round,
			MatchID:                       r.Header.MatchID,
			Username:                      player.Username,
			Operator:                      player.Operator,
			PrepPickupCandidateCount:      len(pickups),
			PocketDronesAfterPrepEstimate: 1,
			Inference:                     "one_pocket_drone_after_prep_no_pickup_candidate",
			Confidence:                    "medium",
		}
		if len(pickups) > 0 {
			state.PocketDronesAfterPrepEstimate = 2
			state.Inference = "two_pocket_drones_after_prep_pickup_candidate"
			state.Confidence = "medium"
			best := pickups[0].DistanceToPlayer
			for _, pickup := range pickups {
				state.PrepPickupEntityIDs = append(state.PrepPickupEntityIDs, pickup.EntityID)
				if pickup.DistanceToPlayer < best {
					best = pickup.DistanceToPlayer
				}
			}
			sort.Ints(state.PrepPickupEntityIDs)
			state.BestPickupDistance = &best
			if best <= 1.0 {
				state.Confidence = "high"
			}
		}
		rep.DroneInventory = append(rep.DroneInventory, state)
		rep.PlayerDroneStates++
	}
}

func extractPositionTracks(raw []byte) map[byte]*entityTrack {
	times := extractTimeMarks(raw)
	tracks := make(map[byte]*entityTrack)
	timeIndex := 0

	for searchAt := 0; searchAt < len(raw); {
		rel := bytes.Index(raw[searchAt:], positionPattern)
		if rel < 0 {
			break
		}
		pos := searchAt + rel
		searchAt = pos + 1
		fieldStart := pos + len(positionPattern)
		if fieldStart+14 > len(raw) {
			continue
		}

		for timeIndex+1 < len(times) && times[timeIndex+1].Offset < pos {
			timeIndex++
		}

		x := float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[fieldStart+2 : fieldStart+6])))
		y := float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[fieldStart+6 : fieldStart+10])))
		z := float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[fieldStart+10 : fieldStart+14])))
		if !validPosition(x, y, z) {
			continue
		}

		eventTime := uint32(0)
		if len(times) > 0 && times[timeIndex].Offset < pos {
			eventTime = times[timeIndex].Seconds
		}
		entityID := raw[fieldStart]
		track := tracks[entityID]
		if track == nil {
			track = &entityTrack{EntityID: entityID}
			tracks[entityID] = track
		}
		sample := positionSample{Offset: pos, Time: eventTime, X: x, Y: y, Z: z}
		if len(track.Samples) > 0 {
			prev := track.Samples[len(track.Samples)-1]
			track.Distance += distance(prev, sample)
		}
		track.Samples = append(track.Samples, sample)
	}

	return tracks
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

func validPosition(x float64, y float64, z float64) bool {
	if math.IsNaN(x) || math.IsNaN(y) || math.IsNaN(z) {
		return false
	}
	if math.IsInf(x, 0) || math.IsInf(y, 0) || math.IsInf(z, 0) {
		return false
	}
	if math.Abs(x) < 0.01 && math.Abs(y) < 0.01 {
		return false
	}
	return x >= -500 && x <= 500 && y >= -500 && y <= 500 && z >= -100 && z <= 100
}

func inferPlayerTracks(r *dissect.Reader, tracks map[byte]*entityTrack) []playerTrack {
	candidates := make([]*entityTrack, 0, len(tracks))
	for _, track := range tracks {
		if len(track.Samples) >= 10 {
			candidates = append(candidates, track)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].EntityID < candidates[j].EntityID
	})
	if len(candidates) > len(r.Header.Players) {
		candidates = candidates[:len(r.Header.Players)]
	}

	players := make([]playerTrack, 0, len(candidates))
	for i, track := range candidates {
		if i >= len(r.Header.Players) {
			break
		}
		player := r.Header.Players[i]
		teamRole := ""
		if player.TeamIndex >= 0 && player.TeamIndex < len(r.Header.Teams) {
			teamRole = string(r.Header.Teams[player.TeamIndex].Role)
		}
		players = append(players, playerTrack{
			Username:  player.Username,
			Operator:  player.Operator.String(),
			TeamRole:  teamRole,
			EntityID:  track.EntityID,
			DissectID: player.DissectID,
			Track:     track,
		})
	}
	return players
}

func nonPlayerTracks(tracks map[byte]*entityTrack, playerEntities map[byte]bool, minSamples int) []*entityTrack {
	out := make([]*entityTrack, 0)
	for _, track := range tracks {
		if playerEntities[track.EntityID] || len(track.Samples) < minSamples {
			continue
		}
		out = append(out, track)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Samples[len(out[i].Samples)-1].Offset == out[j].Samples[len(out[j].Samples)-1].Offset {
			return out[i].EntityID < out[j].EntityID
		}
		return out[i].Samples[len(out[i].Samples)-1].Offset < out[j].Samples[len(out[j].Samples)-1].Offset
	})
	return out
}

func nearDroneDestruction(track *entityTrack, destructions []dissect.DroneDestruction, window float64) bool {
	lastTime := float64(lastSample(track).Time)
	for _, destruction := range destructions {
		if math.Abs(float64(destruction.TimeInSeconds)-lastTime) <= window {
			return true
		}
	}
	return false
}

func destructionMatchesForTrack(file string, round int, track *entityTrack, players []playerTrack, destructions []dissect.DroneDestruction, window float64) []destructionMatch {
	matches := make([]destructionMatch, 0)
	last := lastSample(track)
	for _, destruction := range destructions {
		timeDelta := math.Abs(float64(destruction.TimeInSeconds) - float64(last.Time))
		if timeDelta > window {
			continue
		}
		var destroyerDistance *float64
		if destroyer := playerByUsername(players, destruction.DestroyedBy); destroyer != nil {
			playerPos, ok := positionNearTime(destroyer.Track, last.Time)
			if ok {
				d := distance(last, playerPos)
				destroyerDistance = &d
			}
		}
		matches = append(matches, destructionMatch{
			File:                file,
			Round:               round,
			DestroyedBy:         destruction.DestroyedBy,
			Time:                destruction.TimeInSeconds,
			EntityID:            int(track.EntityID),
			EntityLastTime:      last.Time,
			TimeDelta:           timeDelta,
			LastOffset:          last.Offset,
			Samples:             len(track.Samples),
			Distance:            track.Distance,
			DistanceToDestroyer: destroyerDistance,
		})
	}
	return matches
}

func scanOwnerHints(raw []byte, file string, round int, tracks []*entityTrack, players []playerTrack, maxSamples int) (int, []ownerHint) {
	count := 0
	hints := make([]ownerHint, 0)
	for _, track := range tracks {
		anchors := []struct {
			name   string
			sample positionSample
		}{
			{name: "first_position", sample: track.Samples[0]},
			{name: "last_position", sample: lastSample(track)},
		}
		for _, anchor := range anchors {
			start := anchor.sample.Offset - 128
			if start < 0 {
				start = 0
			}
			end := anchor.sample.Offset + 160
			if end > len(raw) {
				end = len(raw)
			}
			window := raw[start:end]
			for _, player := range players {
				if len(player.DissectID) == 0 {
					continue
				}
				idx := bytes.Index(window, player.DissectID)
				if idx < 0 {
					continue
				}
				count++
				if len(hints) >= maxSamples {
					continue
				}
				hints = append(hints, ownerHint{
					File:         file,
					Round:        round,
					EntityID:     int(track.EntityID),
					Username:     player.Username,
					Anchor:       anchor.name,
					AnchorTime:   anchor.sample.Time,
					AnchorOffset: anchor.sample.Offset,
					HintOffset:   start + idx,
					Pattern:      strings.ToUpper(fmt.Sprintf("%x", player.DissectID)),
				})
			}
		}
	}
	return count, hints
}

func nearestPlayerAtTrackEnd(track *entityTrack, players []playerTrack, requiredRole string) (*playerTrack, float64) {
	last := lastSample(track)
	var best *playerTrack
	bestDistance := math.MaxFloat64
	for i := range players {
		player := &players[i]
		if requiredRole != "" && player.TeamRole != requiredRole {
			continue
		}
		playerPos, ok := positionNearTime(player.Track, last.Time)
		if !ok {
			continue
		}
		d := distance(last, playerPos)
		if d < bestDistance {
			best = player
			bestDistance = d
		}
	}
	return best, bestDistance
}

func positionNearTime(track *entityTrack, target uint32) (positionSample, bool) {
	if track == nil || len(track.Samples) == 0 {
		return positionSample{}, false
	}
	best := track.Samples[0]
	bestDelta := absTimeDelta(best.Time, target)
	for _, sample := range track.Samples[1:] {
		delta := absTimeDelta(sample.Time, target)
		if delta < bestDelta {
			best = sample
			bestDelta = delta
		}
	}
	return best, true
}

func playerByUsername(players []playerTrack, username string) *playerTrack {
	for i := range players {
		if players[i].Username == username {
			return &players[i]
		}
	}
	return nil
}

func lastSample(track *entityTrack) positionSample {
	return track.Samples[len(track.Samples)-1]
}

func distance(a positionSample, b positionSample) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	dz := a.Z - b.Z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func absTimeDelta(a uint32, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
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
	fmt.Printf("non-player tracks: %d\n", rep.NonPlayerTracks)
	fmt.Printf("drone destruction events: %d\n", rep.DestructionEvents)
	fmt.Printf("destruction matches: %d\n", rep.DestructionMatches)
	fmt.Printf("pickup candidates: %d\n", rep.PickupCandidates)
	fmt.Printf("prep pickup candidates: %d\n", rep.PrepPickupCandidates)
	fmt.Printf("player drone states: %d\n", rep.PlayerDroneStates)
	fmt.Printf("direct owner hints: %d\n", rep.DirectOwnerHints)
	for i, match := range rep.DestructionEvidence {
		if i >= 5 {
			break
		}
		fmt.Printf("destroy match file=%s time=%.0f entity=%d destroyedBy=%s dt=%.1f\n", match.File, match.Time, match.EntityID, match.DestroyedBy, match.TimeDelta)
	}
	for i, candidate := range rep.PickupEvidence {
		if i >= 5 {
			break
		}
		fmt.Printf("pickup candidate file=%s time=%d entity=%d nearest=%s dist=%.2f\n", candidate.File, candidate.LastTime, candidate.EntityID, candidate.NearestAttacker, candidate.DistanceToPlayer)
	}
}
