package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

// WriteJSON writes one JSON response with the supplied HTTP status.
func WriteJSON(response http.ResponseWriter, status int, value any) error {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	return json.NewEncoder(response).Encode(value)
}

// WriteError writes the stable OpenAPI error envelope.
func WriteError(
	response http.ResponseWriter,
	request *http.Request,
	status int,
	code string,
	message string,
	details map[string]any,
) {
	requestID, _ := RequestIDFromContext(request.Context())
	_ = WriteJSON(response, status, ErrorResponse{
		Error: ErrorBody{
			Code:      code,
			Message:   message,
			RequestID: requestID,
			Details:   details,
		},
	})
}

// DecodeJSON enforces content type, body size, unknown fields, and one value.
func DecodeJSON(
	response http.ResponseWriter,
	request *http.Request,
	destination any,
) error {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return errors.New("content type must be application/json")
	}

	request.Body = http.MaxBytesReader(response, request.Body, MaximumJSONBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}

	var trailingValue any
	if err := decoder.Decode(&trailingValue); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain one JSON value")
		}
		return err
	}
	return nil
}
