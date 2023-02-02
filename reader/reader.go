package reader

import (
	"errors"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/redraskal/r6-dissect/types"
)

var ErrInvalidFile = errors.New("dissect: not a dissect file")
var ErrInvalidLength = errors.New("dissect: received an invalid length of bytes")
var ErrInvalidStringSep = errors.New("dissect: invalid string separator")

var strSep = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

type DissectReader struct {
	reader      *io.Reader
	compressed  *zstd.Decoder
	offset      int
	queries     [][]byte
	listeners   []func() error
	time        int              // in seconds
	timeRaw     string           // raw dissect format
	partialRead bool             // reads up to the player info packets
	Activities  []types.Activity `json:"activityFeed"`
	Header      types.Header     `json:"header"`
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
		partialRead: false,
	}
	if err = r.readHeaderMagic(); err != nil {
		return
	}
	if h, err := r.readHeader(); err == nil {
		r.Header = h
	}
	r.listen(playerIndicator, r.readPlayer)
	r.listen(timeIndicator, r.readTime)
	r.listen(activityIndicator, r.readActivity)
	return
}

// Read continues reading the replay past the header until the EOF.
func (r *DissectReader) Read() error {
	b := make([]byte, 1)
	indexes := make([]int, len(r.queries))
	for {
		_, err := r.compressed.Read(b)
		r.offset++
		if err != nil {
			return err
		}
		for i, query := range r.queries {
			if b[0] != query[indexes[i]] {
				indexes[i] = 0
				continue
			}
			indexes[i]++
			if indexes[i] == len(query) {
				indexes[i] = 0
				if err := r.listeners[i](); err != nil {
					return err
				}
			}
		}
		if r.partialRead && len(r.Header.Players) == 10 {
			return nil
		}
	}
}

// PartialRead continues reading the replay past the header until the full player list is read.
func (r *DissectReader) PartialRead() error {
	r.partialRead = true
	return r.Read()
}

func (r *DissectReader) listen(query []byte, listener func() error) {
	r.queries = append(r.queries, query)
	r.listeners = append(r.listeners, listener)
}

func (r *DissectReader) seek(query []byte) error {
	b := make([]byte, 1)
	i := 0
	for {
		_, err := r.compressed.Read(b)
		r.offset++
		if err != nil {
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
