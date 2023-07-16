package dissect

import (
	"encoding/binary"
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
	reader                 *io.Reader
	compressed             *zstd.Decoder
	b                      []byte
	offset                 int
	queries                [][]byte
	listeners              []func() error
	time                   float64 // in seconds
	timeRaw                string  // raw dissect format
	lastDefuserPlayerIndex int
	planted                bool
	readPartial            bool // reads up to the player info packets
	playersRead            int
	Header                 Header        `json:"header"`
	MatchFeedback          []MatchUpdate `json:"matchFeedback"`
}

// NewReader decompresses in using zstd and
// validates the dissect header.
func NewReader(in io.Reader) (r *Reader, err error) {
	compressed, err := zstd.NewReader(in)
	if err != nil {
		return
	}
	r = &Reader{
		compressed:  compressed,
		reader:      &in,
		readPartial: false,
	}
	b, err := io.ReadAll(r.compressed)
	if err != nil && !(len(b) > 0 && err == zstd.ErrMagicMismatch) {
		return
	}
	r.b = b
	log.Debug().Int("size", len(r.b)).Send()
	if err = r.readHeaderMagic(); err != nil {
		return
	}
	h, err := r.readHeader()
	r.Header = h
	if err != nil {
		return
	}
	log.Debug().Str("season", r.Header.GameVersion).Int("code", r.Header.CodeVersion).Send()
	r.listen([]byte{0x22, 0x07, 0x94, 0x9B, 0xDC}, r.readPlayer)
	r.listen([]byte{0x22, 0xA9, 0x26, 0x0B, 0xE4}, r.readAtkOpSwap)
	r.listen([]byte{0xAF, 0x98, 0x99, 0xCA}, r.readSpawn)
	if h.CodeVersion >= Y8S1 {
		r.listen([]byte{0x1F, 0x07, 0xEF, 0xC9}, r.readTime)
	} else {
		r.listen([]byte{0x1E, 0xF1, 0x11, 0xAB}, r.readY7Time)
	}
	r.listen([]byte{0x59, 0x34, 0xE5, 0x8B, 0x04}, r.readMatchFeedback)
	r.listen([]byte{0x22, 0xA9, 0xC8, 0x58, 0xD9}, r.readDefuserTimer)
	return
}

type match struct {
	offset        int
	listenerIndex int
}

func (r *Reader) worker(start int, end int, wg *sync.WaitGroup, matches chan<- match) {
	defer wg.Done()
	indexes := make([]int, len(r.queries))
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
	channel := make(chan match)
	start := r.offset
	end := len(r.b)
	if r.readPartial {
		end /= 3
	}
	blockSize := int(math.Floor(float64(end-start) / float64(numWorkers)))
	log.Debug().Int("workers", numWorkers).Int("blockSize", blockSize).Send()
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
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
		r.offset = entry.offset + 1
		if err = r.listeners[entry.listenerIndex](); err != nil {
			return
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

func (r *Reader) listen(query []byte, listener func() error) {
	r.queries = append(r.queries, query)
	r.listeners = append(r.listeners, listener)
}

func (r *Reader) seek(query []byte) error {
	start := r.offset
	i := 0
	for {
		b, err := r.read(1)
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
		if b[0] != query[i] {
			i = 0
			continue
		}
		i++
		if i == len(query) {
			return nil
		}
	}
}

func (r *Reader) skip(n int) error {
	r.offset += n
	if r.offset >= len(r.b) {
		return ErrInvalidLength
	}
	return nil
}

func (r *Reader) read(n int) ([]byte, error) {
	if err := r.skip(n); err != nil {
		return []byte{}, err
	}
	return r.b[r.offset-n : r.offset], nil
}

func (r *Reader) readInt() (int, error) {
	b, err := r.read(1)
	if err != nil {
		return -1, err
	}
	return int(b[0]), nil
}

func (r *Reader) readString() (string, error) {
	size, err := r.readInt()
	if err != nil {
		return "", err
	}
	b, err := r.read(size)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (r *Reader) readUint32() (uint32, error) {
	if err := r.skip(1); err != nil { // size- unnecessary since we already know the length
		return 0, err
	}
	b, err := r.read(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

func (r *Reader) readUint64() (uint64, error) {
	if err := r.skip(1); err != nil { // size- unnecessary since we already know the length
		return 0, err
	}
	b, err := r.read(8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b), nil
}
