package dissect

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"runtime"
	"sort"
	"sync"

	"github.com/klauspost/compress/zstd"
	"github.com/rs/zerolog/log"
)

var strSep = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

type Reader struct {
	b                        []byte
	offset                   int
	queries                  [][]byte
	listeners                [][]func(r *Reader) error
	time                     float64 // in seconds
	timeRaw                  string  // raw dissect format
	lastDefuserPlayerIndex   int
	planted                  bool
	defuserDisabling         bool
	lastDefuserTimer         float64
	readPartial              bool // reads up to the player info packets
	playersRead              int
	lastKillerFromScoreboard string
	pendingSBIDs             [][4]byte          // Y10S4+: sbIDs in header-entry order for deferred mapping
	readPlayerOrder          []int              // Y10S4+: player indices in readPlayer order
	scoreboardIDToPlayer     map[[4]byte]int    // Y10S4+: maps scoreboard entry ID to player index
	scoreboardInitialKills   map[[4]byte]uint32 // Y10S4+: cumulative kills at round start per sbID
	scoreboardFinalKills     map[int]uint32     // Y10S4+: latest cumulative kills seen per player index
	pendingDefuserPlantIdx   int                // Y10S4+: index into MatchFeedback for DefuserPlantComplete/DisableComplete with unknown player (-1 = none)
	pendingDefuserIsPlant    bool               // Y10S4+: true if pending event is plant (attacker), false if disable (defender)
	lastPlayerScores         map[int]uint32     // Y10S4+: last known score per player index (for detecting +100 plant/disable bonus)
	pendingRevives           []scoreReviveCandidate
	positionsByEntity        map[byte]*EntityPositions
	positionRawAfter         map[byte][][]byte   // raw bytes after XYZ per entity for quaternion calibration
	roomEntityToPlayer       map[byte]int        // maps room entity bytes to player indices for location tracking
	roomEntityOrder          []byte              // tracks entity bytes in first-appearance order (first 10 = players)
	Header                   Header              `json:"header"`
	MatchFeedback            []MatchUpdate       `json:"matchFeedback"`
	UtilityEvents            []UtilityEvent      `json:"utilityEvents,omitempty"`
	CameraDestructions       []CameraDestruction `json:"cameraDestructions,omitempty"`
	DroneDestructions        []DroneDestruction  `json:"droneDestructions,omitempty"`
	Reinforcements           []Reinforcement     `json:"reinforcements,omitempty"`
	Barricades               []BarricadePlace    `json:"barricades,omitempty"`
	GadgetDeployments        []GadgetDeployment  `json:"gadgetDeployments,omitempty"`
	Movements                []EntityPositions   `json:"movements,omitempty"`
	LocationEvents           []LocationEvent     `json:"locationEvents,omitempty"`
	Scoreboard               Scoreboard
}

// NewReader decompresses in using zstd and
// validates the dissect header.
func NewReader(in io.Reader) (r *Reader, err error) {
	br := bufio.NewReader(in)
	chunkedCompression, err := testFileCompression(br)
	if err != nil {
		return r, err
	}
	log.Debug().Bool("chunkedCompression (>=Y8S4)", chunkedCompression).Send()
	r = &Reader{
		readPartial:            false,
		lastDefuserPlayerIndex: -1,
		pendingDefuserPlantIdx: -1,
		scoreboardIDToPlayer:   make(map[[4]byte]int),
		scoreboardInitialKills: make(map[[4]byte]uint32),
		scoreboardFinalKills:   make(map[int]uint32),
		lastPlayerScores:       make(map[int]uint32),
		positionsByEntity:      make(map[byte]*EntityPositions),
		positionRawAfter:       make(map[byte][][]byte),
		roomEntityToPlayer:     make(map[byte]int),
		UtilityEvents:          make([]UtilityEvent, 0),
		CameraDestructions:     make([]CameraDestruction, 0),
		DroneDestructions:      make([]DroneDestruction, 0),
		Reinforcements:         make([]Reinforcement, 0),
		Barricades:             make([]BarricadePlace, 0),
		GadgetDeployments:      make([]GadgetDeployment, 0),
		Movements:              make([]EntityPositions, 0),
	}
	if chunkedCompression {
		if err = r.readChunkedData(br); err != nil {
			return r, err
		}
	} else {
		if err = r.readNonChunkedData(br); err != nil {
			return r, err
		}
	}
	log.Debug().Int("size", len(r.b)).Send()
	log.Debug().Str("season", r.Header.GameVersion).Int("code", r.Header.CodeVersion).Send()
	r.Listen([]byte{0x22, 0x07, 0x94, 0x9B, 0xDC}, readPlayer)
	r.Listen([]byte{0x22, 0xA9, 0x26, 0x0B, 0xE4}, readAtkOpSwap)
	r.Listen([]byte{0xAF, 0x98, 0x99, 0xCA}, readSpawn)
	if r.Header.CodeVersion >= Y8S1 {
		r.Listen([]byte{0x1F, 0x07, 0xEF, 0xC9}, readTime)
	} else {
		r.Listen([]byte{0x1E, 0xF1, 0x11, 0xAB}, readY7Time)
	}
	r.Listen([]byte{0x59, 0x34, 0xE5, 0x8B, 0x04}, readMatchFeedback)
	r.Listen([]byte{0x22, 0xA9, 0xC8, 0x58, 0xD9}, readDefuserTimer)
	r.Listen([]byte{0xEC, 0xDA, 0x4F, 0x80}, readScoreboardScore)
	r.Listen([]byte{0x4D, 0x73, 0x7F, 0x9E}, readScoreboardAssists)
	r.Listen([]byte{0x1C, 0xD2, 0xB1, 0x9D}, readScoreboardKills)
	r.Listen([]byte{0x60, 0x73, 0x85, 0xFE}, readPosition)
	return r, err
}

func (r *Reader) readChunkedData(genericReader io.Reader) error {
	log.Debug().Msg("reading data")
	temp, err := io.ReadAll(genericReader)
	if err != nil {
		return err
	}
	r.b = temp
	log.Debug().Msg("reading header magic")
	if err := r.readHeaderMagic(); err != nil {
		return err
	}
	log.Debug().Msg("reading header")
	h, err := r.readHeader()
	r.Header = h
	if err != nil {
		return err
	}
	log.Debug().Msg("decompressing data")
	zstdMagic := []byte{0x28, 0xB5, 0x2F, 0xFD}
	zstdReader, _ := zstd.NewReader(nil)
	memoryReader := bytes.NewReader(nil)
	patternIndex := 0
	sections := 0
	data := make([]byte, 0)
	for !errors.Is(err, io.EOF) {
		for patternIndex != 4 {
			b, scanErr := r.Bytes(1)
			if errors.Is(scanErr, io.EOF) {
				err = scanErr
				break
			}
			if scanErr != nil {
				return scanErr
			}
			if b[0] == zstdMagic[patternIndex] {
				patternIndex++
			} else {
				patternIndex = 0
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		sections++
		patternIndex = 0
		memoryReader.Reset(r.b[r.offset-4:])
		tempReader := countedReader{memoryReader, 0}
		if err = zstdReader.Reset(&tempReader); err != nil {
			return err
		}
		decompressed, err := io.ReadAll(zstdReader)
		if err != nil && !(len(decompressed) > 0 && errors.Is(err, zstd.ErrMagicMismatch)) {
			return err
		}
		for _, b := range decompressed {
			data = append(data, b)
		}
		r.offset += tempReader.n
	}
	r.b = data
	r.offset = 0
	log.Debug().Int("zstd_sections", sections).Send()
	return nil
}

func (r *Reader) readNonChunkedData(genericReader io.Reader) error {
	zstdReader, err := zstd.NewReader(genericReader)
	if err != nil {
		return err
	}
	decompressed, err := io.ReadAll(zstdReader)
	if err != nil && !(len(decompressed) > 0 && errors.Is(err, zstd.ErrMagicMismatch)) {
		return err
	}
	r.b = decompressed
	if err = r.readHeaderMagic(); err != nil {
		return err
	}
	h, err := r.readHeader()
	r.Header = h
	return err
}

type match struct {
	offset        int
	listenerIndex int
}

func (r *Reader) worker(start int, end int, wg *sync.WaitGroup, matches chan<- match) {
	defer wg.Done()
	indexes := make([]int, len(r.queries))
	log.Debug().Int("start", start).Int("end", end).Msg("worker")
	for i := start; i <= end; i++ {
		for j, query := range r.queries {
			if r.b[i] == query[indexes[j]] {
				indexes[j]++
				if indexes[j] == len(query) {
					indexes[j] = 0
					matches <- match{i, j}
				}
			} else {
				indexes[j] = 0
			}
		}
	}
}

// Read continues reading the replay past the header until the EOF.
func (r *Reader) Read() (err error) {
	numWorkers := 5
	var wg sync.WaitGroup
	channel := make(chan match, 300)
	start := r.offset
	end := len(r.b)
	if r.readPartial {
		end /= 3
	}
	blockSize := int(math.Floor(float64(end-start) / float64(numWorkers)))
	log.Debug().Int("workers", numWorkers).Int("blockSize", blockSize).Send()
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		blockStart := r.offset + (i * blockSize)
		blockEnd := blockStart + blockSize
		if i > 0 {
			blockStart += 1
		}
		if i == numWorkers-1 {
			blockEnd = end - 1
		}
		go r.worker(blockStart, blockEnd, &wg, channel)
	}
	go func() {
		wg.Wait()
		close(channel)
	}()
	matches := make([]match, 0)
	log.Debug().Msg("reading from channel")
	for match := range channel {
		matches = append(matches, match)
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].offset < matches[j].offset
	})
	log.Debug().Int("matches", len(matches)).Msg("calling listeners")
	for _, entry := range matches {
		for _, listener := range r.listeners[entry.listenerIndex] {
			r.offset = entry.offset + 1
			if err = listener(r); err != nil {
				return
			}
		}
	}
	if !r.readPartial {
		r.roundEnd()
		r.detectRevivesFromScore()
		r.detectBleedouts()
		r.finalizePositions()
	}
	r.b = nil
	return err
}

// ReadPartial continues reading the replay past the header until the full player list is read.
// This information does not include dynamic data, such as attack operator swaps.
// Use ReadPartial for faster, minimal reads.
func (r *Reader) ReadPartial() error {
	r.readPartial = true
	log.Debug().Msg("using partial read")
	err := r.Read()
	r.readPartial = false
	return err
}

// Listen registers a callback to be run during Read whenever
// the pattern is found.
func (r *Reader) Listen(pattern []byte, callback func(r *Reader) error) {
	var i int
	for i = 0; i < len(r.queries); i++ {
		if bytes.Equal(r.queries[i], pattern) {
			r.listeners[i] = append(r.listeners[i], callback)
			break
		}
	}
	r.queries = append(r.queries, pattern)
	r.listeners = append(r.listeners, []func(reader *Reader) error{callback})
}

// Seek skips through the replay until the pattern is found.
func (r *Reader) Seek(pattern []byte) error {
	start := r.offset
	i := 0
	for {
		b, err := r.Bytes(1)
		if err != nil {
			if Ok(err) {
				pc, _, _, ok := runtime.Caller(1)
				details := runtime.FuncForPC(pc)
				if ok && details != nil {
					log.Warn().Int("bytes", r.offset-start).Interface("func", details.Name()).Msg("large seek")
				} else {
					log.Warn().Int("bytes", r.offset-start).Msg("large seek")
				}
			}
			return err
		}
		if b[0] != pattern[i] {
			i = 0
			continue
		}
		i++
		if i == len(pattern) {
			return nil
		}
	}
}

// Skip increases the replay offset by n bytes.
func (r *Reader) Skip(n int) error {
	r.offset += n
	if r.offset >= len(r.b) {
		return io.EOF
	}
	return nil
}

func (r *Reader) Bytes(n int) ([]byte, error) {
	if err := r.Skip(n); err != nil {
		return []byte{}, err
	}
	return r.b[r.offset-n : r.offset], nil
}

func (r *Reader) PeekBack(n int) []byte {
	start := r.offset - n
	if start < 0 {
		start = 0
	}
	return r.b[start:r.offset]
}

func (r *Reader) Int() (int, error) {
	b, err := r.Bytes(1)
	if err != nil {
		return -1, err
	}
	return int(b[0]), nil
}

func (r *Reader) String() (string, error) {
	size, err := r.Int()
	if err != nil {
		return "", err
	}
	b, err := r.Bytes(size)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (r *Reader) Uint32() (uint32, error) {
	if err := r.Skip(1); err != nil {
		return 0, err
	}
	b, err := r.Bytes(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

func (r *Reader) Uint64() (uint64, error) {
	if err := r.Skip(1); err != nil { // size- unnecessary since we already know the length
		return 0, err
	}
	b, err := r.Bytes(8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b), nil
}

func (r *Reader) Write(w io.Writer) (n int, err error) {
	return w.Write(r.b)
}

func (r *Reader) Float32() (float32, error) {
	b, err := r.Bytes(4)
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(b)), nil
}

func readPosition(r *Reader) error {
	entityID, err := r.Bytes(1)
	if err != nil {
		return err
	}
	if err := r.Skip(1); err != nil {
		return err
	}
	x, err := r.Float32()
	if err != nil {
		return err
	}
	y, err := r.Float32()
	if err != nil {
		return err
	}
	z, err := r.Float32()
	if err != nil {
		return err
	}
	if math.IsNaN(float64(x)) || math.IsNaN(float64(y)) || math.IsNaN(float64(z)) {
		return nil
	}
	if math.IsInf(float64(x), 0) || math.IsInf(float64(y), 0) || math.IsInf(float64(z), 0) {
		return nil
	}
	if math.Abs(float64(x)) < 0.01 && math.Abs(float64(y)) < 0.01 {
		return nil
	}
	if x < -500 || x > 500 || y < -500 || y > 500 || z < -100 || z > 100 {
		return nil
	}
	eid := entityID[0]
	ep, ok := r.positionsByEntity[eid]
	if !ok {
		ep = &EntityPositions{EntityID: eid}
		r.positionsByEntity[eid] = ep
	}
	ep.Positions = append(ep.Positions, PlayerPosition{
		X: x,
		Y: y,
		Z: z,
	})
	// Read raw bytes after XYZ for quaternion rotation calibration (up to 100 bytes)
	rawAfter, err := r.Bytes(100)
	if err == nil {
		r.positionRawAfter[eid] = append(r.positionRawAfter[eid], rawAfter)
	}
	return nil
}

func (r *Reader) finalizePositions() {
	candidates := make([]*EntityPositions, 0)
	for _, ep := range r.positionsByEntity {
		if len(ep.Positions) < 10 {
			continue
		}
		candidates = append(candidates, ep)
	}

	// Player entities have the lowest entity IDs, matching header order
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].EntityID < candidates[j].EntityID
	})

	playerCount := len(r.Header.Players)
	if playerCount == 0 {
		playerCount = 10
	}
	if len(candidates) > playerCount {
		candidates = candidates[:playerCount]
	}

	for i, ep := range candidates {
		if i < 5 {
			ep.Team = string(TeamRole(r.Header.Teams[0].Role))
		} else {
			ep.Team = string(TeamRole(r.Header.Teams[1].Role))
		}
		if i < len(r.Header.Players) {
			ep.Username = r.Header.Players[i].Username
		}

		// Calibrate quaternion offsets for this entity and extract yaw/pitch
		quatOffsets := r.calibrateQuaternionOffsets(ep.EntityID)
		if len(quatOffsets) > 0 {
			r.extractRotations(ep, quatOffsets)
		}

		log.Debug().
			Uint8("entityID", ep.EntityID).
			Str("player", ep.Username).
			Str("team", ep.Team).
			Int("positions", len(ep.Positions)).
			Ints("quatOffsets", quatOffsets).
			Msg("entity_player_mapping")
		r.Movements = append(r.Movements, *ep)
	}

	// Free raw rotation data
	r.positionRawAfter = nil

	log.Debug().Int("entities", len(r.Movements)).Msg("position_tracking_complete")
}

// calibrateQuaternionOffsets finds all offsets after position XYZ where
// a unit quaternion (4 floats with magnitude ≈ 1.0) appears consistently.
// Returns offsets sorted by priority: view direction (high pitch variance) first,
// then body rotation offsets. This allows per-position fallback.
func (r *Reader) calibrateQuaternionOffsets(entityID byte) []int {
	rawSlices := r.positionRawAfter[entityID]
	if len(rawSlices) == 0 {
		return nil
	}

	type candidate struct {
		offset   int
		count    int
		pitchVar float64
	}
	threshold := len(rawSlices) / 10 // 10% minimum for entities with mixed packet formats
	if threshold < 3 {
		threshold = 3
	}

	var candidates []candidate

	for tryOff := 0; tryOff <= 84; tryOff++ {
		if tryOff+16 > 100 {
			break
		}
		quatCount := 0
		var pitchSum, pitchSqSum float64
		for _, raw := range rawSlices {
			if tryOff+16 > len(raw) {
				continue
			}
			q0 := math.Float32frombits(binary.LittleEndian.Uint32(raw[tryOff : tryOff+4]))
			q1 := math.Float32frombits(binary.LittleEndian.Uint32(raw[tryOff+4 : tryOff+8]))
			q2 := math.Float32frombits(binary.LittleEndian.Uint32(raw[tryOff+8 : tryOff+12]))
			q3 := math.Float32frombits(binary.LittleEndian.Uint32(raw[tryOff+12 : tryOff+16]))

			if math.IsNaN(float64(q0)) || math.IsInf(float64(q0), 0) ||
				math.IsNaN(float64(q1)) || math.IsInf(float64(q1), 0) ||
				math.IsNaN(float64(q2)) || math.IsInf(float64(q2), 0) ||
				math.IsNaN(float64(q3)) || math.IsInf(float64(q3), 0) {
				continue
			}

			mag := q0*q0 + q1*q1 + q2*q2 + q3*q3
			if mag > 0.95 && mag < 1.05 {
				quatCount++
				sinP := float64(2 * (q0*q2 - q3*q1))
				if sinP > 1 {
					sinP = 1
				} else if sinP < -1 {
					sinP = -1
				}
				pitch := math.Asin(sinP) * 180 / math.Pi
				pitchSum += pitch
				pitchSqSum += pitch * pitch
			}
		}
		if quatCount >= threshold {
			var pitchVar float64
			if quatCount > 1 {
				mean := pitchSum / float64(quatCount)
				pitchVar = pitchSqSum/float64(quatCount) - mean*mean
			}
			candidates = append(candidates, candidate{tryOff, quatCount, pitchVar})
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// Sort: high pitch variance first (view direction), then by count
	sort.Slice(candidates, func(i, j int) bool {
		ci, cj := candidates[i], candidates[j]
		if (ci.pitchVar > 1.0) != (cj.pitchVar > 1.0) {
			return ci.pitchVar > 1.0
		}
		return ci.count > cj.count
	})

	offsets := make([]int, len(candidates))
	for i, c := range candidates {
		offsets[i] = c.offset
	}
	return offsets
}

// extractRotations reads quaternion data from stored raw bytes and populates
// Yaw/Pitch fields on each PlayerPosition. Tries offsets in priority order
// (view direction first, then body rotation) and uses the first valid quaternion.
func (r *Reader) extractRotations(ep *EntityPositions, quatOffsets []int) {
	rawSlices := r.positionRawAfter[ep.EntityID]
	if len(rawSlices) != len(ep.Positions) {
		return
	}

	for i, raw := range rawSlices {
		for _, quatOffset := range quatOffsets {
			if quatOffset+16 > len(raw) {
				continue
			}
			q0 := math.Float32frombits(binary.LittleEndian.Uint32(raw[quatOffset : quatOffset+4]))
			q1 := math.Float32frombits(binary.LittleEndian.Uint32(raw[quatOffset+4 : quatOffset+8]))
			q2 := math.Float32frombits(binary.LittleEndian.Uint32(raw[quatOffset+8 : quatOffset+12]))
			q3 := math.Float32frombits(binary.LittleEndian.Uint32(raw[quatOffset+12 : quatOffset+16]))

			mag := q0*q0 + q1*q1 + q2*q2 + q3*q3
			if mag < 0.95 || mag > 1.05 {
				continue
			}

			// Quaternion to Euler: yaw (Z-axis rotation) and pitch (Y-axis rotation)
			yaw := float32(math.Atan2(float64(2*(q0*q3+q1*q2)), float64(1-2*(q2*q2+q3*q3))) * 180 / math.Pi)
			sinP := float64(2 * (q0*q2 - q3*q1))
			if sinP > 1 {
				sinP = 1
			} else if sinP < -1 {
				sinP = -1
			}
			pitch := float32(math.Asin(sinP) * 180 / math.Pi)

			if !math.IsNaN(float64(yaw)) && !math.IsNaN(float64(pitch)) {
				ep.Positions[i].Yaw = yaw
				ep.Positions[i].Pitch = pitch
				break // Use first valid quaternion (highest priority offset)
			}
		}
	}
}
