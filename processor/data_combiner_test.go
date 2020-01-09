package processor

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/Financial-Times/post-publication-combiner/v2/utils"
	"github.com/stretchr/testify/assert"
)

func TestGetCombinedModelForContent(t *testing.T) {

	tests := []struct {
		contentModel ContentModel
		retrievedAnn []Annotation
		retrievedErr error
		expModel     CombinedModel
		expError     error
	}{
		{
			ContentModel{},
			[]Annotation{},
			nil,
			CombinedModel{},
			errors.New("content has no UUID provided. Can't deduce annotations for it."),
		},
		{
			ContentModel{
				"uuid": "some uuid",
			},
			[]Annotation{},
			errors.New("some error"),
			CombinedModel{},
			errors.New("some error"),
		},
		{
			ContentModel{
				"uuid": "some uuid",
			},
			[]Annotation{},
			errors.New("could not unmarshall annotations for content with uuid"),
			CombinedModel{},
			errors.New("could not unmarshall annotations for content with uuid"),
		},
		{
			ContentModel{
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
			[]Annotation{
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
						ApiUrl:    "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
					},
				},
			},
			nil,
			CombinedModel{
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
							ApiUrl:    "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
						},
					},
				},
				LastModified: "2017-04-10T08:09:01.808Z",
			},
			nil,
		},
	}

	for _, testCase := range tests {
		combiner := DataCombiner{
			MetadataRetriever: DummyMetadataRetriever{testCase.retrievedAnn, testCase.retrievedErr},
		}
		m, err := combiner.GetCombinedModelForContent(testCase.contentModel)
		assert.True(t, reflect.DeepEqual(testCase.expModel, m),
			fmt.Sprintf("Expected model: %v was not equal with the received one: %v \n", testCase.expModel, m))
		if testCase.expError == nil {
			assert.Equal(t, nil, err)
		} else {
			assert.Contains(t, err.Error(), testCase.expError.Error())
		}
	}
}

func TestGetCombinedModelForAnnotations(t *testing.T) {

	tests := []struct {
		metadata            Annotations
		retrievedContent    ContentModel
		retreivedContentErr error
		retrievedAnn        []Annotation
		retreivedAnnErr     error
		expModel            CombinedModel
		expError            error
	}{
		{
			Annotations{},
			ContentModel{},
			nil,
			[]Annotation{},
			nil,
			CombinedModel{},
			errors.New("annotations have no UUID referenced. Can't deduce content for it."),
		},
		{
			Annotations{UUID: "some_uuid"},
			ContentModel{},
			errors.New("some content error"),
			[]Annotation{},
			nil,
			CombinedModel{},
			errors.New("some content error"),
		},
		{
			Annotations{UUID: "some_uuid"},
			ContentModel{},
			errors.New("some content error"),
			[]Annotation{},
			errors.New("some metadata error"),
			CombinedModel{},
			errors.New("some content error"),
		},
		{
			Annotations{UUID: "some_uuid"},
			ContentModel{},
			nil,
			[]Annotation{},
			errors.New("some metadata error"),
			CombinedModel{},
			errors.New("some metadata error"),
		},
		{
			Annotations{UUID: "some_uuid"},
			ContentModel{
				"uuid":  "some_uuid",
				"title": "title",
				"body":  "body",
			},
			nil,
			[]Annotation{
				{Thing{
					ID:        "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
					PrefLabel: "Barclays",
					Types: []string{"http://base-url/core/Thing",
						"http://base-url/concept/Concept",
					},
					Predicate: "http://base-url/about",
					ApiUrl:    "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
				},
				},
			},
			nil,
			CombinedModel{
				UUID: "some_uuid",
				Content: ContentModel{
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
						ApiUrl:    "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
					},
					},
				},
			},
			nil,
		},
		{
			Annotations{UUID: "some_uuid"},
			ContentModel{
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
			nil,
			[]Annotation{
				{Thing{
					ID:        "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
					PrefLabel: "Barclays",
					Types: []string{"http://base-url/core/Thing",
						"http://base-url/concept/Concept",
					},
					Predicate: "http://base-url/about",
					ApiUrl:    "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
				},
				},
			},
			nil,
			CombinedModel{
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
				Metadata: []Annotation{
					{Thing{
						ID:        "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
						PrefLabel: "Barclays",
						Types: []string{"http://base-url/core/Thing",
							"http://base-url/concept/Concept",
						},
						Predicate: "http://base-url/about",
						ApiUrl:    "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
					},
					},
				},
			},
			nil,
		},
	}

	for _, testCase := range tests {
		combiner := DataCombiner{
			ContentRetriever:  DummyContentRetriever{testCase.retrievedContent, testCase.retreivedContentErr},
			MetadataRetriever: DummyMetadataRetriever{testCase.retrievedAnn, testCase.retreivedAnnErr},
		}

		m, err := combiner.GetCombinedModelForAnnotations(testCase.metadata)
		assert.Equal(t, testCase.expModel, m,
			fmt.Sprintf("Expected model: %v was not equal with the received one: %v \n", testCase.expModel, m))
		if testCase.expError == nil {
			assert.Equal(t, nil, err)
		} else {
			assert.Contains(t, err.Error(), testCase.expError.Error())
		}
	}
}

func TestGetAnnotations(t *testing.T) {

	tests := []struct {
		uuid           string
		address        utils.ApiURL
		dc             utils.Client
		expAnnotations []Annotation
		expError       error
	}{
		{
			"some_uuid",
			utils.ApiURL{"some_host", "some_endpoint"},
			dummyClient{
				statusCode: http.StatusNotFound,
			},
			[]Annotation(nil), //empty value for a slice
			nil,
		},
		{
			"some_uuid",
			utils.ApiURL{"some_host", "some_endpoint"},
			dummyClient{
				err: errors.New("some error"),
			},
			[]Annotation(nil), //empty value for a slice
			errors.New("some error"),
		},
		{
			"some_uuid",
			utils.ApiURL{"some_host", "some_endpoint"},
			dummyClient{
				statusCode: http.StatusOK,
				body:       "text that can't be unmarshalled",
			},
			[]Annotation(nil),
			errors.New("could not unmarshall annotations for content with uuid=some_uuid"),
		},
		{
			"some_uuid",
			utils.ApiURL{"some_host", "some_endpoint"},
			dummyClient{
				statusCode: http.StatusOK,
				body:       `[{"predicate":"http://base-url/about","id":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995","apiUrl":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995","types":["http://base-url/core/Thing","http://base-url/concept/Concept","http://base-url/organisation/Organisation","http://base-url/company/Company","http://base-url/company/PublicCompany"],"prefLabel":"Barclays"},{"predicate":"http://base-url/isClassifiedBy","id":"http://base-url/271ee5f7-d808-497d-bed3-1b961953dedc","apiUrl":"http://base-url/271ee5f7-d808-497d-bed3-1b961953dedc","types":["http://base-url/core/Thing","http://base-url/concept/Concept","http://base-url/classification/Classification","http://base-url/Section"],"prefLabel":"Financials"},{"predicate":"http://base-url/majorMentions","id":"http://base-url/a19d07d5-dc28-4c33-8745-a96f193df5cd","apiUrl":"http://base-url/a19d07d5-dc28-4c33-8745-a96f193df5cd","types":["http://base-url/core/Thing","http://base-url/concept/Concept","http://base-url/person/Person"],"prefLabel":"Jes Staley"}]`,
			},
			[]Annotation{
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
						ApiUrl:    "http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
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
						ApiUrl:    "http://base-url/271ee5f7-d808-497d-bed3-1b961953dedc",
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
						ApiUrl:    "http://base-url/a19d07d5-dc28-4c33-8745-a96f193df5cd",
					},
				},
			},
			nil,
		},
	}

	for _, testCase := range tests {
		dr := dataRetriever{testCase.address, testCase.dc}
		ann, err := dr.getAnnotations(testCase.uuid)
		assert.Equal(t, testCase.expAnnotations, ann,
			fmt.Sprintf("Expected annotations: %v were not equal with received ones: %v \n", testCase.expAnnotations, ann))
		if testCase.expError == nil {
			assert.Equal(t, nil, err)
		} else {
			assert.Contains(t, err.Error(), testCase.expError.Error())
		}
	}
}

func TestGetContent(t *testing.T) {
	tests := []struct {
		uuid       string
		address    utils.ApiURL
		dc         utils.Client
		expContent ContentModel
		expError   error
	}{
		{
			"some_uuid",
			utils.ApiURL{"some_host", "some_endpoint"},
			dummyClient{
				statusCode: http.StatusNotFound,
			},
			nil,
			nil,
		},
		{
			"some_uuid",
			utils.ApiURL{"some_host", "some_endpoint"},
			dummyClient{
				err: errors.New("some error"),
			},
			nil,
			errors.New("some error"),
		},
		{
			"some_uuid",
			utils.ApiURL{"some_host", "some_endpoint"},
			dummyClient{
				statusCode: http.StatusOK,
				body:       "text that can't be unmarshalled",
			},
			nil,
			errors.New("could not unmarshall content with uuid=some_uuid"),
		},
		{
			"622de808-3a7a-49bd-a7fb-2a33f64695be",
			utils.ApiURL{"some_host", "some_endpoint"},
			dummyClient{
				statusCode: http.StatusOK,
				body:       `{"uuid":"622de808-3a7a-49bd-a7fb-2a33f64695be","title":"Title","alternativeTitles":{"promotionalTitle":"Alternative title"},"type":null,"byline":"FT Reporters","brands":[{"id":"http://api.ft.com/things/40f636a3-5507-4311-9629-95376007cb7b"}],"identifiers":[{"authority":"FTCOM-METHODE_identifier","identifierValue":"53217c65-ecef-426e-a3ac-3787e2e62e87"}],"publishedDate":"2017-04-10T08:03:58.000Z","standfirst":"A simple line with an article summary","body":"<body>something relevant here<\/body>","description":null,"mediaType":null,"pixelWidth":null,"pixelHeight":null,"internalBinaryUrl":null,"externalBinaryUrl":null,"members":null,"mainImage":"2934de46-5240-4c7d-8576-f12ae12e4a37","standout":{"editorsChoice":false,"exclusive":false,"scoop":false},"comments":{"enabled":true},"copyright":null,"webUrl":null,"publishReference":"tid_unique_reference","lastModified":"2017-04-10T08:09:01.808Z","canBeSyndicated":"yes","firstPublishedDate":"2017-04-10T08:03:58.000Z","accessLevel":"subscribed","canBeDistributed":"yes"}`,
			},
			ContentModel{
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
			nil,
		},
	}

	for _, testCase := range tests {
		dr := dataRetriever{testCase.address, testCase.dc}
		c, err := dr.getContent(testCase.uuid)

		assert.True(t, reflect.DeepEqual(testCase.expContent, c), fmt.Sprintf("Expected content: %v was not equal with received content: %v \n", testCase.expContent, c))
		if testCase.expError == nil {
			assert.Equal(t, nil, err)
		} else {
			assert.True(t,
				strings.Contains(
					err.Error(),
					testCase.expError.Error()),
				fmt.Sprintf("'%s' does not contains '%s'", err.Error(), testCase.expError.Error()),
			)
		}
	}
}

type dummyClient struct {
	statusCode int
	body       string
	err        error
}

func (c dummyClient) Do(req *http.Request) (*http.Response, error) {

	resp := &http.Response{
		StatusCode: c.statusCode,
		Body:       ioutil.NopCloser(strings.NewReader(c.body)),
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

type DummyMetadataRetriever struct {
	ann []Annotation
	err error
}

func (r DummyMetadataRetriever) getAnnotations(uuid string) ([]Annotation, error) {
	return r.ann, r.err
}
