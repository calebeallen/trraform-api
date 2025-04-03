package schemas

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Plot struct {
	Id          primitive.ObjectID `bson:"_id,omitempty"`
	Ctime       time.Time          `bson:"ctime"`
	PlotId      int64              `bson:"plotId"`
	OwnerId     primitive.ObjectID `bson:"ownerId"`
	Name        string             `bson:"name"`
	Description string             `bson:"description"`
	Link        string             `bson:"link"`
	LinkTitle   string             `bson:"linkTitle"`
	Verified    bool               `bson:"verified"`
	BuildData   []byte             `bson:"buildData"`
}
