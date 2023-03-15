package dissect

import (
	"encoding/binary"
	"io"
	"runtime"

	"github.com/klauspost/compress/zstd"
	"github.com/rs/zerolog/log"
)

var strSep = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

type DissectReader struct {
	reader                 *io.Reader
	compressed             *zstd.Decoder
	offset                 int
	queries                [][]byte
	listeners              []func() error
	time                   float64 // in seconds
	timeRaw                string  // raw dissect format
	lastDefuserPlayerIndex int
	planted                bool
	readPartial            bool // reads up to the player info packets
	playersRead            int
	MatchFeedback          []MatchUpdate `json:"matchFeedback"`
	Header                 Header        `json:"header"`
}

// NewReader decompresses in using zstd and
// validates the dissect header.
func NewReader(in io.Reader) (r *DissectReader, err error) {
	compressed, err := zstd.NewReader(in)
	if err != nil {
		return
	}
	r = &DissectReader{
		compressed:  compressed,
		reader:      &in,
		readPartial: false,
	}
	if err = r.readHeaderMagic(); err != nil {
		return
	}
	h, err := r.readHeader()
	r.Header = h
	if err != nil {
		return
	}
	r.listen([]byte{0x40, 0xF2, 0x15, 0x04}, r.readPlayer)
	r.listen([]byte{0x22, 0xA9, 0x26, 0x0B, 0xE4}, r.readAtkOpSwap)
	r.listen([]byte{0xAF, 0x98, 0x99, 0xCA}, r.readSpawn)
	if h.CodeVersion >= 7408213 { // Y8S1
		r.listen([]byte{0x1F, 0x07, 0xEF, 0xC9}, r.readTime)
	} else {
		r.listen([]byte{0x1E, 0xF1, 0x11, 0xAB}, r.readY7Time)
	}
	r.listen([]byte{0x59, 0x34, 0xE5, 0x8B, 0x04}, r.readMatchFeedback)
	r.listen([]byte{0x22, 0xA9, 0xC8, 0x58, 0xD9}, r.readDefuserTimer)
	return
}

// Read continues reading the replay past the header until the EOF.
func (r *DissectReader) Read() (err error) {
	b := make([]byte, 1)
	indexes := make([]int, len(r.queries))
	defer func() {
		if Ok(err) {
			err = r.deriveTeamRoles()
		}
	}()
	for {
		_, err = r.compressed.Read(b)
		r.offset++
		if err != nil {
			return
		}
		for i, query := range r.queries {
			if b[0] == query[indexes[i]] {
				indexes[i]++
				if indexes[i] == len(query) {
					indexes[i] = 0
					if err = r.listeners[i](); err != nil {
						return
					}
				}
			} else {
				indexes[i] = 0
			}
		}
		if r.readPartial && r.playersRead == 10 {
			return
		}
	}
}

// ReadPartial continues reading the replay past the header until the full player list is read.
func (r *DissectReader) ReadPartial() error {
	r.readPartial = true
	log.Debug().Msg("using partial read")
	err := r.Read()
	r.readPartial = false
	return err
}

func (r *DissectReader) listen(query []byte, listener func() error) {
	r.queries = append(r.queries, query)
	r.listeners = append(r.listeners, listener)
}

func (r *DissectReader) seek(query []byte) error {
	start := r.offset
	b := make([]byte, 1)
	i := 0
	for {
		_, err := r.compressed.Read(b)
		r.offset++
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

func (r *DissectReader) read(n int) ([]byte, error) {
	b := make([]byte, n)
	len, err := r.compressed.Read(b)
	r.offset += len
	if err != nil {
		return nil, err
	}
	if len != n {
		return nil, ErrInvalidLength
	}
	return b, nil
}

func (r *DissectReader) readInt() (int, error) {
	b, err := r.read(1)
	if err != nil {
		return -1, err
	}
	return int(b[0]), nil
}

func (r *DissectReader) readString() (string, error) {
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

func (r *DissectReader) readUint32() (uint32, error) {
	_, err := r.read(1) // size- unnecessary since we already know the length
	if err != nil {
		return 0, err
	}
	b, err := r.read(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

func (r *DissectReader) readUint64() (uint64, error) {
	_, err := r.read(1) // size- unnecessary since we already know the length
	if err != nil {
		return 0, err
	}
	b, err := r.read(8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b), nil
}
