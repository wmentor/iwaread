package base

import (
	"fmt"
	"io"
)

// ReadUvarint reads an unsigned varint from buf starting at pos.
// Returns the value and the new position.
func ReadUvarint(buf []byte, pos int) (uint64, int, error) {
	var x uint64
	var s uint
	for i := 0; ; i++ {
		if pos >= len(buf) {
			return 0, pos, fmt.Errorf("varint: unexpected end of input")
		}
		b := buf[pos]
		pos++
		if b < 0x80 {
			if i > 9 || (i == 9 && b > 1) {
				return 0, pos, fmt.Errorf("varint: overflow")
			}
			return x | uint64(b)<<s, pos, nil
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
}

// ReadUvarintReader reads an unsigned varint from an io.Reader byte by byte.
func ReadUvarintReader(r io.Reader) (uint64, error) {
	var x uint64
	var s uint
	buf := make([]byte, 1)
	for i := 0; i < 10; i++ {
		_, err := io.ReadFull(r, buf)
		if err != nil {
			return 0, err
		}
		b := buf[0]
		if b < 0x80 {
			return x | uint64(b)<<s, nil
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
	return 0, fmt.Errorf("varint: too long")
}
