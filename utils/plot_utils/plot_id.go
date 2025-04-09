package plotutils

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"strconv"
	"trraformapi/utils"
)

type PlotId struct {
	Id uint64
}

var chunkMap map[uint64]uint32

func init() {

	chunkMapBytes, err := os.ReadFile("static/chunk_map.dat")
	if err != nil {
		log.Fatal(err)
	}

	chunkMap = make(map[uint64]uint32)

	for i := 0; i < len(chunkMapBytes); i += 8 {

		chunkId := binary.LittleEndian.Uint32(chunkMapBytes[i : i+4])
		plotId := uint64(i / 8)
		chunkMap[plotId] = chunkId

	}

}

func PlotIdFromHexString(hex string) (*PlotId, error) {

	id, err := strconv.ParseUint(hex, 16, 64)
	if err != nil {
		return nil, fmt.Errorf("in PlotIdFromHexString:\n%w", err)
	}

	plotId := PlotId{
		Id: id,
	}

	return &plotId, nil

}

func CreateSubplotId(plotId *PlotId, local uint64) *PlotId {

	depth := plotId.Depth()
	var subplotId PlotId

	if plotId.Id == 0 {
		subplotId.Id = local
	} else {
		subplotId.Id = plotId.Id | (local << (24 + 12*depth))
	}

	return &subplotId

}

func (plotId *PlotId) ToString() string {

	hexStr := fmt.Sprintf("%x", plotId.Id)
	if len(hexStr)%2 != 0 {
		hexStr = "0" + hexStr
	}

	return hexStr

}

func (plotId *PlotId) Depth() int {

	id, depth := plotId.Id>>24, 0

	for id > 0 {
		depth++
		id >>= 12
	}

	return depth

}

func (plotId *PlotId) Validate() bool {

	idCopy := plotId.Id
	localId := idCopy & 0xffffff

	if localId == 0 || localId > utils.Depth0Count {
		return false
	}

	idCopy >>= 24

	for depth := 0; idCopy > 0 && depth < 2; depth++ {

		localId = idCopy & 0xfff
		if localId == 0 || localId > utils.SubplotCount {
			return false
		}
		idCopy >>= 12

	}

	return idCopy == 0

}

func (plotId *PlotId) GetChunkId() string {

	depth := plotId.Depth()

	if depth == 0 {
		return fmt.Sprintf("0_%d", chunkMap[plotId.Id-1])
	}

	chunkId := plotId.Id / utils.ChunkSize
	return fmt.Sprintf("%s_%d", plotId.ToString(), chunkId)

}
