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
	ContentRetriever  contentRetrieverI
	MetadataRetriever metadataRetrieverI
}

type contentRetrieverI interface {
	getContent(uuid string) (ContentModel, error)
}

type metadataRetrieverI interface {
	getAnnotations(uuid string) ([]Annotation, error)
}

func NewDataCombiner(docStoreAPIUrl utils.ApiURL, annAPIUrl utils.ApiURL, c utils.Client) DataCombinerI {
	var cRetriever contentRetrieverI = dataRetriever{docStoreAPIUrl, c}
	var mRetriever metadataRetrieverI = dataRetriever{annAPIUrl, c}

	return DataCombiner{
		ContentRetriever:  cRetriever,
		MetadataRetriever: mRetriever,
	}
}

func (dc DataCombiner) GetCombinedModelForContent(content ContentModel) (CombinedModel, error) {
	uuid := content.getUUID()
	if uuid == "" {
		return CombinedModel{}, errors.New("content has no UUID provided, can't deduce annotations for it")
	}

	ann, err := dc.MetadataRetriever.getAnnotations(uuid)
	if err != nil {
		return CombinedModel{}, err
	}

	return CombinedModel{
		UUID:         uuid,
		Content:      content,
		Metadata:     ann,
		LastModified: content.getLastModified(),
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

type dataRetriever struct {
	Address utils.ApiURL
	client  utils.Client
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
		return ann, fmt.Errorf("could not unmarshal annotations for content with uuid=%v, error=%v", uuid, err.Error())
	}
	for _, t := range things {
		ann = append(ann, Annotation{t})
	}

	return ann, nil
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
