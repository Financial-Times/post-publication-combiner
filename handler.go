package main

import (
	"errors"
	"github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/post-publication-combiner/processor"
	"github.com/gorilla/mux"
	"github.com/satori/go.uuid"
	"net/http"
)

const (
	idpathVar = "id"
)

type requestHandler struct {
	processor processor.Processor
}

func (handler *requestHandler) postMessage(writer http.ResponseWriter, request *http.Request) {
	uuid := mux.Vars(request)[idpathVar]
	transactionID := request.Header.Get("X-Request-Id")

	defer request.Body.Close()

	if !isValidUUID(uuid) {
		logger.NewEntry(transactionID).WithError(errors.New("Invalid UUID")).Error("Invalid UUID")
		writer.WriteHeader(http.StatusBadRequest)
		return

	}

	err := handler.processor.ForceMessagePublish(uuid, transactionID)
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
	_, err := uuid.FromString(id)
	return err == nil
}
