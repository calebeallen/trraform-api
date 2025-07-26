package plotutils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
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

	// create plot data (don't set verified status here)
	plotData := PlotData{
		Owner:     user.Username,
		BuildData: buildData,
	}
	plotDataBytes, err := plotData.Encode()
	if err != nil {
		return fmt.Errorf("in SetDefaultPlotData:\n%w", err)
	}

	// set verified status in metadata so it can be changed easily
	metadata := map[string]string{"verified": strconv.FormatBool(user.Subscription.IsActive)}

	// upload plot data
	err = utils.PutObjectR2(ctx, "plots", plotIdStr+".dat", bytes.NewReader(plotDataBytes), "application/octet-stream", metadata)
	if err != nil {
		return fmt.Errorf("in SetDefaultPlotData:\n%w", err)
	}

	// get default image
	imageData, err := os.ReadFile("static/default_cactus_img.png")
	if err != nil {
		return fmt.Errorf("in SetDefaultPlotData:\n%w", err)
	}

	// upload default image
	err = utils.PutObjectR2(ctx, "images", plotIdStr+".png", bytes.NewReader(imageData), "image/png", nil)
	if err != nil {
		return fmt.Errorf("in SetDefaultPlotData:\n%w", err)
	}

	err = FlagPlotForUpdate(ctx, plotId)
	if err != nil {
		return fmt.Errorf("in SetDefaultPlotData:\n%w", err)
	}

	return nil

}
