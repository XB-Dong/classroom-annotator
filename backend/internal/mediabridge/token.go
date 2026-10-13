package mediabridge

import (
	"errors"
	"io"
	"os"
	"strings"
)

// ReadTokenFile loads a private bearer token without accepting links or large files.
func ReadTokenFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return "", errors.New("invalid Mac bridge token file")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", errors.New("Mac bridge token unavailable")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil || len(data) > 4096 {
		return "", errors.New("invalid Mac bridge token file")
	}
	token := strings.TrimSpace(string(data))
	if len(token) < 16 {
		return "", errors.New("invalid Mac bridge token")
	}
	for i := 0; i < len(token); i++ {
		if token[i] < '!' || token[i] > '~' {
			return "", errors.New("invalid Mac bridge token")
		}
	}
	return token, nil
}
