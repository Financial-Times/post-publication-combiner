package processor

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Financial-Times/post-publication-combiner/v2/httputils"
)

type contentRetriever interface {
	getContent(uuid string) (ContentModel, error)
}

type internalContentRetriever interface {
	getInternalContent(uuid string) (ContentModel, []Annotation, error)
}

type dataRetriever struct {
	address string
	client  httputils.Client
}

func (dr dataRetriever) getInternalContent(uuid string) (ContentModel, []Annotation, error) {
	b, status, err := httputils.ExecuteRequestForUUID(dr.address, uuid, dr.client)
	if status == http.StatusNotFound {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	var content map[string]interface{}
	if err = json.Unmarshal(b, &content); err != nil {
		return nil, nil, fmt.Errorf("error unmarshalling internal content: %w", err)
	}
	delete(content, "annotations")

	var annotations struct {
		Things []Thing `json:"annotations"`
	}
	if err = json.Unmarshal(b, &annotations); err != nil {
		return nil, nil, fmt.Errorf("error unmarshalling annotations for internal content: %w", err)
	}

	var ann []Annotation
	for _, t := range annotations.Things {
		ann = append(ann, Annotation{t})
	}

	return content, ann, nil
}

func (dr dataRetriever) getContent(uuid string) (ContentModel, error) {
	b, status, err := httputils.ExecuteRequestForUUID(dr.address, uuid, dr.client)
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var content map[string]interface{}
	if err = json.Unmarshal(b, &content); err != nil {
		return nil, fmt.Errorf("error unmarshalling content: %w", err)
	}

	return content, nil
}
