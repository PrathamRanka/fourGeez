package disputes

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/fourgeez/agentpay/internal/api"
	"github.com/fourgeez/agentpay/internal/domain"
	"github.com/fourgeez/agentpay/internal/persistence"
)

type RecordManualRefundBody struct {
	Amount    domain.Amount `json:"amount"`
	Asset     string        `json:"asset"`
	Network   string        `json:"network"`
	Reference string        `json:"reference"`
}

type ManualRemediationHTTPController struct {
	service          *ManualRemediationService
	idempotencyStore domain.IdempotencyStore
}

func NewManualRemediationHTTPController(service *ManualRemediationService, idempotencyStore domain.IdempotencyStore) *ManualRemediationHTTPController {
	return &ManualRemediationHTTPController{service: service, idempotencyStore: idempotencyStore}
}

func (controller *ManualRemediationHTTPController) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("POST /v1/sellers/{sellerId}/disputes/{disputeId}/refund-records", api.RequireSeller(http.HandlerFunc(controller.record)))
}

func (controller *ManualRemediationHTTPController) record(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	sellerID, err := domain.ParseID(request.PathValue("sellerId"), domain.SellerIDPrefix)
	if err != nil {
		writeRemediationError(response, request, persistence.ErrNotFound)
		return
	}
	disputeID, err := domain.ParseID(request.PathValue("disputeId"), domain.DisputeIDPrefix)
	if err != nil {
		writeRemediationError(response, request, persistence.ErrNotFound)
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, api.MaximumJSONBodyBytes)
	body, err := io.ReadAll(request.Body)
	if err != nil {
		writeRemediationError(response, request, domain.NewValidationError("body", "size", "invalid request body"))
		return
	}
	var input RecordManualRefundBody
	if err := api.DecodeJSONBytes(body, &input); err != nil {
		writeRemediationError(response, request, domain.NewValidationError("body", "json", "invalid request body"))
		return
	}
	principal, _ := api.PrincipalFromContext(request.Context())
	scope := principal.Subject + ":recordManualRefund:" + disputeID.String()
	decision, err := api.CheckIdempotency(request.Context(), controller.idempotencyStore, scope, request.Header.Get("Idempotency-Key"), body)
	if err != nil {
		writeRemediationError(response, request, err)
		return
	}
	if decision.Replay {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(decision.Status)
		_, _ = response.Write(decision.Body)
		return
	}
	record, err := controller.service.RecordManualRefund(request.Context(), RecordManualRefundRequest{
		DisputeID: disputeID, SellerID: sellerID, Amount: input.Amount, Asset: input.Asset,
		Network: input.Network, Reference: input.Reference, RecordedBy: principal.Subject,
	})
	if err != nil {
		writeRemediationError(response, request, err)
		return
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		api.WriteError(response, request, http.StatusInternalServerError, api.ErrorCodeInternal, "response encoding failed", nil)
		return
	}
	encoded = append(encoded, '\n')
	if err := api.SaveIdempotency(request.Context(), controller.idempotencyStore, scope, decision, http.StatusCreated, encoded, time.Now().UTC()); err != nil {
		api.WriteError(response, request, http.StatusInternalServerError, api.ErrorCodeInternal, "idempotency persistence failed", nil)
		return
	}
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusCreated)
	_, _ = response.Write(encoded)
}

func writeRemediationError(response http.ResponseWriter, request *http.Request, err error) {
	var validationErrors domain.ValidationErrors
	status, code := http.StatusInternalServerError, api.ErrorCodeInternal
	switch {
	case errors.As(err, &validationErrors):
		status, code = http.StatusBadRequest, api.ErrorCodeBadRequest
	case errors.Is(err, persistence.ErrNotFound), errors.Is(err, ErrRemediationAccess):
		status, code = http.StatusNotFound, api.ErrorCodeNotFound
	case errors.Is(err, ErrRefundNotAllowed):
		status, code = http.StatusUnprocessableEntity, api.ErrorCodeUnprocessable
	case errors.Is(err, ErrRemediationConflict), errors.Is(err, api.ErrIdempotencyConflict), errors.Is(err, persistence.ErrConditionFailed):
		status, code = http.StatusConflict, api.ErrorCodeConflict
	}
	api.WriteError(response, request, status, code, err.Error(), nil)
}
