package schemas

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Plot struct {
	Id          bson.ObjectID `bson:"_id,omitempty"`
	Ctime       time.Time     `bson:"ctime"`
	PlotId      string        `bson:"plotId"`
	OwnerId     bson.ObjectID `bson:"ownerId"`
	Name        string        `bson:"name"`
	Description string        `bson:"description"`
	Link        string        `bson:"link"`
	LinkTitle   string        `bson:"linkTitle"`
	Verified    bool          `bson:"verified"`
	BuildData   []byte        `bson:"buildData"`
}
