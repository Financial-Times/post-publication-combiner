package processor

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Financial-Times/post-publication-combiner/v2/utils"
)

type dataCombiner interface {
	GetCombinedModelForContent(content ContentModel) (CombinedModel, error)
	GetCombinedModelForAnnotations(metadata AnnotationsMessage) (CombinedModel, error)
	GetCombinedModel(uuid string) (CombinedModel, error)
}

type DataCombiner struct {
	contentRetriever           contentRetriever
	contentCollectionRetriever contentRetriever
	internalContentRetriever   internalContentRetriever
}

type contentRetriever interface {
	getContent(uuid string) (ContentModel, error)
}

type internalContentRetriever interface {
	getInternalContent(uuid string) (ContentModel, []Annotation, error)
}

func NewDataCombiner(docStoreAPIURL, internalContentAPIURL, contentCollectionRWURL string, client utils.Client) *DataCombiner {
	return &DataCombiner{
		contentRetriever: dataRetriever{
			address: docStoreAPIURL,
			client:  client,
		},
		contentCollectionRetriever: dataRetriever{
			address: contentCollectionRWURL,
			client:  client,
		},
		internalContentRetriever: dataRetriever{
			address: internalContentAPIURL,
			client:  client,
		},
	}
}

func (dc DataCombiner) GetCombinedModelForContent(content ContentModel) (CombinedModel, error) {
	uuid := content.getUUID()
	if uuid == "" {
		return CombinedModel{}, fmt.Errorf("content has no UUID provided, can't deduce annotations for it")
	}

	internalContent, ann, err := dc.internalContentRetriever.getInternalContent(uuid)
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
		return CombinedModel{}, fmt.Errorf("annotations have no UUID referenced")
	}

	return dc.GetCombinedModel(uuid)
}

func (dc DataCombiner) GetCombinedModel(uuid string) (CombinedModel, error) {
	content, err := dc.contentRetriever.getContent(uuid)
	if err != nil {
		return CombinedModel{}, err
	}

	var internalContent ContentModel
	var annotations []Annotation
	if content.getUUID() != "" {
		// Internal content is available only if content is available
		internalContent, annotations, err = dc.internalContentRetriever.getInternalContent(uuid)
		if err != nil {
			return CombinedModel{}, err
		}
	} else {
		// There is nothing in the document store
		// try to retrieve data from the content collection store
		content, err = dc.contentCollectionRetriever.getContent(uuid)
		if err != nil {
			return CombinedModel{}, err
		}

		// Content collection found but the RW-er does not include type in the content
		if content.getUUID() != "" {
			content["type"] = "ContentCollection"
		}
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
