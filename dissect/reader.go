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
	Header                   Header        `json:"header"`
	MatchFeedback            []MatchUpdate `json:"matchFeedback"`
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
		readPartial: false,
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
	if err := r.Skip(1); err != nil { // size- unnecessary since we already know the length
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
