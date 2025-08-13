package payment

import (
	"net/http"
	"trraformapi/internal/api"
	"trraformapi/pkg/schemas"

	"github.com/stripe/stripe-go/v82"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (h *Handler) CreatePortalSession(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	ctx := r.Context()
	uid := ctx.Value("uid").(bson.ObjectID)
	resParams := &api.ResParams{W: w, R: r}

	var user schemas.User
	if err := h.MongoDB.Collection("users").FindOne(ctx, bson.M{
		"_id": uid,
	}).Decode(&user); err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	// must have customer id
	if user.StripeCustomer == "" {
		resParams.ResData = &struct {
			NoStripeCustomer bool `json:"noStripeCustomer"`
		}{NoStripeCustomer: true}
	}

	params := &stripe.BillingPortalSessionCreateParams{
		Customer:  stripe.String(user.StripeCustomer),
		ReturnURL: stripe.String("https://yourapp.com/account/billing"),
	}

	portalSession, err := h.StripeCli.V1BillingPortalSessions.Create(ctx, params)
	if err != nil {
		resParams.Code = http.StatusInternalServerError
		resParams.Err = err
		h.Res(resParams)
		return
	}

	resParams.ResData = &struct {
		StripeSession    string `json:"stripeSession"`
		StripeSessionUrl string `json:"stripeSessionUrl"`
	}{
		StripeSession:    portalSession.ID,
		StripeSessionUrl: portalSession.URL,
	}
	resParams.Code = http.StatusOK
	h.Res(resParams)

}
