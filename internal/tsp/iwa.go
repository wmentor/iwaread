package tsp

import (
	"fmt"

	"github.com/wmentor/iwaread/internal/base"
)

// IwaObject represents one object inside an IWA component.
type IwaObject struct {
	Identifier uint64
	Type       int
	Message    *base.RawMessage
	Payloads   [][]byte
}

// ParseIwaStream parses a decompressed IWA archive stream into objects.
func ParseIwaStream(raw []byte) ([]*IwaObject, error) {
	var objects []*IwaObject
	pos := 0
	for pos < len(raw) {
		infoLen, newPos, err := base.ReadUvarint(raw, pos)
		if err != nil {
			return nil, fmt.Errorf("iwa: %w", err)
		}
		pos = newPos
		if pos+int(infoLen) > len(raw) {
			return nil, fmt.Errorf("iwa: truncated ArchiveInfo")
		}
		archiveInfo, err := base.ParseMessage(raw[pos : pos+int(infoLen)])
		if err != nil {
			return nil, fmt.Errorf("iwa: %w", err)
		}
		pos += int(infoLen)

		// Extract identifier
		id, _ := archiveInfo.GetVarint(1) // ArchiveInfo.identifier = 1

		// Extract message infos
		msgInfos, err := archiveInfo.GetMessages(2) // ArchiveInfo.message_infos = 2
		if err != nil {
			return nil, fmt.Errorf("iwa: %w", err)
		}

		var payloads [][]byte
		var objType int
		for _, info := range msgInfos {
			length, _ := info.GetUint(3) // MessageInfo.length = 3
			if pos+length > len(raw) {
				return nil, fmt.Errorf("iwa: truncated payload")
			}
			payload := raw[pos : pos+length]
			payloads = append(payloads, payload)
			pos += length

			// Extract type from first MessageInfo
			if objType == 0 {
				t, _ := info.GetUint(1) // MessageInfo.type = 1
				objType = t
			}
		}

		// Parse first payload as message
		var msg *base.RawMessage
		if len(payloads) > 0 {
			msg, err = base.ParseMessage(payloads[0])
			if err != nil {
				return nil, fmt.Errorf("iwa: %w", err)
			}
		}

		objects = append(objects, &IwaObject{
			Identifier: id,
			Type:       objType,
			Message:    msg,
			Payloads:   payloads,
		})
	}
	return objects, nil
}

// ParseIwaFile decodes a .iwa file (snappy chunks) into its objects.
func ParseIwaFile(fileBytes []byte) ([]*IwaObject, error) {
	raw, err := base.DecodeIwaData(fileBytes)
	if err != nil {
		return nil, err
	}
	return ParseIwaStream(raw)
}
