package base

import (
	"encoding/binary"
	"fmt"
)

// Wire types for protobuf fields.
const (
	WireVarint  = 0
	WireFixed64 = 1
	WireBytes   = 2
	WireFixed32 = 5
)

// RawField represents a single protobuf field.
type RawField struct {
	No    int
	Wire  int
	Value any // uint64 for varint, []byte for bytes/fixed64/fixed32
}

// RawMessage is a schema-less protobuf message.
type RawMessage struct {
	Fields []RawField
	source []byte
}

// Parse parses raw protobuf bytes into a RawMessage.
func ParseMessage(data []byte) (*RawMessage, error) {
	m := &RawMessage{source: data}
	pos := 0
	for pos < len(data) {
		key, newPos, err := ReadUvarint(data, pos)
		if err != nil {
			return nil, fmt.Errorf("protobuf: %w", err)
		}
		pos = newPos
		no := int(key >> 3)
		wire := int(key & 7)
		if no == 0 {
			return nil, fmt.Errorf("protobuf: field number 0 is invalid")
		}
		var field RawField
		field.No = no
		field.Wire = wire
		switch wire {
		case WireVarint:
			v, newPos, err := ReadUvarint(data, pos)
			if err != nil {
				return nil, fmt.Errorf("protobuf: %w", err)
			}
			pos = newPos
			field.Value = v
		case WireFixed64:
			if pos+8 > len(data) {
				return nil, fmt.Errorf("protobuf: truncated fixed64")
			}
			field.Value = append([]byte(nil), data[pos:pos+8]...)
			pos += 8
		case WireBytes:
			l, newPos, err := ReadUvarint(data, pos)
			if err != nil {
				return nil, fmt.Errorf("protobuf: %w", err)
			}
			pos = newPos
			dataLen := int(l)
			if pos+dataLen > len(data) {
				return nil, fmt.Errorf("protobuf: truncated bytes field")
			}
			field.Value = append([]byte(nil), data[pos:pos+dataLen]...)
			pos += dataLen
		case WireFixed32:
			if pos+4 > len(data) {
				return nil, fmt.Errorf("protobuf: truncated fixed32")
			}
			field.Value = append([]byte(nil), data[pos:pos+4]...)
			pos += 4
		default:
			return nil, fmt.Errorf("protobuf: unsupported wire type %d", wire)
		}
		m.Fields = append(m.Fields, field)
	}
	return m, nil
}

// lastField returns the last field with the given number.
func (m *RawMessage) lastField(no int) *RawField {
	for i := len(m.Fields) - 1; i >= 0; i-- {
		if m.Fields[i].No == no {
			return &m.Fields[i]
		}
	}
	return nil
}

// Has returns true if the message has a field with the given number.
func (m *RawMessage) Has(no int) bool {
	return m.lastField(no) != nil
}

// GetVarint returns the varint value of the last field with the given number.
func (m *RawMessage) GetVarint(no int) (uint64, bool) {
	f := m.lastField(no)
	if f == nil || f.Wire != WireVarint {
		return 0, false
	}
	return f.Value.(uint64), true //nolint:forcetypeassert // ok.
}

// GetUint returns the varint as uint (for convenience).
func (m *RawMessage) GetUint(no int) (int, bool) {
	v, ok := m.GetVarint(no)
	if !ok {
		return 0, false
	}
	return int(v), true
}

// GetBool returns the varint as bool.
func (m *RawMessage) GetBool(no int) (bool, bool) {
	v, ok := m.GetVarint(no)
	if !ok {
		return false, false
	}
	return v != 0, true
}

// GetBytes returns the bytes value of the last field with the given number.
func (m *RawMessage) GetBytes(no int) ([]byte, bool) {
	f := m.lastField(no)
	if f == nil || f.Wire != WireBytes {
		return nil, false
	}
	return f.Value.([]byte), true //nolint:forcetypeassert // ok.
}

// GetString returns the string value of the last field with the given number.
func (m *RawMessage) GetString(no int) (string, bool) {
	b, ok := m.GetBytes(no)
	if !ok {
		return "", false
	}
	return string(b), true
}

// GetStrings returns all string values for the given field number.
func (m *RawMessage) GetStrings(no int) []string {
	var out []string
	for _, f := range m.Fields {
		if f.No == no && f.Wire == WireBytes {
			out = append(out, string(f.Value.([]byte))) //nolint:forcetypeassert // ok.
		}
	}
	return out
}

// GetMessage parses and returns a submessage for the given field number.
func (m *RawMessage) GetMessage(no int) (*RawMessage, error) {
	b, ok := m.GetBytes(no)
	if !ok {
		return nil, nil //nolint:nilnil // ok.
	}
	return ParseMessage(b)
}

// GetMessages parses and returns all submessages for the given field number.
func (m *RawMessage) GetMessages(no int) ([]*RawMessage, error) {
	var out []*RawMessage
	for _, f := range m.Fields {
		if f.No == no && f.Wire == WireBytes {
			sub, err := ParseMessage(f.Value.([]byte)) //nolint:forcetypeassert // ok.
			if err != nil {
				return nil, err
			}
			out = append(out, sub)
		}
	}
	return out, nil
}

// GetFloat returns the float32 value of a fixed32 field.
func (m *RawMessage) GetFloat(no int) (float32, bool) {
	f := m.lastField(no)
	if f == nil || f.Wire != WireFixed32 {
		return 0, false
	}
	b := f.Value.([]byte) //nolint:forcetypeassert // ok.
	return float32(binary.LittleEndian.Uint32(b)), true
}

// GetDouble returns the float64 value of a fixed64 field.
func (m *RawMessage) GetDouble(no int) (float64, bool) {
	f := m.lastField(no)
	if f == nil || f.Wire != WireFixed64 {
		return 0, false
	}
	b := f.Value.([]byte) //nolint:forcetypeassert // ok.
	return float64(binary.LittleEndian.Uint64(b)), true
}

// GetPackedVarints returns all varints for a packed or unpacked repeated field.
func (m *RawMessage) GetPackedVarints(no int) ([]uint64, error) {
	var out []uint64
	for _, f := range m.Fields {
		if f.No != no {
			continue
		}
		if f.Wire == WireVarint { //nolint:staticcheck //later.
			out = append(out, f.Value.(uint64)) //nolint:forcetypeassert // ok.
		} else if f.Wire == WireBytes {
			b := f.Value.([]byte) //nolint:forcetypeassert // ok.
			pos := 0
			for pos < len(b) {
				v, newPos, err := ReadUvarint(b, pos)
				if err != nil {
					return nil, err
				}
				pos = newPos
				out = append(out, v)
			}
		}
	}
	return out, nil
}
