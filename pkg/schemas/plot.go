package schemas

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Plot struct {
	Id     bson.ObjectID `bson:"_id,omitempty"`
	PlotId string        `bson:"plotId"`
	Ctime  time.Time     `bson:"ctime"`
	Owner  bson.ObjectID `bson:"owner"`
	Votes  int           `bson:"votes"`
}
