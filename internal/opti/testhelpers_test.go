package opti

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

var (
	gzipNew   = func(w *os.File) *gzip.Writer { return gzip.NewWriter(w) }
	tarNew    = func(w io.Writer) *tar.Writer { return tar.NewWriter(w) }
	ioCopy    = io.Copy
	sha256Sum = sha256.Sum256
	hexEncode = hex.EncodeToString
)
