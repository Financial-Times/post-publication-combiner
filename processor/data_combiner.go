package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Financial-Times/post-publication-combiner/utils"
	"net/http"
	"strings"
)

type DataCombinerI interface {
	GetCombinedModelForContent(content ContentModel, platformVersion string) (CombinedModel, error)
	GetCombinedModelForAnnotations(metadata Annotations, platformVersion string) (CombinedModel, error)
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
	getAnnotations(uuid string, platformVersion string) ([]Annotation, error)
}

type dataRetriever struct {
	Address utils.ApiURL
	client  utils.Client
}

func (dc DataCombiner) GetCombinedModelForContent(content ContentModel, platformVersion string) (CombinedModel, error) {

	if content.getUUID() == "" {
		return CombinedModel{}, errors.New("Content has no UUID provided. Can't deduce annotations for it.")
	}

	ann, err := dc.MetadataRetriever.getAnnotations(content.getUUID(), platformVersion)
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

func (dc DataCombiner) GetCombinedModelForAnnotations(metadata Annotations, platformVersion string) (CombinedModel, error) {

	if metadata.UUID == "" {
		return CombinedModel{}, errors.New("Annotations have no UUID referenced. Can't deduce content for it.")
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

	// Deduce the platform version
	platform := PlatformV1
	if content.getType() == contentTypeVideo {
		platform = PlatformVideo
	}
	for _, identifier := range content.getIdentifiers() {
		if strings.HasPrefix(identifier.Authority, videoAuthority) {
			platform = PlatformVideo
		}
	}

	// Get annotations for platformVersion
	annotations, err := dc.MetadataRetriever.getAnnotations(uuid, platform)
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

func (dr dataRetriever) getAnnotations(uuid string, platformVersion string) ([]Annotation, error) {

	var ann []Annotation

	if platformVersion != "" {
		dr.Address.Endpoint = strings.Replace(dr.Address.Endpoint, "{platformVersion}", platformVersion, -1)
	}

	b, status, err := utils.ExecuteHTTPRequest(uuid, dr.Address, dr.client)

	if status == http.StatusNotFound {
		return ann, nil
	}

	if err != nil {
		return ann, err
	}

	var things []Thing
	if err := json.Unmarshal(b, &things); err != nil {
		return ann, fmt.Errorf("Could not unmarshall annotations for content with uuid=%v, error=%v", uuid, err.Error())
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
		return c, fmt.Errorf("Could not unmarshall content with uuid=%v, error=%v", uuid, err.Error())
	}

	return c, nil
}
