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
	GetCombinedModelForAnnotations(metadata Annotations) (CombinedModel, error)
	GetCombinedModel(uuid string) (CombinedModel, error)
}

type DataCombiner struct {
	ContentRetriever  contentRetrieverI
	MetadataRetriever metadataRetrieverI
}

type contentRetrieverI interface {
	getContent(uuid string) (ContentModel, error)
}

type metadataRetrieverI interface {
	getAnnotations(uuid string) ([]Annotation, error)
}

type dataRetriever struct {
	Address utils.ApiURL
	client  utils.Client
}

func NewDataCombiner(docStoreApiUrl utils.ApiURL, annApiUrl utils.ApiURL, c utils.Client) DataCombinerI {
	var cRetriever contentRetrieverI = dataRetriever{docStoreApiUrl, c}
	var mRetriever metadataRetrieverI = dataRetriever{annApiUrl, c}

	return DataCombiner{
		ContentRetriever:  cRetriever,
		MetadataRetriever: mRetriever,
	}
}

func (dc DataCombiner) GetCombinedModelForContent(content ContentModel) (CombinedModel, error) {

	if content.getUUID() == "" {
		return CombinedModel{}, errors.New("content has no UUID provided. Can't deduce annotations for it.")
	}

	ann, err := dc.MetadataRetriever.getAnnotations(content.getUUID())
	if err != nil {
		return CombinedModel{}, err
	}

	return CombinedModel{
		UUID:         content.getUUID(),
		Content:      content,
		Metadata:     ann,
		LastModified: content.getLastModified(),
	}, nil
}

func (dc DataCombiner) GetCombinedModelForAnnotations(metadata Annotations) (CombinedModel, error) {

	if metadata.UUID == "" {
		return CombinedModel{}, errors.New("annotations have no UUID referenced. Can't deduce content for it.")
	}

	return dc.GetCombinedModel(metadata.UUID)
}

func (dc DataCombiner) GetCombinedModel(uuid string) (CombinedModel, error) {
	type annResponse struct {
		ann []Annotation
		err error
	}
	type cResponse struct {
		c   map[string]interface{}
		err error
	}

	// Get content
	content, err := dc.ContentRetriever.getContent(uuid)
	if err != nil {
		return CombinedModel{}, err
	}

	// Get annotations
	annotations, err := dc.MetadataRetriever.getAnnotations(uuid)
	if err != nil {
		return CombinedModel{}, err
	}

	return CombinedModel{
		UUID:         uuid,
		Content:      content,
		Metadata:     annotations,
		LastModified: content.getLastModified(),
	}, nil
}

func (dr dataRetriever) getAnnotations(uuid string) ([]Annotation, error) {

	var ann []Annotation

	b, status, err := utils.ExecuteHTTPRequest(uuid, dr.Address, dr.client)

	if status == http.StatusNotFound {
		return ann, nil
	}

	if err != nil {
		return ann, err
	}

	var things []Thing
	if err := json.Unmarshal(b, &things); err != nil {
		return ann, fmt.Errorf("could not unmarshall annotations for content with uuid=%v, error=%v", uuid, err.Error())
	}
	for _, t := range things {
		ann = append(ann, Annotation{t})
	}

	return ann, nil
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
		return c, fmt.Errorf("could not unmarshall content with uuid=%v, error=%v", uuid, err.Error())
	}

	return c, nil
}
