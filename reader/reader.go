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
	reader     *io.Reader
	compressed *zstd.Decoder
	static     []byte
	Header     types.Header `json:"header"`
}

// NewReader decompresses in using zstd and
// validates the dissect header.
func NewReader(in io.Reader) (r DissectReader, err error) {
	compressed, err := zstd.NewReader(in)
	if err != nil {
		return
	}
	r = DissectReader{
		compressed: compressed,
		reader:     &in,
	}
	if err = r.readHeaderMagic(); err != nil {
		return
	}
	if h, err := r.readHeader(); err == nil {
		r.Header = h
	}
	if err = r.readPlayers(); err != nil {
		return
	}
	return
}

func (r *DissectReader) Read(n int) ([]byte, error) {
	b := make([]byte, n)
	len, err := r.compressed.Read(b)
	if err != nil {
		return nil, err
	}
	if len != n {
		return nil, ErrInvalidLength
	}
	return b, nil
}

func (r *DissectReader) Seek(query []byte) error {
	b := make([]byte, 1)
	i := 0
	for {
		_, err := r.compressed.Read(b)
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

func (r *DissectReader) ReadInt() (int, error) {
	b, err := r.Read(1)
	if err != nil {
		return -1, err
	}
	return int(b[0]), nil
}

func (r *DissectReader) ReadString() (string, error) {
	size, err := r.ReadInt()
	if err != nil {
		return "", err
	}
	b, err := r.Read(size)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
