package main

import (
	"errors"
	"net/http"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/post-publication-combiner/v2/processor"
	"github.com/dchest/uniuri"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const (
	idPathVar = "id"
)

type requestProcessor interface {
	ForcePublication(uuid string, tid string) error
}

type requestHandler struct {
	requestProcessor requestProcessor
	log              *logger.UPPLogger
}

func (h *requestHandler) publishMessage(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)[idPathVar]
	transactionID := r.Header.Get("X-Request-Id")

	log := h.log.
		WithTransactionID(transactionID).
		WithUUID(uuid)

	if !isValidUUID(uuid) {
		log.Error("Invalid UUID")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if transactionID == "" {
		transactionID = "tid_force_publish" + uniuri.NewLen(10) + "_post_publication_combiner"
		log = log.WithTransactionID(transactionID)
		log.Info("Transaction ID was not provided. Generated a new one")
	}

	err := h.requestProcessor.ForcePublication(uuid, transactionID)
	if err != nil {
		log.WithError(err).Error("Failed message publication")

		if errors.Is(err, processor.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if errors.Is(err, processor.ErrInvalidContentType) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Info("Message published successfully")
	w.WriteHeader(http.StatusOK)
}

func isValidUUID(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
}
