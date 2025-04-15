package plotutils

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"trraformapi/utils"

	"github.com/go-playground/validator/v10"
	"github.com/rivo/uniseg"
)

type PlotData struct {
	Name        string `validate:"maxgraphemes=48"`
	Description string `validate:"maxgraphemes=128"`
	Link        string `validate:"maxgraphemes=256"`
	LinkTitle   string `validate:"maxgraphemes=64"`
	Verified    bool
	Owner       string
	BuildData   []uint16 `validate:"builddata"`
}

type plotDataJsonPart struct {
	Version     int    `json:"ver"`
	Name        string `json:"name"`
	Description string `json:"desc"`
	Link        string `json:"link"`
	LinkTitle   string `json:"linkTitle"`
	Owner       string `json:"owner"`
	Verified    bool   `json:"verified"`
}

func init() {

	utils.Validate.RegisterValidation("maxgraphemes", func(fl validator.FieldLevel) bool {

		param := fl.Param()
		maxLength, err := strconv.Atoi(param)
		if err != nil {
			return false
		}

		field := fl.Field().String()
		gr := uniseg.NewGraphemes(field)
		count := 0
		for gr.Next() {
			count++
			if count > maxLength {
				return false
			}
		}

		return true

	})

	utils.Validate.RegisterValidation("builddata", func(fl validator.FieldLevel) bool {

		data, ok := fl.Field().Interface().([]uint16)
		if !ok {
			return false
		}

		// must contain at least 2 values (version, build size)
		if len(data) < 2 {
			return false
		}

		_, bs := data[0], int(data[1])
		bs3 := bs * bs * bs

		// create array to track subplots
		var subplotsUsed [utils.SubplotCount]bool
		for i := range subplotsUsed {
			subplotsUsed[i] = false
		}

		data = data[2:]
		blkCnt := 0

		// check that each block is valid, count blocks
		for i := range data {

			write, val := data[i]&1, data[i]>>1

			if write == 1 {

				if val > utils.MaxColorIndex {
					return false
				}

				//if subplot
				if val > 0 && val <= utils.SubplotCount {

					// if subplot is already placed, it can't be placed again
					if subplotsUsed[val-1] {
						return false
					}

					// next value must not be a repeat type
					if i+1 < len(data) && data[i+1]&1 == 0 {
						return false
					}

					subplotsUsed[val-1] = true

				}

				blkCnt++

			} else {
				blkCnt += int(val)
			}

			// terminate if block count exceeds bs^3
			if blkCnt > bs3 {
				return false
			}

		}

		return true

	})

}

func Decode(data []byte) (*PlotData, error) {

	var parts [][]byte
	buf := data

	for len(buf) > 0 {

		//first 4 bytes give the length of the data
		if len(data) < 4 {
			return nil, fmt.Errorf("in splitPlotDataParts: invalid prefix")
		}

		// read length (little-endian)
		partLen := binary.LittleEndian.Uint32(buf[:4])
		buf = buf[4:]

		if uint32(len(buf)) < partLen {
			return nil, fmt.Errorf("in splitPlotDataParts: invalid part")
		}

		chunk := buf[:partLen]
		parts = append(parts, chunk)
		buf = buf[partLen:]

	}

	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid plot data")
	}

	var jsonData plotDataJsonPart
	err := json.Unmarshal(parts[0], &jsonData)
	if err != nil {
		return nil, fmt.Errorf("error decoding json part")
	}

	buildData, err := utils.BytesToUint16Arr(parts[1])
	if err != nil {
		return nil, fmt.Errorf("error decoding build data")
	}

	plotData := PlotData{
		Name:        jsonData.Name,
		Description: jsonData.Description,
		Link:        jsonData.Link,
		LinkTitle:   jsonData.LinkTitle,
		Verified:    jsonData.Verified,
		Owner:       jsonData.Owner,
		BuildData:   buildData,
	}

	return &plotData, nil

}

func (plotData *PlotData) Encode() ([]byte, error) {

	jsonData := plotDataJsonPart{
		Version:     0,
		Name:        plotData.Name,
		Description: plotData.Description,
		Link:        plotData.Link,
		LinkTitle:   plotData.LinkTitle,
		Owner:       plotData.Owner,
		Verified:    plotData.Verified,
	}

	// Convert the struct to JSON bytes
	json, err := json.Marshal(jsonData)
	if err != nil {
		return nil, fmt.Errorf("in plotData.Encode:\n%w", err)
	}

	buildData := utils.Uint16ArrToBytes(plotData.BuildData)
	jsonLen := len(json)
	buildLen := len(buildData)
	length := jsonLen + buildLen + 8

	data := make([]byte, length)

	// set json data
	binary.LittleEndian.PutUint32(data, uint32(jsonLen))
	copy(data[4:], json)

	binary.LittleEndian.PutUint32(data[4+jsonLen:], uint32(buildLen)) // length prefix
	copy(data[8+jsonLen:], buildData)

	// Return the JSON bytes
	return data, nil

}
