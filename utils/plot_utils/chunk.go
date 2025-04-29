package plotutils

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"trraformapi/utils"
)

func FlagPlotForUpdate(ctx context.Context, plotId *PlotId) error {

	chunkId := plotId.GetChunkId()
	plotIdStr := plotId.ToString()

	/* deprecated */

	//flag chunk for update
	err := utils.RedisCli.SAdd(ctx, "needsupdate", chunkId).Err()
	if err != nil {
		return fmt.Errorf("in FlagPlotForUpdate:\n%w", err)
	}

	//flag plot for update
	err = utils.RedisCli.SAdd(ctx, fmt.Sprintf("updatechunk:%s", chunkId), plotIdStr).Err()
	if err != nil {
		return fmt.Errorf("in FlagPlotForUpdate:\n%w", err)
	}

	/* new */
	err = utils.RedisCli.SAdd(ctx, "needs_update:plot_id", plotIdStr).Err()
	if err != nil {
		return fmt.Errorf("in FlagPlotForUpdate:\n%w", err)
	}

	return nil

}

func DecodeChunk(data []byte) (map[uint64][]byte, error) {

	result := make(map[uint64][]byte)
	cursor := 0

	for cursor < len(data) {

		// need at least 4 bytes for the body length and 8 bytes for the key
		if cursor+4+8 > len(data) {
			return nil, errors.New("not enough bytes to read length + key")
		}

		// Read body length (4 bytes)
		bodyLen := binary.LittleEndian.Uint32(data[cursor : cursor+4])
		cursor += 4

		// Read key (8 bytes)
		key := binary.LittleEndian.Uint64(data[cursor : cursor+8])
		cursor += 8

		// Check there's enough space for the body
		if cursor+int(bodyLen) > len(data) {
			return nil, errors.New("body length exceeds remaining data")
		}

		// Read the body
		body := data[cursor : cursor+int(bodyLen)]
		cursor += int(bodyLen)

		// Insert into the map
		result[key] = body
	}

	return result, nil

}

func EncodeChunk(m map[uint64][]byte) *bytes.Buffer {

	var buf bytes.Buffer

	for k, v := range m {
		// Write the 4-byte length of the body
		lengthBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(lengthBuf, uint32(len(v)))
		buf.Write(lengthBuf)

		// Write the 8-byte key
		keyBuf := make([]byte, 8)
		binary.LittleEndian.PutUint64(keyBuf, k)
		buf.Write(keyBuf)

		// Write the body bytes
		buf.Write(v)
	}

	return &buf

}
