package schemas

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Offense struct {
	Action   string     `bson:"action"`
	IssuedAt time.Time  `bson:"issuedAt"`
	EndsAt   *time.Time `bson:"endsAt"`
	Reason   string     `bson:"reason"`
}

type Subscription struct {
	ProductId      string `bson:"productId"`
	SubscriptionId string `bson:"subscriptionId"`
	IsActive       bool   `bson:"isActive"`
	IsCanceled     bool   `bson:"isCanceled"`
	RecurredCount  int    `bson:"recurredCount"`
}

type User struct {
	Id             bson.ObjectID `bson:"_id,omitempty"`
	Ctime          time.Time     `bson:"ctime"`
	Email          string        `bson:"email"`
	EmailVerified  bool          `bson:"emailVerified"`
	PassHash       string        `bson:"passHash"`
	GoogleId       string        `bson:"googleId"`
	Username       string        `bson:"username"`
	UnameChangedAt time.Time     `bson:"unameChangedAt"`
	StripeCustomer string        `bson:"stripeCustomer"`
	Subscription   Subscription  `bson:"subscription"`
	PlotCredits    int           `bson:"plotCredits"`
	PlotIds        []string      `bson:"plotIds"`
	PurchasedIds   []string      `bson:"purchasedIds"`
	Offenses       []Offense     `bson:"offenses"`
}
