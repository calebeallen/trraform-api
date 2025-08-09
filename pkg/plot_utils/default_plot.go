package plotutils

import (
	"bytes"
	"context"
	"log"
	"os"
	"strconv"
	"trraformapi/pkg/config"
	"trraformapi/pkg/schemas"
	"trraformapi/pkg/utils"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/redis/go-redis/v9"
)

var buildData []uint16

func init() {

	// get default buildData
	buildDataBytes, err := os.ReadFile("static/default_cactus.dat")
	if err != nil {
		log.Fatal(err)
	}
	buildData, err = utils.BytesToUint16Arr(buildDataBytes)
	if err != nil {
		log.Fatal(err)
	}

}

func SetDefaultPlot(redisCli *redis.Client, r2Cli *s3.Client, ctx context.Context, plotId *PlotId, user *schemas.User) error {

	plotIdStr := plotId.ToString()

	// create plot data (don't set verified status here)
	plotData := PlotData{
		Owner:     user.Username,
		BuildData: buildData,
	}
	plotDataBytes, err := plotData.Encode()
	if err != nil {
		return err
	}

	// upload plot data
	metadata := map[string]string{"verified": strconv.FormatBool(user.Subscription.IsActive)}
	if err := utils.PutObjectR2(r2Cli, ctx, config.CF_PLOT_BUCKET, plotIdStr+".dat", bytes.NewReader(plotDataBytes), "application/octet-stream", metadata); err != nil {
		return err
	}

	err = FlagPlotForUpdate(redisCli, ctx, plotId)
	if err != nil {
		return err
	}

	return nil

}
