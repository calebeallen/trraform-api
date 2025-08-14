package plotutils

import (
	"context"
	"strconv"
	"trraformapi/pkg/config"
	"trraformapi/pkg/schemas"
	"trraformapi/pkg/utils"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/redis/go-redis/v9"
)

func SetDefaultPlot(redisCli *redis.Client, r2Cli *s3.Client, ctx context.Context, plotId *PlotId, user *schemas.User) error {

	metadata := map[string]string{
		"owner":    user.Username,
		"verified": strconv.FormatBool(user.Subscription.IsActive),
	}
	if err := utils.CopyObjectR2(r2Cli, ctx, config.CF_PLOT_BUCKET, "default.dat", plotId.ToString()+".dat", "application/octet-stream", metadata); err != nil {
		return err
	}

	if err := FlagPlotForUpdate(redisCli, ctx, plotId, false); err != nil {
		return err
	}

	return nil

}
