package payment

import (
	"fmt"
	"net/http"
	"trraformapi/utils"

	"github.com/stripe/stripe-go/v82/paymentintent"
)

// stripe doesn't allow front end query of metadata
func GetPaymentIntentDetails(w http.ResponseWriter, r *http.Request) {

	// validate request
	_, err := utils.ValidateAuthToken(r)
	if err != nil {
		utils.MakeAPIResponse(w, r, http.StatusForbidden, nil, "Invalid token", true)
		return
	}

	intentId := r.URL.Query().Get("intent-id")

	var responseData struct {
		Type    string   `json:"type"`
		PlotIds []string `json:"plotIds"`
	}

	// get payment intent
	paymentIntent, err := paymentintent.Get(intentId, nil)
	if err != nil {
		utils.MakeAPIResponse(w, r, http.StatusBadRequest, nil, "Invalid payment intent", true)
		return
	}

	intentType, ok := paymentIntent.Metadata["type"]
	if !ok {
		utils.LogErrorDiscord("GetPaymentIntentDetails", fmt.Errorf("metadata missing type"), nil)
		utils.MakeAPIResponse(w, r, http.StatusInternalServerError, nil, "Internal server error", true)
		return
	}
	responseData.Type = intentType

	// get list of plotIds
	if intentType == "pay" {

		var plotIds []string

		for i := 0; ; i++ {
			key := fmt.Sprintf("i%d", i)
			if val, ok := paymentIntent.Metadata[key]; ok {
				plotIds = append(plotIds, val)
			} else {
				break

			}
		}

		responseData.PlotIds = plotIds

	} else {
		responseData.PlotIds = []string{}
	}

	utils.MakeAPIResponse(w, r, http.StatusOK, &responseData, "Success", false)

}
