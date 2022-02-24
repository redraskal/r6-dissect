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

type Container struct {
	reader     *io.Reader
	compressed *zstd.Decoder
	static     []byte
	Header     types.Header `json:"header"`
}

// NewReader decompresses r using zstd and
// validates the dissect header.
func NewReader(r io.Reader) (c Container, err error) {
	compressed, err := zstd.NewReader(r)
	if err != nil {
		return
	}
	c = Container{
		compressed: compressed,
		reader:     &r,
	}
	if err = readHeaderMagic(compressed); err != nil {
		return
	}
	if h, err := readHeader(compressed); err == nil {
		c.Header = h
	}
	return
}

// readHeaderMagic reads the header magic of the reader
// and validates the dissect format.
// If there is an error, it will be of type *ErrInvalidFile.
func readHeaderMagic(r io.Reader) error {
	// Checks for the dissect header.
	b, err := readBytes(7, r)
	if err != nil {
		return err
	}
	if string(b[:7]) != "dissect" {
		return ErrInvalidFile
	}
	// Skips to the end of the unknown dissect versioning scheme.
	// Probably will be replaced later when more info is uncovered.
	// We are skipping to the end of the second sequence of 7 0x00 bytes
	// where the string values are stored.
	b = make([]byte, 1)
	n := 0
	t := 0
	for t != 2 {
		len, err := r.Read(b)
		if err != nil {
			return err
		}
		if len != 1 {
			return ErrInvalidFile
		}
		if b[0] == 0x00 {
			if n != 6 {
				n++
			} else {
				n = 0
				t++
			}
		} else if n > 0 {
			n = 0
		}
	}
	return nil
}
