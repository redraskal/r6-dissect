package reader

import (
	"errors"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
)

var ErrInvalidFile = errors.New("dissect: not a dissect file")
var ErrInvalidLength = errors.New("dissect: received an invalid length of bytes")
var ErrInvalidStringSep = errors.New("dissect: invalid string separator")

var strSep = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

// Open opens the named compressed file for reading with the dissect format.
func Open(name string) (*io.Reader, error) {
	r, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	return NewReader(r)
}

// NewReader decompresses r using zstd and calls ReadHeader
// to validate the dissect format.
func NewReader(r io.Reader) (*io.Reader, error) {
	r, err := zstd.NewReader(r)
	if err != nil {
		return nil, err
	}
	return ReadHeaderMagic(r)
}

// ReadHeaderMagic reads the header magic of r
// and validates the dissect format.
// If there is an error, it will be of type *ErrInvalidFile.
func ReadHeaderMagic(r io.Reader) (*io.Reader, error) {
	// Checks for the dissect header.
	b, err := readBytes(7, r)
	if err != nil {
		return nil, err
	}
	if string(b[:7]) != "dissect" {
		return nil, ErrInvalidFile
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
			return nil, err
		}
		if len != 1 {
			return nil, ErrInvalidFile
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
	return &r, nil
}
