package dissect

import (
	"errors"
	"io"

	"github.com/klauspost/compress/zstd"
)

var ErrInvalidFile = errors.New("dissect: not a dissect file")
var ErrInvalidFolder = errors.New("dissect: not a match folder")
var ErrInvalidStringSep = errors.New("dissect: invalid string separator")

// Ok returns true if err only pertains to EOF (read was successful).
func Ok(err error) bool {
	// zstd.ErrMagicMismatch is expected at EOF because .rec files have extra non-compressed data.
	return err == nil || errors.Is(err, io.EOF) || errors.Is(err, zstd.ErrMagicMismatch)
}
