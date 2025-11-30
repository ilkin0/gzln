package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func HashReader(r io.Reader) (string, error) {
	hasher := sha256.New()

	if _, err := io.Copy(hasher, r); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func CompareHash(expected, computed string) bool {
	return expected == computed
}
