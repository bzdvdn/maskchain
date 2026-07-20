package mask

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// @sk-task 22-shield-mask-storage#T1.1: Implement UUIDv7 generator (AC-007)
//
// NewUUIDv7 creates a new UUIDv7.
func NewUUIDv7() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}

	ms := uint64(time.Now().UnixMilli())

	buf[0] = byte(ms >> 40)
	buf[1] = byte(ms >> 32)
	buf[2] = byte(ms >> 24)
	buf[3] = byte(ms >> 16)
	buf[4] = byte(ms >> 8)
	buf[5] = byte(ms)

	buf[6] = (buf[6] & 0x0f) | 0x70
	buf[8] = (buf[8] & 0x3f) | 0x80

	dst := make([]byte, 36)
	hex.Encode(dst[0:8], buf[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], buf[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], buf[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], buf[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], buf[10:16])
	return string(dst), nil
}

func NewShortID() (string, error) {
	var buf [8]byte
	ms := uint64(time.Now().UnixMilli())
	buf[0] = byte(ms >> 40)
	buf[1] = byte(ms >> 32)
	buf[2] = byte(ms >> 24)
	buf[3] = byte(ms >> 16)
	buf[4] = byte(ms >> 8)
	buf[5] = byte(ms)
	if _, err := rand.Read(buf[6:8]); err != nil {
		return "", err
	}
	return base62Encode(buf[:]), nil
}

func base62Encode(src []byte) string {
	n := uint64(0)
	for _, b := range src {
		n = (n << 8) | uint64(b)
	}
	if n == 0 {
		return string(base62Alphabet[0])
	}
	var dst [12]byte
	i := len(dst)
	for n > 0 {
		i--
		dst[i] = base62Alphabet[n%62]
		n /= 62
	}
	return string(dst[i:])
}
