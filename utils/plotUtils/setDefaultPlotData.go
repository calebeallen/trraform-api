package plotutils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"
	"trraformapi/utils"
	"trraformapi/utils/schemas"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func SetDefaultPlotData(ctx context.Context, plotId *PlotId, user *schemas.User) error {

	// get default buildData
	buildData, err := os.ReadFile("static/default_cactus")
	if err != nil {
		return fmt.Errorf("in SetDefaultPlotData:\n%w", err)
	}
	plotIdStr := plotId.ToString()

	// upsert plot data
	plot := schemas.Plot{
		Ctime:     time.Now().UTC(),
		PlotId:    plotIdStr,
		OwnerId:   user.Id,
		Verified:  user.Subscribed,
		BuildData: buildData,
	}
	_, err = utils.MongoDB.Collection("plots").UpdateOne(ctx, bson.M{"plotId": plotIdStr}, bson.M{"$set": &plot}, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("in SetDefaultPlotData:\n%w", err)
	}

	// set default image data
	imageData, err := os.ReadFile("static/default_img.png")
	if err != nil {
		return fmt.Errorf("in SetDefaultPlotData:\n%w", err)
	}
	err = utils.PutObjectR2("images", plotIdStr+".png", bytes.NewReader(imageData), "image/png", ctx)
	if err != nil {
		return fmt.Errorf("in SetDefaultPlotData:\n%w", err)
	}

	//TODO: flag chunk for update

	return nil

}
