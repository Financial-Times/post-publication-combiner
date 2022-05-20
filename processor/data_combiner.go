package processor

import (
	"fmt"

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
