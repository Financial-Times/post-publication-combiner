package main

import (
	"net/http"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/post-publication-combiner/v2/processor"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const (
	idPathVar = "id"
)

type requestHandler struct {
	requestProcessor processor.RequestProcessorI
	log              *logger.UPPLogger
}

func (handler *requestHandler) postMessage(writer http.ResponseWriter, request *http.Request) {
	uuid := mux.Vars(request)[idPathVar]
	transactionID := request.Header.Get("X-Request-Id")

	defer request.Body.Close()

	if !isValidUUID(uuid) {
		handler.log.WithTransactionID(transactionID).Errorf("Invalid UUID %s", uuid)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	err := handler.requestProcessor.ForceMessagePublish(uuid, transactionID)
	switch err {
	case nil:
		writer.WriteHeader(http.StatusOK)
	case processor.NotFoundError:
		writer.WriteHeader(http.StatusNotFound)
	case processor.InvalidContentTypeError:
		writer.WriteHeader(http.StatusUnprocessableEntity)
	default:
		writer.WriteHeader(http.StatusInternalServerError)
	}

}

func isValidUUID(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
}
