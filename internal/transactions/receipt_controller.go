package transactions

import (
	"encoding/json"
	"net/http"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

const purchaseReceiptMediaType = "application/vnd.agentpay.receipt+json"

// receipt downloads one verified machine-readable purchase receipt.
func (controller *HTTPController) receipt(
	response http.ResponseWriter,
	request *http.Request,
) {
	response.Header().Set("Cache-Control", "no-store")
	transactionID, err := domain.ParseID(
		request.PathValue("transactionId"),
		domain.TransactionIDPrefix,
	)
	if err != nil {
		writeTransactionError(response, request, persistence.ErrNotFound)
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	var receipt PurchaseReceipt
	if principal.Kind == api.PrincipalSeller {
		receipt, err = controller.service.GetReceiptForSeller(
			request.Context(),
			transactionID,
			principal.Subject,
		)
	} else {
		receipt, err = controller.service.GetReceiptForBuyer(
			request.Context(),
			transactionID,
			principal.Subject,
		)
	}
	if err != nil {
		writeTransactionError(response, request, err)
		return
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		writeTransactionError(response, request, err)
		return
	}
	encoded = append(encoded, '\n')
	response.Header().Set("Content-Type", purchaseReceiptMediaType)
	response.Header().Set(
		"Content-Disposition",
		"attachment; filename=\"agentpay-"+transactionID.String()+"-receipt.json\"",
	)
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(encoded)
}
