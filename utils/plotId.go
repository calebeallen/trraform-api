package utils

import (
	"fmt"
	"strconv"
)

type PlotId struct {
	Id uint64
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

func (plotId *PlotId) Verify() bool {

	idCopy := plotId.Id
	localId := idCopy & 0xffffff

	if localId == 0 || localId > Depth0Count {
		return false
	}

	idCopy >>= 24

	for depth := 0; idCopy > 0 && depth < 2; depth++ {

		localId = idCopy & 0xfff
		if localId == 0 || localId > SubplotCount {
			return false
		}
		idCopy >>= 12

	}

	return idCopy == 0

}
