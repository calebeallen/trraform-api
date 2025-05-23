package plotutils

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"strconv"
	"trraformapi/utils"

	"github.com/go-playground/validator/v10"
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

	utils.Validate.RegisterValidation("plotid", func(fl validator.FieldLevel) bool {

		plotId, err := PlotIdFromHexString(fl.Field().String())
		if err != nil {
			return false
		}

		return plotId.Validate()

	})

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

	return fmt.Sprintf("%x", plotId.Id)

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

func (plotId *PlotId) Split() []uint64 {

	locIds := []uint64{plotId.Id & 0xffffff}
	id := plotId.Id >> 24

	for id > 0 {
		locIds = append(locIds, id&0xfff)
		id >>= 12
	}

	return locIds

}

func (plotId *PlotId) GetParent() *PlotId {

	depth := plotId.Depth()
	if depth == 0 {
		return nil
	}
	mask := (1 << (24 + 12*(depth-1))) - 1
	parentId := plotId.Id & uint64(mask)
	return &PlotId{Id: parentId}

}

func (plotId *PlotId) GetChunkId() string {

	depth := plotId.Depth()

	if depth == 0 {
		return fmt.Sprintf("0_%x", chunkMap[plotId.Id-1])
	}

	parentId := plotId.GetParent()
	split := plotId.Split()
	chunkId := (split[len(split)-1] - 1) / utils.ChunkSize

	return fmt.Sprintf("%s_%x", parentId.ToString(), chunkId)

}
