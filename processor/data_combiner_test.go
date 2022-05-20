package processor

import (
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/Financial-Times/post-publication-combiner/v2/httputils"
	"github.com/stretchr/testify/assert"
)

func TestGetCombinedModelForContent(t *testing.T) {
	tests := []struct {
		name                 string
		contentModel         ContentModel
		internalContentModel ContentModel
		retrievedAnn         []Annotation
		retrievedErr         error
		expModel             CombinedModel
		expError             string
	}{
		{
			name:                 "no content UUID",
			contentModel:         ContentModel{},
			internalContentModel: ContentModel{},
			retrievedAnn:         []Annotation{},
			retrievedErr:         nil,
			expModel:             CombinedModel{},
			expError:             "content has no UUID provided, can't deduce annotations for it",
		},
		{
			name: "retrieving error",
			contentModel: ContentModel{
				"uuid": "some uuid",
			},
			internalContentModel: ContentModel{"uuid": "some uuid"},
			retrievedAnn:         []Annotation{},
			retrievedErr:         fmt.Errorf("some error"),
			expModel:             CombinedModel{},
			expError:             "some error",
		},
		{
			name: "valid content, no internal properties",
			contentModel: ContentModel{
				"uuid":  "622de808-3a7a-49bd-a7fb-2a33f64695be",
				"title": "Title",
				"body":  "<body>something relevant here</body>",
				"identifiers": []Identifier{
					{
						Authority:       "FTCOM-METHODE_identifier",
						IdentifierValue: "53217c65-ecef-426e-a3ac-3787e2e62e87",
					},
				},
				"publishedDate":      "2017-04-10T08:03:58.000Z",
				"lastModified":       "2017-04-10T08:09:01.808Z",
				"firstPublishedDate": "2017-04-10T08:03:58.000Z",
				"mediaType":          "mediaType",
				"byline":             "FT Reporters",
				"standfirst":         "A simple line with an article summary",
				"description":        "descr",
				"mainImage":          "2934de46-5240-4c7d-8576-f12ae12e4a37",
				"publishReference":   "tid_unique_reference",
			},
			internalContentModel: ContentModel{
				"uuid":  "622de808-3a7a-49bd-a7fb-2a33f64695be",
				"title": "Title",
				"body":  "<body>something relevant here</body>",
				"identifiers": []Identifier{
					{
						Authority:       "FTCOM-METHODE_identifier",
						IdentifierValue: "53217c65-ecef-426e-a3ac-3787e2e62e87",
					},
				},
				"publishedDate":      "2017-04-10T08:03:58.000Z",
				"lastModified":       "2017-04-10T08:09:01.808Z",
				"firstPublishedDate": "2017-04-10T08:03:58.000Z",
				"mediaType":          "mediaType",
				"byline":             "FT Reporters",
				"standfirst":         "A simple line with an article summary",
				"description":        "descr",
				"mainImage":          "2934de46-5240-4c7d-8576-f12ae12e4a37",
				"publishReference":   "tid_unique_reference",
			},
			retrievedAnn: []Annotation{
				{
					Thing: Thing{
						ID:        "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
						PrefLabel: "Barclays",
						Types: []string{"http://base-url/core/Thing",
							"http://base-url/concept/Concept",
							"http://base-url/organisation/Organisation",
							"http://base-url/company/Company",
							"http://base-url/company/PublicCompany",
						},
						Predicate: "http://base-url/about",
						APIURL:    "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
					},
				},
			},
			retrievedErr: nil,
			expModel: CombinedModel{
				UUID: "622de808-3a7a-49bd-a7fb-2a33f64695be",
				Content: ContentModel{
					"uuid":  "622de808-3a7a-49bd-a7fb-2a33f64695be",
					"title": "Title",
					"body":  "<body>something relevant here</body>",
					"identifiers": []Identifier{
						{
							Authority:       "FTCOM-METHODE_identifier",
							IdentifierValue: "53217c65-ecef-426e-a3ac-3787e2e62e87",
						},
					},
					"publishedDate":      "2017-04-10T08:03:58.000Z",
					"lastModified":       "2017-04-10T08:09:01.808Z",
					"firstPublishedDate": "2017-04-10T08:03:58.000Z",
					"mediaType":          "mediaType",
					"byline":             "FT Reporters",
					"standfirst":         "A simple line with an article summary",
					"description":        "descr",
					"mainImage":          "2934de46-5240-4c7d-8576-f12ae12e4a37",
					"publishReference":   "tid_unique_reference",
				},
				InternalContent: ContentModel{
					"uuid":  "622de808-3a7a-49bd-a7fb-2a33f64695be",
					"title": "Title",
					"body":  "<body>something relevant here</body>",
					"identifiers": []Identifier{
						{
							Authority:       "FTCOM-METHODE_identifier",
							IdentifierValue: "53217c65-ecef-426e-a3ac-3787e2e62e87",
						},
					},
					"publishedDate":      "2017-04-10T08:03:58.000Z",
					"lastModified":       "2017-04-10T08:09:01.808Z",
					"firstPublishedDate": "2017-04-10T08:03:58.000Z",
					"mediaType":          "mediaType",
					"byline":             "FT Reporters",
					"standfirst":         "A simple line with an article summary",
					"description":        "descr",
					"mainImage":          "2934de46-5240-4c7d-8576-f12ae12e4a37",
					"publishReference":   "tid_unique_reference",
				},
				Metadata: []Annotation{
					{
						Thing: Thing{
							ID:        "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
							PrefLabel: "Barclays",
							Types: []string{"http://base-url/core/Thing",
								"http://base-url/concept/Concept",
								"http://base-url/organisation/Organisation",
								"http://base-url/company/Company",
								"http://base-url/company/PublicCompany",
							},
							Predicate: "http://base-url/about",
							APIURL:    "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
						},
					},
				},
				LastModified: "2017-04-10T08:09:01.808Z",
			},
			expError: "",
		},
		{
			name: "valid content, no internal properties, extended annotations",
			contentModel: ContentModel{
				"uuid":  "3cd58c64-039e-4147-a744-af676de1691d",
				"title": "Tokyo Olympics will need bailout if games go ahead without spectators",
				"body":  "<body><p>The Tokyo Olympics will need a public bailout of about $800m if the games are held behind closed doors, as organisers delay a decision on domestic spectators to the last possible moment.</p></body>",
				"identifiers": []Identifier{
					{
						Authority:       "http://api.ft.com/system/cct",
						IdentifierValue: "3cd58c64-039e-4147-a744-af676de1691d",
					},
				},
				"publishedDate":      "2021-06-16T00:30:27.145Z",
				"lastModified":       "2021-06-16T01:06:11.671Z",
				"firstPublishedDate": "2021-06-16T00:30:27.145Z",
				"publishReference":   "tid_unique_reference",
				"editorialDesk":      "/FT/WorldNews/Asia Pacific",
				"canBeDistributed":   "yes",
				"canBeSyndicated":    "yes",
				"comments": map[string]bool{
					"enabled": true,
				},
			},
			internalContentModel: ContentModel{
				"uuid":  "3cd58c64-039e-4147-a744-af676de1691d",
				"title": "Tokyo Olympics will need bailout if games go ahead without spectators",
				"body":  "<body><p>The Tokyo Olympics will need a public bailout of about $800m if the games are held behind closed doors, as organisers delay a decision on domestic spectators to the last possible moment.</p></body>",
				"identifiers": []Identifier{
					{
						Authority:       "http://api.ft.com/system/cct",
						IdentifierValue: "3cd58c64-039e-4147-a744-af676de1691d",
					},
				},
				"publishedDate":      "2021-06-16T00:30:27.145Z",
				"lastModified":       "2021-06-16T01:06:11.671Z",
				"firstPublishedDate": "2021-06-16T00:30:27.145Z",
				"publishReference":   "tid_unique_reference",
				"editorialDesk":      "/FT/WorldNews/Asia Pacific",
				"canBeDistributed":   "yes",
				"canBeSyndicated":    "yes",
				"comments": map[string]bool{
					"enabled": true,
				},
			},
			retrievedAnn: []Annotation{
				{
					Thing: Thing{

						NAICS: []IndustryClassification{
							{
								Identifier: "611620",
								PrefLabel:  "Sports and Recreation Instruction",
								Rank:       1,
							},
						},
						APIURL:     "http://api.ft.com/organisations/8ec5ba9a-7648-4d72-a3ca-18c0ab3defe5",
						DirectType: "http://www.ft.com/ontology/organisation/Organisation",
						ID:         "http://api.ft.com/things/8ec5ba9a-7648-4d72-a3ca-18c0ab3defe5",
						LeiCode:    "50670095ZD640VOZOO03",
						Predicate:  "http://www.ft.com/ontology/annotation/mentions",
						PrefLabel:  "International Olympic Committee",
						Type:       "ORGANISATION",
						Types: []string{
							"http://www.ft.com/ontology/core/Thing",
							"http://www.ft.com/ontology/concept/Concept",
							"http://www.ft.com/ontology/organisation/Organisation",
						},
					},
				},
			},
			retrievedErr: nil,
			expModel: CombinedModel{
				UUID: "3cd58c64-039e-4147-a744-af676de1691d",
				Content: ContentModel{
					"uuid":  "3cd58c64-039e-4147-a744-af676de1691d",
					"title": "Tokyo Olympics will need bailout if games go ahead without spectators",
					"body":  "<body><p>The Tokyo Olympics will need a public bailout of about $800m if the games are held behind closed doors, as organisers delay a decision on domestic spectators to the last possible moment.</p></body>",
					"identifiers": []Identifier{
						{
							Authority:       "http://api.ft.com/system/cct",
							IdentifierValue: "3cd58c64-039e-4147-a744-af676de1691d",
						},
					},
					"publishedDate":      "2021-06-16T00:30:27.145Z",
					"lastModified":       "2021-06-16T01:06:11.671Z",
					"firstPublishedDate": "2021-06-16T00:30:27.145Z",
					"publishReference":   "tid_unique_reference",
					"editorialDesk":      "/FT/WorldNews/Asia Pacific",
					"canBeDistributed":   "yes",
					"canBeSyndicated":    "yes",
					"comments": map[string]bool{
						"enabled": true,
					},
				},
				InternalContent: ContentModel{
					"uuid":  "3cd58c64-039e-4147-a744-af676de1691d",
					"title": "Tokyo Olympics will need bailout if games go ahead without spectators",
					"body":  "<body><p>The Tokyo Olympics will need a public bailout of about $800m if the games are held behind closed doors, as organisers delay a decision on domestic spectators to the last possible moment.</p></body>",
					"identifiers": []Identifier{
						{
							Authority:       "http://api.ft.com/system/cct",
							IdentifierValue: "3cd58c64-039e-4147-a744-af676de1691d",
						},
					},
					"publishedDate":      "2021-06-16T00:30:27.145Z",
					"lastModified":       "2021-06-16T01:06:11.671Z",
					"firstPublishedDate": "2021-06-16T00:30:27.145Z",
					"publishReference":   "tid_unique_reference",
					"editorialDesk":      "/FT/WorldNews/Asia Pacific",
					"canBeDistributed":   "yes",
					"canBeSyndicated":    "yes",
					"comments": map[string]bool{
						"enabled": true,
					},
				},
				Metadata: []Annotation{
					{
						Thing: Thing{

							NAICS: []IndustryClassification{
								{
									Identifier: "611620",
									PrefLabel:  "Sports and Recreation Instruction",
									Rank:       1,
								},
							},
							APIURL:     "http://api.ft.com/organisations/8ec5ba9a-7648-4d72-a3ca-18c0ab3defe5",
							DirectType: "http://www.ft.com/ontology/organisation/Organisation",
							ID:         "http://api.ft.com/things/8ec5ba9a-7648-4d72-a3ca-18c0ab3defe5",
							LeiCode:    "50670095ZD640VOZOO03",
							Predicate:  "http://www.ft.com/ontology/annotation/mentions",
							PrefLabel:  "International Olympic Committee",
							Type:       "ORGANISATION",
							Types: []string{
								"http://www.ft.com/ontology/core/Thing",
								"http://www.ft.com/ontology/concept/Concept",
								"http://www.ft.com/ontology/organisation/Organisation",
							},
						},
					},
				},
				LastModified: "2021-06-16T01:06:11.671Z",
			},
			expError: "",
		},
		{
			name: "valid content with internal properties",
			contentModel: ContentModel{
				"uuid":          "67731161-9ca4-4074-a50b-878b163bb02d",
				"id":            "http://www.ft.com/thing/67731161-9ca4-4074-a50b-878b163bb02d",
				"prefLabel":     "Coronavirus latest: Taj Mahal reopens as India’s Covid wave recedes",
				"publishedDate": "2021-06-15T22:26:35.076Z",
				"lastModified":  "2021-06-16T11:56:05.331Z",
				"realtime":      true,
				"types": []interface{}{
					"http://www.ft.com/ontology/content/LiveBlogPackage",
				},
			},
			internalContentModel: ContentModel{
				"id": "http://www.ft.com/thing/67731161-9ca4-4074-a50b-878b163bb02d",
				"leadImages": []interface{}{
					map[string]interface{}{
						"id":   "https://api.ft.com/content/adc816ef-58c3-46c2-9fca-ddfdab8ae42d",
						"type": "standard",
					},
					map[string]interface{}{
						"id":   "https://api.ft.com/content/2e9c8fe2-2f1a-4f77-bd77-11a79236a788",
						"type": "square",
					},
					map[string]interface{}{
						"id":   "https://api.ft.com/content/1c41707b-b0ba-4cf7-bee5-f9a599d64486",
						"type": "wide",
					},
				},
				"prefLabel":     "Coronavirus latest: Taj Mahal reopens as India’s Covid wave recedes",
				"publishedDate": "2021-06-15T22:26:35.076Z",
				"realtime":      true,
				"summary": map[string]interface{}{
					"bodyXML": "<body></body>",
				},
				"topper": map[string]interface{}{
					"backgroundColour": "auto",
					"layout":           "full-bleed-offset",
				},
				"type": "http://www.ft.com/ontology/content/LiveBlogPackage",
				"types": []interface{}{
					"http://www.ft.com/ontology/content/LiveBlogPackage",
				},
			},
			retrievedAnn: []Annotation{},
			retrievedErr: nil,
			expModel: CombinedModel{
				UUID: "67731161-9ca4-4074-a50b-878b163bb02d",
				Content: ContentModel{
					"uuid":          "67731161-9ca4-4074-a50b-878b163bb02d",
					"id":            "http://www.ft.com/thing/67731161-9ca4-4074-a50b-878b163bb02d",
					"prefLabel":     "Coronavirus latest: Taj Mahal reopens as India’s Covid wave recedes",
					"publishedDate": "2021-06-15T22:26:35.076Z",
					"lastModified":  "2021-06-16T11:56:05.331Z",
					"realtime":      true,
					"types": []interface{}{
						"http://www.ft.com/ontology/content/LiveBlogPackage",
					},
				},
				InternalContent: ContentModel{
					"id": "http://www.ft.com/thing/67731161-9ca4-4074-a50b-878b163bb02d",
					"leadImages": []interface{}{
						map[string]interface{}{
							"id":   "https://api.ft.com/content/adc816ef-58c3-46c2-9fca-ddfdab8ae42d",
							"type": "standard",
						},
						map[string]interface{}{
							"id":   "https://api.ft.com/content/2e9c8fe2-2f1a-4f77-bd77-11a79236a788",
							"type": "square",
						},
						map[string]interface{}{
							"id":   "https://api.ft.com/content/1c41707b-b0ba-4cf7-bee5-f9a599d64486",
							"type": "wide",
						},
					},
					"prefLabel":     "Coronavirus latest: Taj Mahal reopens as India’s Covid wave recedes",
					"publishedDate": "2021-06-15T22:26:35.076Z",
					"realtime":      true,
					"summary": map[string]interface{}{
						"bodyXML": "<body></body>",
					},
					"topper": map[string]interface{}{
						"backgroundColour": "auto",
						"layout":           "full-bleed-offset",
					},
					"type": "http://www.ft.com/ontology/content/LiveBlogPackage",
					"types": []interface{}{
						"http://www.ft.com/ontology/content/LiveBlogPackage",
					},
				},
				Metadata:     []Annotation{},
				LastModified: "2021-06-16T11:56:05.331Z",
			},
			expError: "",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			combiner := DataCombiner{
				internalContentRetriever: DummyInternalContentRetriever{testCase.internalContentModel, testCase.retrievedAnn, testCase.retrievedErr},
			}
			m, err := combiner.GetCombinedModelForContent(testCase.contentModel)
			assert.True(t, reflect.DeepEqual(testCase.expModel, m))
			if testCase.expError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, testCase.expError)
			}
		})
	}
}

func TestGetCombinedModelForAnnotations(t *testing.T) {
	tests := []struct {
		name                string
		metadata            AnnotationsMessage
		retrievedContent    ContentModel
		retrievedContentErr error
		retrievedAnn        []Annotation
		retrievedAnnErr     error
		expModel            CombinedModel
		expError            string
	}{
		{
			name:     "no content UUID",
			expError: "annotations have no UUID referenced",
		},
		{
			name:                "content error",
			metadata:            AnnotationsMessage{Annotations: &AnnotationsModel{UUID: "some_uuid"}},
			retrievedContentErr: fmt.Errorf("some content error"),
			expError:            "some content error",
		},
		{
			name:                "content & annotations error",
			metadata:            AnnotationsMessage{Annotations: &AnnotationsModel{UUID: "some_uuid"}},
			retrievedContentErr: fmt.Errorf("some content error"),
			retrievedAnnErr:     fmt.Errorf("some metadata error"),
			expError:            "some content error",
		},
		{
			name:     "annotations error",
			metadata: AnnotationsMessage{Annotations: &AnnotationsModel{UUID: "some_uuid"}},
			retrievedContent: ContentModel{
				"uuid":  "some_uuid",
				"title": "title",
				"body":  "body",
			},
			retrievedAnnErr: fmt.Errorf("some metadata error"),
			expError:        "some metadata error",
		},
		{
			name:     "valid content and annotations",
			metadata: AnnotationsMessage{Annotations: &AnnotationsModel{UUID: "some_uuid"}},
			retrievedContent: ContentModel{
				"uuid":  "some_uuid",
				"title": "title",
				"body":  "body",
			},
			retrievedAnn: []Annotation{
				{Thing{
					ID:        "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
					PrefLabel: "Barclays",
					Types: []string{"http://base-url/core/Thing",
						"http://base-url/concept/Concept",
					},
					Predicate: "http://base-url/about",
					APIURL:    "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
				},
				},
			},
			expModel: CombinedModel{
				UUID: "some_uuid",
				Content: ContentModel{
					"uuid":  "some_uuid",
					"title": "title",
					"body":  "body",
				},
				InternalContent: ContentModel{
					"uuid":  "some_uuid",
					"title": "title",
					"body":  "body",
				},
				Metadata: []Annotation{
					{Thing{
						ID:        "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
						PrefLabel: "Barclays",
						Types: []string{"http://base-url/core/Thing",
							"http://base-url/concept/Concept",
						},
						Predicate: "http://base-url/about",
						APIURL:    "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
					},
					},
				},
			},
		},
		{
			name:     "valid video content & annotations",
			metadata: AnnotationsMessage{Annotations: &AnnotationsModel{UUID: "some_uuid"}},
			retrievedContent: ContentModel{
				"uuid":  "some_uuid",
				"title": "title",
				"body":  "body",
				"type":  "Video",
				"identifiers": []Identifier{
					{
						Authority:       "http://api.ft.com/system/NEXT-VIDEO-EDITOR",
						IdentifierValue: "some_uuid",
					},
				},
			},
			retrievedAnn: []Annotation{
				{Thing{
					ID:        "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
					PrefLabel: "Barclays",
					Types: []string{"http://base-url/core/Thing",
						"http://base-url/concept/Concept",
					},
					Predicate: "http://base-url/about",
					APIURL:    "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
				},
				},
			},
			expModel: CombinedModel{
				UUID: "some_uuid",
				Content: ContentModel{
					"uuid":  "some_uuid",
					"title": "title",
					"body":  "body",
					"type":  "Video",
					"identifiers": []Identifier{
						{
							Authority:       "http://api.ft.com/system/NEXT-VIDEO-EDITOR",
							IdentifierValue: "some_uuid",
						},
					},
				},
				InternalContent: ContentModel{
					"uuid":  "some_uuid",
					"title": "title",
					"body":  "body",
					"type":  "Video",
					"identifiers": []Identifier{
						{
							Authority:       "http://api.ft.com/system/NEXT-VIDEO-EDITOR",
							IdentifierValue: "some_uuid",
						},
					},
				},
				Metadata: []Annotation{
					{
						Thing{
							ID:        "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
							PrefLabel: "Barclays",
							Types: []string{"http://base-url/core/Thing",
								"http://base-url/concept/Concept",
							},
							Predicate: "http://base-url/about",
							APIURL:    "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
						},
					},
				},
			},
		},
		{
			name:     "valid content and extended annotations",
			metadata: AnnotationsMessage{Annotations: &AnnotationsModel{UUID: "67731161-9ca4-4074-a50b-878b163bb02d"}},
			retrievedContent: ContentModel{
				"prefLabel":     "Coronavirus latest: Taj Mahal reopens as India’s Covid wave recedes",
				"publishedDate": "2021-06-15T22:26:35.076Z",
				"realtime":      true,
				"title":         "Coronavirus latest: Taj Mahal reopens as India’s Covid wave recedes",
				"type":          "http://www.ft.com/ontology/content/LiveBlogPackage",
				"types": []string{
					"http://www.ft.com/ontology/content/LiveBlogPackage",
				},
				"uuid": "67731161-9ca4-4074-a50b-878b163bb02d",
			},
			retrievedContentErr: nil,
			retrievedAnn: []Annotation{
				{
					Thing{
						APIURL:     "http://api.ft.com/people/65b38eaf-5f5c-447a-b5e6-59965c8a5055",
						DirectType: "http://www.ft.com/ontology/person/Person",
						ID:         "http://api.ft.com/things/65b38eaf-5f5c-447a-b5e6-59965c8a5055",
						Predicate:  "http://www.ft.com/ontology/hasContributor",
						PrefLabel:  "Leke Oso Alabi",
						Type:       "PERSON",
						Types: []string{
							"http://www.ft.com/ontology/core/Thing",
							"http://www.ft.com/ontology/concept/Concept",
							"http://www.ft.com/ontology/person/Person",
						},
					},
				},
				{
					Thing{
						APIURL:     "http://api.ft.com/things/9a6861ff-50ef-4e40-acf7-6659e127ae4e",
						DirectType: "http://www.ft.com/ontology/Location",
						ID:         "http://api.ft.com/things/9a6861ff-50ef-4e40-acf7-6659e127ae4e",
						Predicate:  "http://www.ft.com/ontology/annotation/mentions",
						PrefLabel:  "India",
						Type:       "LOCATION",
						Types: []string{
							"http://www.ft.com/ontology/core/Thing",
							"http://www.ft.com/ontology/concept/Concept",
							"http://www.ft.com/ontology/Location",
						},
					},
				},
			},
			retrievedAnnErr: nil,
			expModel: CombinedModel{
				UUID: "67731161-9ca4-4074-a50b-878b163bb02d",
				Content: ContentModel{
					"prefLabel":     "Coronavirus latest: Taj Mahal reopens as India’s Covid wave recedes",
					"publishedDate": "2021-06-15T22:26:35.076Z",
					"realtime":      true,
					"title":         "Coronavirus latest: Taj Mahal reopens as India’s Covid wave recedes",
					"type":          "http://www.ft.com/ontology/content/LiveBlogPackage",
					"types": []string{
						"http://www.ft.com/ontology/content/LiveBlogPackage",
					},
					"uuid": "67731161-9ca4-4074-a50b-878b163bb02d",
				},
				InternalContent: ContentModel{
					"prefLabel":     "Coronavirus latest: Taj Mahal reopens as India’s Covid wave recedes",
					"publishedDate": "2021-06-15T22:26:35.076Z",
					"realtime":      true,
					"title":         "Coronavirus latest: Taj Mahal reopens as India’s Covid wave recedes",
					"type":          "http://www.ft.com/ontology/content/LiveBlogPackage",
					"types": []string{
						"http://www.ft.com/ontology/content/LiveBlogPackage",
					},
					"uuid": "67731161-9ca4-4074-a50b-878b163bb02d",
				},
				Metadata: []Annotation{
					{
						Thing{
							APIURL:     "http://api.ft.com/people/65b38eaf-5f5c-447a-b5e6-59965c8a5055",
							DirectType: "http://www.ft.com/ontology/person/Person",
							ID:         "http://api.ft.com/things/65b38eaf-5f5c-447a-b5e6-59965c8a5055",
							Predicate:  "http://www.ft.com/ontology/hasContributor",
							PrefLabel:  "Leke Oso Alabi",
							Type:       "PERSON",
							Types: []string{
								"http://www.ft.com/ontology/core/Thing",
								"http://www.ft.com/ontology/concept/Concept",
								"http://www.ft.com/ontology/person/Person",
							},
						},
					},
					{
						Thing{
							APIURL:     "http://api.ft.com/things/9a6861ff-50ef-4e40-acf7-6659e127ae4e",
							DirectType: "http://www.ft.com/ontology/Location",
							ID:         "http://api.ft.com/things/9a6861ff-50ef-4e40-acf7-6659e127ae4e",
							Predicate:  "http://www.ft.com/ontology/annotation/mentions",
							PrefLabel:  "India",
							Type:       "LOCATION",
							Types: []string{
								"http://www.ft.com/ontology/core/Thing",
								"http://www.ft.com/ontology/concept/Concept",
								"http://www.ft.com/ontology/Location",
							},
						},
					},
				},
			},
			expError: "",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			combiner := DataCombiner{
				contentRetriever:           DummyContentRetriever{testCase.retrievedContent, testCase.retrievedContentErr},
				contentCollectionRetriever: DummyContentRetriever{ContentModel{}, nil},
				internalContentRetriever:   DummyInternalContentRetriever{testCase.retrievedContent, testCase.retrievedAnn, testCase.retrievedAnnErr},
			}

			m, err := combiner.GetCombinedModelForAnnotations(testCase.metadata)
			assert.Equal(t, testCase.expModel, m)
			if testCase.expError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, testCase.expError)
			}
		})
	}
}

func TestGetCombinedModel(t *testing.T) {
	tests := []struct {
		name                          string
		retrievedContent              ContentModel
		retrievedContentErr           error
		retrievedAnn                  []Annotation
		retrievedAnnErr               error
		retrievedContentCollection    ContentModel
		retrievedContentCollectionErr error
		expModel                      CombinedModel
		expError                      error
	}{
		{
			name:                       "content not found",
			retrievedContent:           ContentModel{},
			retrievedContentCollection: ContentModel{},
			expModel: CombinedModel{
				UUID:            "some-uuid",
				Content:         ContentModel{},
				InternalContent: nil,
				Metadata:        nil,
			},
		},
		{
			name:                          "content collection retrieval error",
			retrievedContent:              ContentModel{},
			retrievedContentCollectionErr: fmt.Errorf("some content collection retrieval error"),
			expError:                      fmt.Errorf("some content collection retrieval error"),
		},
		{
			name:             "valid content collection",
			retrievedContent: ContentModel{},
			retrievedContentCollection: ContentModel{
				"uuid": "some-uuid",
				"items": []map[string]string{
					{
						"uuid": "d9403324-6d33-11e7-bfeb-33fe0c5b7eaa",
					},
					{
						"uuid": "427b8cb0-71d7-11e7-aca6-c6bd07df1a3c",
					},
				},
				"publishReference": "tid_123456",
				"lastModified":     "2020-10-31T10:39:47.598Z",
			},
			expModel: CombinedModel{
				UUID: "some-uuid",
				Content: ContentModel{
					"uuid": "some-uuid",
					"items": []map[string]string{
						{
							"uuid": "d9403324-6d33-11e7-bfeb-33fe0c5b7eaa",
						},
						{
							"uuid": "427b8cb0-71d7-11e7-aca6-c6bd07df1a3c",
						},
					},
					"type":             "ContentCollection",
					"publishReference": "tid_123456",
					"lastModified":     "2020-10-31T10:39:47.598Z",
				},
				InternalContent: nil,
				Metadata:        nil,
				LastModified:    "2020-10-31T10:39:47.598Z",
			},
		},
		{
			name: "valid content and extended annotations",
			retrievedContent: ContentModel{
				"prefLabel":     "Coronavirus latest: Taj Mahal reopens as India’s Covid wave recedes",
				"publishedDate": "2021-06-15T22:26:35.076Z",
				"realtime":      true,
				"title":         "Coronavirus latest: Taj Mahal reopens as India’s Covid wave recedes",
				"type":          "http://www.ft.com/ontology/content/LiveBlogPackage",
				"types": []string{
					"http://www.ft.com/ontology/content/LiveBlogPackage",
				},
				"uuid": "some-uuid",
			},
			retrievedContentErr: nil,
			retrievedAnn: []Annotation{
				{
					Thing{
						APIURL:     "http://api.ft.com/people/65b38eaf-5f5c-447a-b5e6-59965c8a5055",
						DirectType: "http://www.ft.com/ontology/person/Person",
						ID:         "http://api.ft.com/things/65b38eaf-5f5c-447a-b5e6-59965c8a5055",
						Predicate:  "http://www.ft.com/ontology/hasContributor",
						PrefLabel:  "Leke Oso Alabi",
						Type:       "PERSON",
						Types: []string{
							"http://www.ft.com/ontology/core/Thing",
							"http://www.ft.com/ontology/concept/Concept",
							"http://www.ft.com/ontology/person/Person",
						},
					},
				},
				{
					Thing{
						APIURL:     "http://api.ft.com/things/9a6861ff-50ef-4e40-acf7-6659e127ae4e",
						DirectType: "http://www.ft.com/ontology/Location",
						ID:         "http://api.ft.com/things/9a6861ff-50ef-4e40-acf7-6659e127ae4e",
						Predicate:  "http://www.ft.com/ontology/annotation/mentions",
						PrefLabel:  "India",
						Type:       "LOCATION",
						Types: []string{
							"http://www.ft.com/ontology/core/Thing",
							"http://www.ft.com/ontology/concept/Concept",
							"http://www.ft.com/ontology/Location",
						},
					},
				},
			},
			retrievedAnnErr: nil,
			expModel: CombinedModel{
				UUID: "some-uuid",
				Content: ContentModel{
					"prefLabel":     "Coronavirus latest: Taj Mahal reopens as India’s Covid wave recedes",
					"publishedDate": "2021-06-15T22:26:35.076Z",
					"realtime":      true,
					"title":         "Coronavirus latest: Taj Mahal reopens as India’s Covid wave recedes",
					"type":          "http://www.ft.com/ontology/content/LiveBlogPackage",
					"types": []string{
						"http://www.ft.com/ontology/content/LiveBlogPackage",
					},
					"uuid": "some-uuid",
				},
				InternalContent: ContentModel{
					"prefLabel":     "Coronavirus latest: Taj Mahal reopens as India’s Covid wave recedes",
					"publishedDate": "2021-06-15T22:26:35.076Z",
					"realtime":      true,
					"title":         "Coronavirus latest: Taj Mahal reopens as India’s Covid wave recedes",
					"type":          "http://www.ft.com/ontology/content/LiveBlogPackage",
					"types": []string{
						"http://www.ft.com/ontology/content/LiveBlogPackage",
					},
					"uuid": "some-uuid",
				},
				Metadata: []Annotation{
					{
						Thing{
							APIURL:     "http://api.ft.com/people/65b38eaf-5f5c-447a-b5e6-59965c8a5055",
							DirectType: "http://www.ft.com/ontology/person/Person",
							ID:         "http://api.ft.com/things/65b38eaf-5f5c-447a-b5e6-59965c8a5055",
							Predicate:  "http://www.ft.com/ontology/hasContributor",
							PrefLabel:  "Leke Oso Alabi",
							Type:       "PERSON",
							Types: []string{
								"http://www.ft.com/ontology/core/Thing",
								"http://www.ft.com/ontology/concept/Concept",
								"http://www.ft.com/ontology/person/Person",
							},
						},
					},
					{
						Thing{
							APIURL:     "http://api.ft.com/things/9a6861ff-50ef-4e40-acf7-6659e127ae4e",
							DirectType: "http://www.ft.com/ontology/Location",
							ID:         "http://api.ft.com/things/9a6861ff-50ef-4e40-acf7-6659e127ae4e",
							Predicate:  "http://www.ft.com/ontology/annotation/mentions",
							PrefLabel:  "India",
							Type:       "LOCATION",
							Types: []string{
								"http://www.ft.com/ontology/core/Thing",
								"http://www.ft.com/ontology/concept/Concept",
								"http://www.ft.com/ontology/Location",
							},
						},
					},
				},
			},
			expError: nil,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			combiner := DataCombiner{
				contentRetriever:           DummyContentRetriever{testCase.retrievedContent, testCase.retrievedContentErr},
				contentCollectionRetriever: DummyContentRetriever{testCase.retrievedContentCollection, testCase.retrievedContentCollectionErr},
				internalContentRetriever:   DummyInternalContentRetriever{testCase.retrievedContent, testCase.retrievedAnn, testCase.retrievedAnnErr},
			}

			m, err := combiner.GetCombinedModel("some-uuid")
			assert.Equal(t, testCase.expModel, m,
				fmt.Sprintf("Expected model: %v was not equal with the received one: %v \n", testCase.expModel, m))
			if testCase.expError == nil {
				assert.Equal(t, nil, err)
			} else {
				assert.Contains(t, err.Error(), testCase.expError.Error())
			}
		})
	}
}

func TestGetInternalContent(t *testing.T) {
	tests := []struct {
		name           string
		dc             httputils.Client
		expContent     ContentModel
		expAnnotations []Annotation
		expError       string
	}{
		{
			name: "internal content - not found",
			dc: dummyClient{
				statusCode: http.StatusNotFound,
			},
			expContent:     nil,
			expAnnotations: []Annotation(nil), //empty value for a slice
			expError:       "",
		},
		{
			name: "internal content - error response",
			dc: dummyClient{
				err: fmt.Errorf("some error"),
			},
			expContent:     nil,
			expAnnotations: []Annotation(nil), //empty value for a slice
			expError:       "error executing request for url \"some_host/some_endpoint\": some error",
		},
		{
			name: "internal content - invalid body",
			dc: dummyClient{
				statusCode: http.StatusOK,
				body:       "text that can't be unmarshalled",
			},
			expContent:     nil,
			expAnnotations: []Annotation(nil),
			expError:       "error unmarshalling internal content: invalid character 'e' in literal true (expecting 'r')",
		},
		{
			name: "internal content - valid body",
			dc: dummyClient{
				statusCode: http.StatusOK,
				body:       `{"uuid":"622de808-3a7a-49bd-a7fb-2a33f64695be","title":"Title","alternativeTitles":{"promotionalTitle":"Alternative title"},"type":null,"byline":"FT Reporters","brands":[{"id":"http://api.ft.com/things/40f636a3-5507-4311-9629-95376007cb7b"}],"identifiers":[{"authority":"FTCOM-METHODE_identifier","identifierValue":"53217c65-ecef-426e-a3ac-3787e2e62e87"}],"publishedDate":"2017-04-10T08:03:58.000Z","standfirst":"A simple line with an article summary","body":"<body>something relevant here<\/body>","description":null,"mediaType":null,"pixelWidth":null,"pixelHeight":null,"internalBinaryUrl":null,"externalBinaryUrl":null,"members":null,"mainImage":"2934de46-5240-4c7d-8576-f12ae12e4a37","standout":{"editorsChoice":false,"exclusive":false,"scoop":false},"comments":{"enabled":true},"copyright":null,"webUrl":null,"publishReference":"tid_unique_reference","lastModified":"2017-04-10T08:09:01.808Z","canBeSyndicated":"yes","firstPublishedDate":"2017-04-10T08:03:58.000Z","accessLevel":"subscribed","canBeDistributed":"yes","annotations":[{"predicate":"http://base-url/about","id":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995","apiUrl":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995","types":["http://base-url/core/Thing","http://base-url/concept/Concept","http://base-url/organisation/Organisation","http://base-url/company/Company","http://base-url/company/PublicCompany"],"prefLabel":"Barclays"},{"predicate":"http://base-url/isClassifiedBy","id":"http://base-url/271ee5f7-d808-497d-bed3-1b961953dedc","apiUrl":"http://base-url/271ee5f7-d808-497d-bed3-1b961953dedc","types":["http://base-url/core/Thing","http://base-url/concept/Concept","http://base-url/classification/Classification","http://base-url/Section"],"prefLabel":"Financials"},{"predicate":"http://base-url/majorMentions","id":"http://base-url/a19d07d5-dc28-4c33-8745-a96f193df5cd","apiUrl":"http://base-url/a19d07d5-dc28-4c33-8745-a96f193df5cd","types":["http://base-url/core/Thing","http://base-url/concept/Concept","http://base-url/person/Person"],"prefLabel":"Jes Staley"}]}`,
			},
			expContent: ContentModel{
				"uuid":  "622de808-3a7a-49bd-a7fb-2a33f64695be",
				"title": "Title",
				"alternativeTitles": map[string]interface{}{
					"promotionalTitle": "Alternative title",
				},
				"type":   nil,
				"byline": "FT Reporters",
				"brands": []interface{}{
					map[string]interface{}{
						"id": "http://api.ft.com/things/40f636a3-5507-4311-9629-95376007cb7b",
					},
				},
				"identifiers": []interface{}{
					map[string]interface{}{
						"authority":       "FTCOM-METHODE_identifier",
						"identifierValue": "53217c65-ecef-426e-a3ac-3787e2e62e87",
					},
				},
				"publishedDate":     "2017-04-10T08:03:58.000Z",
				"standfirst":        "A simple line with an article summary",
				"body":              "<body>something relevant here</body>",
				"description":       nil,
				"mediaType":         nil,
				"pixelWidth":        nil,
				"pixelHeight":       nil,
				"internalBinaryUrl": nil,
				"externalBinaryUrl": nil,
				"members":           nil,
				"mainImage":         "2934de46-5240-4c7d-8576-f12ae12e4a37",
				"standout": map[string]interface{}{
					"editorsChoice": false,
					"exclusive":     false,
					"scoop":         false,
				},
				"comments": map[string]interface{}{
					"enabled": true,
				},
				"copyright":          nil,
				"webUrl":             nil,
				"publishReference":   "tid_unique_reference",
				"lastModified":       "2017-04-10T08:09:01.808Z",
				"canBeSyndicated":    "yes",
				"firstPublishedDate": "2017-04-10T08:03:58.000Z",
				"accessLevel":        "subscribed",
				"canBeDistributed":   "yes",
			},
			expAnnotations: []Annotation{
				{
					Thing: Thing{
						ID:        "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
						PrefLabel: "Barclays",
						Types: []string{"http://base-url/core/Thing",
							"http://base-url/concept/Concept",
							"http://base-url/organisation/Organisation",
							"http://base-url/company/Company",
							"http://base-url/company/PublicCompany",
						},
						Predicate: "http://base-url/about",
						APIURL:    "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
					},
				},
				{
					Thing: Thing{
						ID:        "http://base-url/271ee5f7-d808-497d-bed3-1b961953dedc",
						PrefLabel: "Financials",
						Types: []string{"http://base-url/core/Thing",
							"http://base-url/concept/Concept",
							"http://base-url/classification/Classification",
							"http://base-url/Section"},
						Predicate: "http://base-url/isClassifiedBy",
						APIURL:    "http://base-url/271ee5f7-d808-497d-bed3-1b961953dedc",
					},
				},
				{
					Thing: Thing{
						ID:        "http://base-url/a19d07d5-dc28-4c33-8745-a96f193df5cd",
						PrefLabel: "Jes Staley",
						Types: []string{"http://base-url/core/Thing",
							"http://base-url/concept/Concept",
							"http://base-url/person/Person"},
						Predicate: "http://base-url/majorMentions",
						APIURL:    "http://base-url/a19d07d5-dc28-4c33-8745-a96f193df5cd",
					},
				},
			},
			expError: "",
		},
		{
			name: "internal content - valid body with internal content properties & extended annotations",
			dc: dummyClient{
				statusCode: http.StatusOK,
				body:       `{"annotations":[{"apiUrl":"http://api.ft.com/things/5a347026-38cf-4171-a2cf-ec078677add0","directType":"http://www.ft.com/ontology/Topic","id":"http://api.ft.com/things/5a347026-38cf-4171-a2cf-ec078677add0","predicate":"http://www.ft.com/ontology/annotation/mentions","prefLabel":"Olympic Games","type":"TOPIC","types":["http://www.ft.com/ontology/core/Thing","http://www.ft.com/ontology/concept/Concept","http://www.ft.com/ontology/Topic"]},{"NAICS":[{"identifier":"325414","prefLabel":"Biological Product (except Diagnostic) Manufacturing","rank":1}],"apiUrl":"http://api.ft.com/organisations/be2d839b-1969-4694-ae75-abe48d1e904c","directType":"http://www.ft.com/ontology/company/PublicCompany","id":"http://api.ft.com/things/be2d839b-1969-4694-ae75-abe48d1e904c","leiCode":"894500UZJ5LG1F8J1U58","predicate":"http://www.ft.com/ontology/annotation/mentions","prefLabel":"BioNTech","type":"ORGANISATION","types":["http://www.ft.com/ontology/core/Thing","http://www.ft.com/ontology/concept/Concept","http://www.ft.com/ontology/organisation/Organisation","http://www.ft.com/ontology/company/Company","http://www.ft.com/ontology/company/PublicCompany"]}],"id":"http://www.ft.com/thing/67731161-9ca4-4074-a50b-878b163bb02d","leadImages":[{"id":"https://api.ft.com/content/adc816ef-58c3-46c2-9fca-ddfdab8ae42d","type":"standard"},{"id":"https://api.ft.com/content/2e9c8fe2-2f1a-4f77-bd77-11a79236a788","type":"square"},{"id":"https://api.ft.com/content/1c41707b-b0ba-4cf7-bee5-f9a599d64486","type":"wide"}],"prefLabel":"Coronavirus latest: Taj Mahal reopens as India’s Covid wave recedes","publishedDate":"2021-06-15T22:26:35.076Z","realtime":true,"summary":{"bodyXML":"<body></body>"},"topper":{"backgroundColour":"auto","layout":"full-bleed-offset"},"type":"http://www.ft.com/ontology/content/LiveBlogPackage","types":["http://www.ft.com/ontology/content/LiveBlogPackage"]}`,
			},
			expContent: ContentModel{
				"id": "http://www.ft.com/thing/67731161-9ca4-4074-a50b-878b163bb02d",
				"leadImages": []interface{}{
					map[string]interface{}{
						"id":   "https://api.ft.com/content/adc816ef-58c3-46c2-9fca-ddfdab8ae42d",
						"type": "standard",
					},
					map[string]interface{}{
						"id":   "https://api.ft.com/content/2e9c8fe2-2f1a-4f77-bd77-11a79236a788",
						"type": "square",
					},
					map[string]interface{}{
						"id":   "https://api.ft.com/content/1c41707b-b0ba-4cf7-bee5-f9a599d64486",
						"type": "wide",
					},
				},
				"prefLabel":     "Coronavirus latest: Taj Mahal reopens as India’s Covid wave recedes",
				"publishedDate": "2021-06-15T22:26:35.076Z",
				"realtime":      true,
				"summary": map[string]interface{}{
					"bodyXML": "<body></body>",
				},
				"topper": map[string]interface{}{
					"backgroundColour": "auto",
					"layout":           "full-bleed-offset",
				},
				"type": "http://www.ft.com/ontology/content/LiveBlogPackage",
				"types": []interface{}{
					"http://www.ft.com/ontology/content/LiveBlogPackage",
				},
			},
			expAnnotations: []Annotation{
				{
					Thing{
						APIURL:     "http://api.ft.com/things/5a347026-38cf-4171-a2cf-ec078677add0",
						DirectType: "http://www.ft.com/ontology/Topic",
						ID:         "http://api.ft.com/things/5a347026-38cf-4171-a2cf-ec078677add0",
						Predicate:  "http://www.ft.com/ontology/annotation/mentions",
						PrefLabel:  "Olympic Games",
						Type:       "TOPIC",
						Types: []string{
							"http://www.ft.com/ontology/core/Thing",
							"http://www.ft.com/ontology/concept/Concept",
							"http://www.ft.com/ontology/Topic",
						},
					},
				},
				{
					Thing{
						NAICS: []IndustryClassification{
							{
								Identifier: "325414",
								PrefLabel:  "Biological Product (except Diagnostic) Manufacturing",
								Rank:       1,
							},
						},
						APIURL:     "http://api.ft.com/organisations/be2d839b-1969-4694-ae75-abe48d1e904c",
						DirectType: "http://www.ft.com/ontology/company/PublicCompany",
						ID:         "http://api.ft.com/things/be2d839b-1969-4694-ae75-abe48d1e904c",
						LeiCode:    "894500UZJ5LG1F8J1U58",
						Predicate:  "http://www.ft.com/ontology/annotation/mentions",
						PrefLabel:  "BioNTech",
						Type:       "ORGANISATION",
						Types: []string{
							"http://www.ft.com/ontology/core/Thing",
							"http://www.ft.com/ontology/concept/Concept",
							"http://www.ft.com/ontology/organisation/Organisation",
							"http://www.ft.com/ontology/company/Company",
							"http://www.ft.com/ontology/company/PublicCompany",
						},
					},
				},
			},
			expError: "",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			dr := dataRetriever{"some_host/some_endpoint", testCase.dc}
			c, ann, err := dr.getInternalContent("some_uuid")
			assert.Equal(t, testCase.expAnnotations, ann)
			if testCase.expError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, testCase.expError)
			}

			assert.True(t, reflect.DeepEqual(testCase.expContent, c))
		})
	}
}

func TestGetContent(t *testing.T) {
	tests := []struct {
		name       string
		dc         httputils.Client
		expContent ContentModel
		expError   string
	}{
		{
			name: "content - not found",
			dc: dummyClient{
				statusCode: http.StatusNotFound,
			},
			expContent: nil,
			expError:   "",
		},
		{
			name: "content - error",
			dc: dummyClient{
				err: fmt.Errorf("some error"),
			},
			expContent: nil,
			expError:   "error executing request for url \"some_host/some_endpoint\": some error",
		},
		{
			name: "content - invalid body",
			dc: dummyClient{
				statusCode: http.StatusOK,
				body:       "text that can't be unmarshalled",
			},
			expContent: nil,
			expError:   "error unmarshalling content: invalid character 'e' in literal true (expecting 'r')",
		},
		{
			name: "content - valid",
			dc: dummyClient{
				statusCode: http.StatusOK,
				body:       `{"uuid":"622de808-3a7a-49bd-a7fb-2a33f64695be","title":"Title","alternativeTitles":{"promotionalTitle":"Alternative title"},"type":null,"byline":"FT Reporters","brands":[{"id":"http://api.ft.com/things/40f636a3-5507-4311-9629-95376007cb7b"}],"identifiers":[{"authority":"FTCOM-METHODE_identifier","identifierValue":"53217c65-ecef-426e-a3ac-3787e2e62e87"}],"publishedDate":"2017-04-10T08:03:58.000Z","standfirst":"A simple line with an article summary","body":"<body>something relevant here<\/body>","description":null,"mediaType":null,"pixelWidth":null,"pixelHeight":null,"internalBinaryUrl":null,"externalBinaryUrl":null,"members":null,"mainImage":"2934de46-5240-4c7d-8576-f12ae12e4a37","standout":{"editorsChoice":false,"exclusive":false,"scoop":false},"comments":{"enabled":true},"copyright":null,"webUrl":null,"publishReference":"tid_unique_reference","lastModified":"2017-04-10T08:09:01.808Z","canBeSyndicated":"yes","firstPublishedDate":"2017-04-10T08:03:58.000Z","accessLevel":"subscribed","canBeDistributed":"yes"}`,
			},
			expContent: ContentModel{
				"uuid":  "622de808-3a7a-49bd-a7fb-2a33f64695be",
				"title": "Title",
				"alternativeTitles": map[string]interface{}{
					"promotionalTitle": "Alternative title",
				},
				"type":   nil,
				"byline": "FT Reporters",
				"brands": []interface{}{
					map[string]interface{}{
						"id": "http://api.ft.com/things/40f636a3-5507-4311-9629-95376007cb7b",
					},
				},
				"identifiers": []interface{}{
					map[string]interface{}{
						"authority":       "FTCOM-METHODE_identifier",
						"identifierValue": "53217c65-ecef-426e-a3ac-3787e2e62e87",
					},
				},
				"publishedDate":     "2017-04-10T08:03:58.000Z",
				"standfirst":        "A simple line with an article summary",
				"body":              "<body>something relevant here</body>",
				"description":       nil,
				"mediaType":         nil,
				"pixelWidth":        nil,
				"pixelHeight":       nil,
				"internalBinaryUrl": nil,
				"externalBinaryUrl": nil,
				"members":           nil,
				"mainImage":         "2934de46-5240-4c7d-8576-f12ae12e4a37",
				"standout": map[string]interface{}{
					"editorsChoice": false,
					"exclusive":     false,
					"scoop":         false,
				},
				"comments": map[string]interface{}{
					"enabled": true,
				},
				"copyright":          nil,
				"webUrl":             nil,
				"publishReference":   "tid_unique_reference",
				"lastModified":       "2017-04-10T08:09:01.808Z",
				"canBeSyndicated":    "yes",
				"firstPublishedDate": "2017-04-10T08:03:58.000Z",
				"accessLevel":        "subscribed",
				"canBeDistributed":   "yes",
			},
			expError: "",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			dr := dataRetriever{"some_host/some_endpoint", testCase.dc}
			c, err := dr.getContent("some_uuid")

			assert.True(t, reflect.DeepEqual(testCase.expContent, c))
			if testCase.expError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, testCase.expError)
			}
		})
	}
}

type dummyClient struct {
	statusCode int
	body       string
	err        error
}

func (c dummyClient) Do(*http.Request) (*http.Response, error) {
	resp := &http.Response{
		StatusCode: c.statusCode,
		Body:       io.NopCloser(strings.NewReader(c.body)),
	}

	return resp, c.err
}

type DummyContentRetriever struct {
	c   ContentModel
	err error
}

func (r DummyContentRetriever) getContent(uuid string) (ContentModel, error) {
	return r.c, r.err
}

type DummyInternalContentRetriever struct {
	c   ContentModel
	ann []Annotation
	err error
}

func (r DummyInternalContentRetriever) getInternalContent(uuid string) (ContentModel, []Annotation, error) {
	return r.c, r.ann, r.err
}
