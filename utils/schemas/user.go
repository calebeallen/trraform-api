package schemas

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	Id            primitive.ObjectID `bson:"_id,omitempty"`
	Ctime         time.Time          `bson:"ctime"`
	Username      string             `bson:"username"`
	Email         string             `bson:"email"`
	EmailVerified bool               `bson:"emailVerified"`
	PassHash      string             `bson:"pwdHash"`
	GoogleId      string             `bson:"googleId"`
	Subscribed    bool               `bson:"subscribed"`
	PlotCredits   int                `bson:"plotCredits"`
	PlotIds       []int64            `bson:"plotIds"`
	RsxEnd        time.Time          `bson:"rsxEnd"`
	Banned        bool               `bson:"banned"`
}
