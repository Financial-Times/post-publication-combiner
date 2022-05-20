package processor

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Financial-Times/post-publication-combiner/v2/utils"
)

type contentRetriever interface {
	getContent(uuid string) (ContentModel, error)
}

type internalContentRetriever interface {
	getInternalContent(uuid string) (ContentModel, []Annotation, error)
}

type dataRetriever struct {
	address string
	client  utils.Client
}

func (dr dataRetriever) getInternalContent(uuid string) (ContentModel, []Annotation, error) {
	var c map[string]interface{}
	var ann []Annotation

	b, status, err := utils.ExecuteHTTPRequest(uuid, dr.address, dr.client)
	if status == http.StatusNotFound {
		return c, ann, nil
	}
	if err != nil {
		return c, ann, err
	}

	// Unmarshal as content
	if err = json.Unmarshal(b, &c); err != nil {
		return c, ann, fmt.Errorf("error unmarshalling internal content: %w", err)
	}
	delete(c, "annotations")

	// Unmarshal as annotations
	var annotations struct {
		Things []Thing `json:"annotations"`
	}
	if err = json.Unmarshal(b, &annotations); err != nil {
		return c, ann, fmt.Errorf("error unmarshalling annotations for internal content: %w", err)
	}
	for _, t := range annotations.Things {
		ann = append(ann, Annotation{t})
	}

	return c, ann, nil
}

func (dr dataRetriever) getContent(uuid string) (ContentModel, error) {
	var c map[string]interface{}
	b, status, err := utils.ExecuteHTTPRequest(uuid, dr.address, dr.client)

	if status == http.StatusNotFound {
		return c, nil
	}

	if err != nil {
		return c, err
	}

	if err = json.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("error unmarshalling content: %w", err)
	}

	return c, nil
}
