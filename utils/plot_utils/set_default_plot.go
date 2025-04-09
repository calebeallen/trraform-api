package plotutils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"trraformapi/utils"
	"trraformapi/utils/schemas"
)

func SetDefaultPlot(ctx context.Context, plotId *PlotId, user *schemas.User) error {

	plotIdStr := plotId.ToString()

	// get default buildData
	buildDataBytes, err := os.ReadFile("static/default_cactus.dat")
	if err != nil {
		return fmt.Errorf("in SetDefaultPlotData:\n%w", err)
	}
	buildData, err := utils.BytesToUint16Arr(buildDataBytes)
	if err != nil {
		return fmt.Errorf("in SetDefaultPlotData:\n%w", err)
	}

	// create plot data
	plotData := PlotData{
		Owner:     user.Username,
		Verified:  user.Subscribed,
		BuildData: buildData,
	}
	plotDataBytes, err := plotData.Encode()
	if err != nil {
		return fmt.Errorf("in SetDefaultPlotData:\n%w", err)
	}

	// upload plot data
	err = utils.PutObjectR2("plots", plotIdStr+".dat", bytes.NewReader(plotDataBytes), "application/octet-stream", ctx)
	if err != nil {
		return fmt.Errorf("in SetDefaultPlotData:\n%w", err)
	}

	// get default image
	imageData, err := os.ReadFile("static/default_cactus_img.png")
	if err != nil {
		return fmt.Errorf("in SetDefaultPlotData:\n%w", err)
	}

	// upload default image
	err = utils.PutObjectR2("images", plotIdStr+".png", bytes.NewReader(imageData), "image/png", ctx)
	if err != nil {
		return fmt.Errorf("in SetDefaultPlotData:\n%w", err)
	}

	//TODO: flag chunk for update

	return nil

}
