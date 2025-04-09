package plotutils

import (
	"context"
	"fmt"
	"trraformapi/utils"
)

func FlagPlotForUpdate(ctx context.Context, plotId *PlotId) error {

	chunkId := plotId.GetChunkId()
	plotIdStr := plotId.ToString()

	//flag chunk for update
	_, err := utils.RedisCli.SAdd(ctx, "needsupdate", chunkId).Result()
	if err != nil {
		return fmt.Errorf("in FlagPlotForUpdate:\n%w", err)
	}

	//flag plot for update
	_, err = utils.RedisCli.SAdd(ctx, fmt.Sprintf("updatechunk:%s", chunkId), plotIdStr).Result()
	if err != nil {
		return fmt.Errorf("in FlagPlotForUpdate:\n%w", err)
	}

	return nil

}
