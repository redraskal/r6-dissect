package dissect

import (
	"fmt"
	"math"
	"sort"
)

const (
	crosshairOnTargetDeg    = 5.0
	crosshairNearTargetDeg  = 10.0
	farDistanceThreshold    = 20.0
	angularSnapThresholdDeg = 60.0
	snapToTargetDeg         = 10.0
	trackingWindowSize      = 10
	maxCrosshairPassDetail  = 200
)

type DistanceBucket struct {
	MinDistance     float64 `json:"minDistance"`
	MaxDistance     float64 `json:"maxDistance"`
	CrosshairOnRate float64 `json:"crosshairOnRate"`
	Samples         int     `json:"samples"`
}

type SuspiciousEvent struct {
	Type        string  `json:"type"`
	Description string  `json:"description"`
	SampleIndex int     `json:"sampleIndex"`
	Severity    float64 `json:"severity"`
}

type CrosshairPassEvent struct {
	SampleIndex int     `json:"sampleIndex"`
	TargetUser  string  `json:"targetUser"`
	AngleDeg    float64 `json:"angleDeg"`
	Distance    float64 `json:"distance"`
}

type PlayerAnalysis struct {
	Username                 string               `json:"username"`
	Team                     string               `json:"team"`
	Samples                  int                  `json:"samples"`
	CrosshairOnEnemyRate     float64              `json:"crosshairOnEnemyRate"`
	CrosshairOnEnemyFarRate  float64              `json:"crosshairOnEnemyFarRate"`
	AvgMinAngleToEnemy       float64              `json:"avgMinAngleToEnemy"`
	CrosshairPassesPerSample float64              `json:"crosshairPassesPerSample"`
	DistanceBuckets          []DistanceBucket     `json:"distanceBuckets"`
	MaxAngularSnap           float64              `json:"maxAngularSnap"`
	AvgAngularVelocity       float64              `json:"avgAngularVelocity"`
	P99AngularVelocity       float64              `json:"p99AngularVelocity"`
	SnapCount                int                  `json:"snapCount"`
	SnapToTargetCount        int                  `json:"snapToTargetCount"`
	TrackingScore            float64              `json:"trackingScore"`
	AvgApproachCurvature     float64              `json:"avgApproachCurvature"`
	SuspiciousEvents         []SuspiciousEvent    `json:"suspiciousEvents,omitempty"`
	CrosshairPasses          []CrosshairPassEvent `json:"crosshairPasses,omitempty"`
}

type RoundAnalysis struct {
	Players []PlayerAnalysis `json:"players"`
}

type MatchAnalysis struct {
	Rounds  []RoundAnalysis      `json:"rounds"`
	Summary []PlayerMatchSummary `json:"summary"`
}

type PlayerMatchSummary struct {
	Username                string  `json:"username"`
	RoundsAnalyzed          int     `json:"roundsAnalyzed"`
	AvgCrosshairOnEnemyRate float64 `json:"avgCrosshairOnEnemyRate"`
	AvgCrosshairOnFarRate   float64 `json:"avgCrosshairOnEnemyFarRate"`
	AvgMinAngleToEnemy      float64 `json:"avgMinAngleToEnemy"`
	MaxAngularSnap          float64 `json:"maxAngularSnap"`
	TotalSnapToTargetCount  int     `json:"totalSnapToTargetCount"`
	AvgTrackingScore        float64 `json:"avgTrackingScore"`
	AvgApproachCurvature    float64 `json:"avgApproachCurvature"`
	TotalSuspiciousEvents   int     `json:"totalSuspiciousEvents"`
}

func degToRad(d float64) float64 {
	return d * math.Pi / 180.0
}

func radToDeg(r float64) float64 {
	return r * 180.0 / math.Pi
}

func lookDirection(yawDeg, pitchDeg float64) (dx, dy, dz float64) {
	yaw := degToRad(yawDeg)
	pitch := degToRad(pitchDeg)
	dx = math.Cos(pitch) * math.Cos(yaw)
	dy = math.Cos(pitch) * math.Sin(yaw)
	dz = math.Sin(pitch)
	return
}

func angleBetweenDeg(pos PlayerPosition, tx, ty, tz float64) float64 {
	toX := tx - float64(pos.X)
	toY := ty - float64(pos.Y)
	toZ := tz - float64(pos.Z)
	dist := math.Sqrt(toX*toX + toY*toY + toZ*toZ)
	if dist < 0.001 {
		return 0
	}
	toX /= dist
	toY /= dist
	toZ /= dist

	lx, ly, lz := lookDirection(float64(pos.Yaw), float64(pos.Pitch))
	dot := lx*toX + ly*toY + lz*toZ
	if dot > 1 {
		dot = 1
	}
	if dot < -1 {
		dot = -1
	}
	return radToDeg(math.Acos(dot))
}

func distanceBetween(a, b PlayerPosition) float64 {
	dx := float64(a.X - b.X)
	dy := float64(a.Y - b.Y)
	dz := float64(a.Z - b.Z)
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func angleDelta(a, b PlayerPosition) float64 {
	lx1, ly1, lz1 := lookDirection(float64(a.Yaw), float64(a.Pitch))
	lx2, ly2, lz2 := lookDirection(float64(b.Yaw), float64(b.Pitch))
	dot := lx1*lx2 + ly1*ly2 + lz1*lz2
	if dot > 1 {
		dot = 1
	}
	if dot < -1 {
		dot = -1
	}
	return radToDeg(math.Acos(dot))
}

func hasRotation(p PlayerPosition) bool {
	return p.Yaw != 0 || p.Pitch != 0
}

func getEnemyPositionAt(enemy EntityPositions, proportion float64) PlayerPosition {
	n := len(enemy.Positions)
	if n == 0 {
		return PlayerPosition{}
	}
	idx := int(math.Round(proportion * float64(n-1)))
	if idx >= n {
		idx = n - 1
	}
	if idx < 0 {
		idx = 0
	}
	return enemy.Positions[idx]
}

func AnalyzeRound(r *Reader) *RoundAnalysis {
	if r == nil || len(r.Movements) == 0 {
		return &RoundAnalysis{}
	}

	teamByEntity := make(map[byte]string)
	for _, ep := range r.Movements {
		teamByEntity[ep.EntityID] = ep.Team
	}

	analysis := &RoundAnalysis{
		Players: make([]PlayerAnalysis, 0, len(r.Movements)),
	}
	for _, entity := range r.Movements {
		pa := analyzePlayer(entity, r.Movements, teamByEntity)
		analysis.Players = append(analysis.Players, pa)
	}
	return analysis
}

func AnalyzeMatch(m *MatchReader) *MatchAnalysis {
	ma := &MatchAnalysis{
		Rounds: make([]RoundAnalysis, 0, m.NumRounds()),
	}

	type acc struct {
		rounds          int
		crosshairOn     float64
		crosshairOnFar  float64
		minAngle        float64
		maxSnap         float64
		snapToTarget    int
		tracking        float64
		curvature       float64
		suspiciousCount int
	}
	players := make(map[string]*acc)

	for i := 0; i < m.NumRounds(); i++ {
		r, err := m.RoundAt(i)
		if err != nil || r == nil {
			continue
		}
		ra := AnalyzeRound(r)
		ma.Rounds = append(ma.Rounds, *ra)

		for _, pa := range ra.Players {
			a, ok := players[pa.Username]
			if !ok {
				a = &acc{}
				players[pa.Username] = a
			}
			a.rounds++
			a.crosshairOn += pa.CrosshairOnEnemyRate
			a.crosshairOnFar += pa.CrosshairOnEnemyFarRate
			a.minAngle += pa.AvgMinAngleToEnemy
			if pa.MaxAngularSnap > a.maxSnap {
				a.maxSnap = pa.MaxAngularSnap
			}
			a.snapToTarget += pa.SnapToTargetCount
			a.tracking += pa.TrackingScore
			a.curvature += pa.AvgApproachCurvature
			a.suspiciousCount += len(pa.SuspiciousEvents)
		}
	}

	ma.Summary = make([]PlayerMatchSummary, 0, len(players))
	for user, a := range players {
		n := float64(a.rounds)
		ma.Summary = append(ma.Summary, PlayerMatchSummary{
			Username:                user,
			RoundsAnalyzed:          a.rounds,
			AvgCrosshairOnEnemyRate: a.crosshairOn / n,
			AvgCrosshairOnFarRate:   a.crosshairOnFar / n,
			AvgMinAngleToEnemy:      a.minAngle / n,
			MaxAngularSnap:          a.maxSnap,
			TotalSnapToTargetCount:  a.snapToTarget,
			AvgTrackingScore:        a.tracking / n,
			AvgApproachCurvature:    a.curvature / n,
			TotalSuspiciousEvents:   a.suspiciousCount,
		})
	}

	sort.Slice(ma.Summary, func(i, j int) bool {
		return ma.Summary[i].TotalSuspiciousEvents > ma.Summary[j].TotalSuspiciousEvents
	})

	return ma
}

func analyzePlayer(player EntityPositions, allEntities []EntityPositions, teamByEntity map[byte]string) PlayerAnalysis {
	pa := PlayerAnalysis{
		Username: player.Username,
		Team:     player.Team,
		Samples:  len(player.Positions),
	}

	if len(player.Positions) < 10 {
		return pa
	}

	enemies := make([]EntityPositions, 0)
	for _, ep := range allEntities {
		if teamByEntity[ep.EntityID] != player.Team && len(ep.Positions) > 0 {
			enemies = append(enemies, ep)
		}
	}
	if len(enemies) == 0 {
		return pa
	}

	positions := player.Positions
	totalSamples := len(positions)

	bucketEdges := []float64{0, 5, 10, 20, 40, 80, 500}
	bucketOnCount := make([]int, len(bucketEdges)-1)
	bucketTotal := make([]int, len(bucketEdges)-1)

	var (
		crosshairOnCount    int
		crosshairOnFarCount int
		validRotationCount  int
		validFarCount       int
		totalMinAngle       float64
		crosshairPassCount  int
		angularVelocities   []float64
		maxSnap             float64
		snapCount           int
		snapToTargetCount   int
		trackingErrors      []float64
	)

	for i, pos := range positions {
		if !hasRotation(pos) {
			continue
		}
		validRotationCount++
		proportion := float64(i) / float64(totalSamples-1)

		minAngle := 180.0
		minDist := math.MaxFloat64
		var closestUser string
		for _, enemy := range enemies {
			ep := getEnemyPositionAt(enemy, proportion)
			angle := angleBetweenDeg(pos, float64(ep.X), float64(ep.Y), float64(ep.Z))
			dist := distanceBetween(pos, ep)
			if angle < minAngle {
				minAngle = angle
				minDist = dist
				closestUser = enemy.Username
			}
			if angle < crosshairNearTargetDeg {
				crosshairPassCount++
				if len(pa.CrosshairPasses) < maxCrosshairPassDetail {
					pa.CrosshairPasses = append(pa.CrosshairPasses, CrosshairPassEvent{
						SampleIndex: i,
						TargetUser:  enemy.Username,
						AngleDeg:    math.Round(angle*100) / 100,
						Distance:    math.Round(dist*100) / 100,
					})
				}
			}
		}

		totalMinAngle += minAngle

		if minAngle < crosshairOnTargetDeg {
			crosshairOnCount++
		}

		if minDist > farDistanceThreshold {
			validFarCount++
			if minAngle < crosshairOnTargetDeg {
				crosshairOnFarCount++
			}
		}

		for b := 0; b < len(bucketEdges)-1; b++ {
			if minDist >= bucketEdges[b] && minDist < bucketEdges[b+1] {
				bucketTotal[b]++
				if minAngle < crosshairOnTargetDeg {
					bucketOnCount[b]++
				}
				break
			}
		}

		trackingErrors = append(trackingErrors, minAngle)

		if i > 0 && hasRotation(positions[i-1]) {
			delta := angleDelta(positions[i-1], pos)
			angularVelocities = append(angularVelocities, delta)
			if delta > maxSnap {
				maxSnap = delta
			}
			if delta > angularSnapThresholdDeg {
				snapCount++
				if minAngle < snapToTargetDeg {
					snapToTargetCount++
					severity := math.Min(1.0, (delta/180.0)*(1.0-minAngle/snapToTargetDeg))
					pa.SuspiciousEvents = append(pa.SuspiciousEvents, SuspiciousEvent{
						Type: "snap_to_target",
						Description: fmt.Sprintf(
							"%.1f° snap landing %.1f° from %s at distance %.1f",
							delta, minAngle, closestUser, minDist,
						),
						SampleIndex: i,
						Severity:    math.Round(severity*1000) / 1000,
					})
				}
			}
		}
	}

	if validRotationCount > 0 {
		pa.CrosshairOnEnemyRate = round4(float64(crosshairOnCount) / float64(validRotationCount))
		pa.AvgMinAngleToEnemy = round2(totalMinAngle / float64(validRotationCount))
		pa.CrosshairPassesPerSample = round4(float64(crosshairPassCount) / float64(validRotationCount))
	}
	if validFarCount > 0 {
		pa.CrosshairOnEnemyFarRate = round4(float64(crosshairOnFarCount) / float64(validFarCount))
	}

	pa.DistanceBuckets = make([]DistanceBucket, 0, len(bucketEdges)-1)
	for b := 0; b < len(bucketEdges)-1; b++ {
		rate := 0.0
		if bucketTotal[b] > 0 {
			rate = round4(float64(bucketOnCount[b]) / float64(bucketTotal[b]))
		}
		pa.DistanceBuckets = append(pa.DistanceBuckets, DistanceBucket{
			MinDistance:     bucketEdges[b],
			MaxDistance:     bucketEdges[b+1],
			CrosshairOnRate: rate,
			Samples:         bucketTotal[b],
		})
	}

	pa.MaxAngularSnap = round2(maxSnap)
	pa.SnapCount = snapCount
	pa.SnapToTargetCount = snapToTargetCount

	if len(angularVelocities) > 0 {
		sum := 0.0
		for _, v := range angularVelocities {
			sum += v
		}
		pa.AvgAngularVelocity = round2(sum / float64(len(angularVelocities)))

		sorted := make([]float64, len(angularVelocities))
		copy(sorted, angularVelocities)
		sort.Float64s(sorted)
		p99Idx := int(math.Floor(0.99 * float64(len(sorted)-1)))
		pa.P99AngularVelocity = round2(sorted[p99Idx])
	}

	if len(trackingErrors) > trackingWindowSize {
		pa.TrackingScore = round4(computeTrackingScore(trackingErrors))
	}

	if len(angularVelocities) > 5 {
		pa.AvgApproachCurvature = round4(computeApproachCurvature(angularVelocities, trackingErrors))
	}

	sort.Slice(pa.CrosshairPasses, func(i, j int) bool {
		return pa.CrosshairPasses[i].AngleDeg < pa.CrosshairPasses[j].AngleDeg
	})
	sort.Slice(pa.SuspiciousEvents, func(i, j int) bool {
		return pa.SuspiciousEvents[i].Severity > pa.SuspiciousEvents[j].Severity
	})

	return pa
}

func computeTrackingScore(trackingErrors []float64) float64 {
	if len(trackingErrors) < trackingWindowSize {
		return 0
	}

	consistentWindows := 0
	totalWindows := 0
	step := trackingWindowSize / 2
	if step < 1 {
		step = 1
	}

	for i := 0; i <= len(trackingErrors)-trackingWindowSize; i += step {
		window := trackingErrors[i : i+trackingWindowSize]
		totalWindows++

		allLow := true
		for _, e := range window {
			if e > crosshairNearTargetDeg {
				allLow = false
				break
			}
		}
		if allLow {
			consistentWindows++
		}
	}

	if totalWindows == 0 {
		return 0
	}
	return float64(consistentWindows) / float64(totalWindows)
}

func computeApproachCurvature(angVelocities []float64, trackingErrors []float64) float64 {
	approachCount := 0
	linearCount := 0

	minLen := len(trackingErrors)
	if len(angVelocities) < minLen {
		minLen = len(angVelocities)
	}

	for i := 2; i < minLen; i++ {
		if trackingErrors[i] < trackingErrors[i-1] && trackingErrors[i-1] < trackingErrors[i-2] {
			approachCount++
			if i-1 < len(angVelocities) && i-2 < len(angVelocities) {
				if angVelocities[i-1] >= angVelocities[i-2] {
					linearCount++
				}
			}
		}
	}

	if approachCount == 0 {
		return 0
	}
	return float64(linearCount) / float64(approachCount)
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
func round4(v float64) float64 { return math.Round(v*10000) / 10000 }
