package base

import (
	"fmt"
)

// snappyUncompressBlock decompresses a single Snappy block.
func snappyUncompressBlock(input []byte) ([]byte, error) {
	outLen, pos, err := ReadUvarint(input, 0)
	if err != nil {
		return nil, fmt.Errorf("snappy: %w", err)
	}
	out := make([]byte, outLen)
	ip := pos
	op := 0
	inLen := len(input)

	for ip < inLen {
		tag := input[ip]
		ip++
		kind := tag & 3
		if kind == 0 {
			// Literal run
			litLen := int(tag >> 2)
			if litLen >= 60 {
				extra := litLen - 59
				if ip+extra > inLen {
					return nil, fmt.Errorf("snappy: truncated literal length")
				}
				litLen = 0
				for i := range extra {
					litLen |= int(input[ip+i]) << (8 * i)
				}
				ip += extra
			}
			litLen++
			if ip+litLen > inLen {
				return nil, fmt.Errorf("snappy: truncated literal")
			}
			if op+litLen > int(outLen) {
				return nil, fmt.Errorf("snappy: output overflow (literal)")
			}
			copy(out[op:], input[ip:ip+litLen])
			ip += litLen
			op += litLen
			continue
		}

		var length int
		var offset int
		if kind == 1 {
			// Copy with 1-byte offset
			length = int((tag>>2)&0x7) + 4
			if ip >= inLen {
				return nil, fmt.Errorf("snappy: truncated copy-1")
			}
			offset = int(tag>>5)<<8 | int(input[ip])
			ip++
		} else if kind == 2 {
			// Copy with 2-byte offset
			length = int(tag>>2) + 1
			if ip+2 > inLen {
				return nil, fmt.Errorf("snappy: truncated copy-2")
			}
			offset = int(input[ip]) | int(input[ip+1])<<8
			ip += 2
		} else {
			// Copy with 4-byte offset
			length = int(tag>>2) + 1
			if ip+4 > inLen {
				return nil, fmt.Errorf("snappy: truncated copy-4")
			}
			offset = int(input[ip]) | int(input[ip+1])<<8 | int(input[ip+2])<<16 | int(input[ip+3])<<24
			ip += 4
		}

		if offset == 0 || offset > op {
			return nil, fmt.Errorf("snappy: invalid copy offset")
		}
		if op+length > int(outLen) {
			return nil, fmt.Errorf("snappy: output overflow (copy)")
		}
		from := op - offset
		for i := 0; i < length; i++ {
			out[op] = out[from]
			op++
			from++
		}
	}

	if op != int(outLen) {
		return nil, fmt.Errorf("snappy: expected %d bytes, produced %d", outLen, op)
	}
	return out, nil
}

// DecodeIwaData decodes IWA file bytes (Snappy chunks) into raw data.
func DecodeIwaData(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("iwa: empty data")
	}
	if data[0] != 0x00 {
		return nil, fmt.Errorf("iwa: unsupported chunk type 0x%x", data[0])
	}
	var chunks [][]byte
	pos := 0
	for pos < len(data) {
		if pos+4 > len(data) {
			return nil, fmt.Errorf("iwa: truncated chunk header")
		}
		chunkType := data[pos]
		if chunkType != 0x00 {
			return nil, fmt.Errorf("iwa: unsupported chunk type 0x%x at offset %d", chunkType, pos)
		}
		length := int(data[pos+1]) | int(data[pos+2])<<8 | int(data[pos+3])<<16
		pos += 4
		if pos+length > len(data) {
			return nil, fmt.Errorf("iwa: truncated chunk payload")
		}
		chunk, err := snappyUncompressBlock(data[pos : pos+length])
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
		pos += length
	}
	// Concatenate chunks
	totalLen := 0
	for _, c := range chunks {
		totalLen += len(c)
	}
	result := make([]byte, totalLen)
	offset := 0
	for _, c := range chunks {
		copy(result[offset:], c)
		offset += len(c)
	}
	return result, nil
}
