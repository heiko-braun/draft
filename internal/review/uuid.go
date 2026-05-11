package review

import (
	"crypto/rand"
	"fmt"
)

// newUUID generates a random UUID v4 string using crypto/rand.
func newUUID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generating UUID: %w", err)
	}
	// Set version (4) and variant (RFC 4122) bits.
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16]), nil
}
