package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Financial-Times/post-publication-combiner/v2/utils"
)

type DataCombinerI interface {
	GetCombinedModelForContent(content ContentModel) (CombinedModel, error)
	GetCombinedModelForAnnotations(metadata AnnotationsMessage) (CombinedModel, error)
	GetCombinedModel(uuid string) (CombinedModel, error)
}

type DataCombiner struct {
	ContentRetriever         contentRetrieverI
	InternalContentRetriever internalContentRetrieverI
}

type contentRetrieverI interface {
	getContent(uuid string) (ContentModel, error)
}

type internalContentRetrieverI interface {
	getInternalContent(uuid string) (ContentModel, []Annotation, error)
}

func NewDataCombiner(docStoreAPIUrl utils.ApiURL, internalContentAPIUrl utils.ApiURL, c utils.Client) DataCombinerI {
	var cRetriever contentRetrieverI = dataRetriever{docStoreAPIUrl, c}
	var icRetriever internalContentRetrieverI = dataRetriever{internalContentAPIUrl, c}

	return DataCombiner{
		ContentRetriever:         cRetriever,
		InternalContentRetriever: icRetriever,
	}
}

func (dc DataCombiner) GetCombinedModelForContent(content ContentModel) (CombinedModel, error) {
	uuid := content.getUUID()
	if uuid == "" {
		return CombinedModel{}, errors.New("content has no UUID provided, can't deduce annotations for it")
	}

	internalContent, ann, err := dc.InternalContentRetriever.getInternalContent(uuid)
	if err != nil {
		return CombinedModel{}, err
	}

	return CombinedModel{
		UUID:            uuid,
		Content:         content,
		InternalContent: internalContent,
		Metadata:        ann,
		LastModified:    content.getLastModified(),
	}, nil
}

func (dc DataCombiner) GetCombinedModelForAnnotations(metadata AnnotationsMessage) (CombinedModel, error) {

	uuid := metadata.getContentUUID()
	if uuid == "" {
		return CombinedModel{}, errors.New("annotations have no UUID referenced")
	}

	return dc.GetCombinedModel(uuid)
}

func (dc DataCombiner) GetCombinedModel(uuid string) (CombinedModel, error) {

	// Get content
	content, err := dc.ContentRetriever.getContent(uuid)
	if err != nil {
		return CombinedModel{}, err
	}

	// Get internal content
	internalContent, annotations, err := dc.InternalContentRetriever.getInternalContent(uuid)
	if err != nil {
		return CombinedModel{}, err
	}

	return CombinedModel{
		UUID:            uuid,
		Content:         content,
		InternalContent: internalContent,
		Metadata:        annotations,
		LastModified:    content.getLastModified(),
	}, nil
}

type dataRetriever struct {
	Address utils.ApiURL
	client  utils.Client
}

func (dr dataRetriever) getInternalContent(uuid string) (ContentModel, []Annotation, error) {
	var c map[string]interface{}
	var ann []Annotation

	b, status, err := utils.ExecuteHTTPRequest(uuid, dr.Address, dr.client)
	if status == http.StatusNotFound {
		return c, ann, nil
	}
	if err != nil {
		return c, ann, err
	}

	// Unmarshal as content
	if err = json.Unmarshal(b, &c); err != nil {
		return c, ann, fmt.Errorf("could not unmarshal internal content with uuid=%v, error=%v", uuid, err.Error())
	}
	delete(c, "annotations")

	// Unmarshal as annotations
	var annotations struct {
		Things []Thing `json:"annotations"`
	}
	if err := json.Unmarshal(b, &annotations); err != nil {
		return c, ann, fmt.Errorf("could not unmarshal annotations for internal content with uuid=%v, error=%v", uuid, err.Error())
	}
	for _, t := range annotations.Things {
		ann = append(ann, Annotation{t})
	}

	return c, ann, nil
}

func (dr dataRetriever) getContent(uuid string) (ContentModel, error) {
	var c map[string]interface{}
	b, status, err := utils.ExecuteHTTPRequest(uuid, dr.Address, dr.client)

	if status == http.StatusNotFound {
		return c, nil
	}

	if err != nil {
		return c, err
	}

	if err := json.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("could not unmarshal content with uuid=%v, error=%v", uuid, err.Error())
	}

	return c, nil
}
