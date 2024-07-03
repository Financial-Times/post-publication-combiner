package processor

import (
	"fmt"
	"testing"

	"github.com/Financial-Times/kafka-client-go/v4"
	"github.com/Financial-Times/post-publication-combiner/v2/policy"
	"github.com/stretchr/testify/assert"
)

func TestRequestProcessor_ForcePublication(t *testing.T) {
	testUUID := "some_uuid"
	testTID := "some_transaction_id"
	testErr := fmt.Errorf("some error")
	allowedContentTypes := []string{"Article", "Video"}

	tests := []struct {
		name            string
		publishTID      string
		dataCombiner    dataCombiner
		messageProducer messageProducer
		err             error
	}{
		{
			name:       "Content with transaction ID is published",
			publishTID: testTID,
			dataCombiner: DummyDataCombiner{
				t:            t,
				expectedUUID: testUUID,
				data: CombinedModel{
					UUID: testUUID,
					Content: ContentModel{
						"uuid":  testUUID,
						"title": "simple title",
						"type":  "Article",
					},
					InternalContent: ContentModel{
						"uuid":  testUUID,
						"title": "simple title",
						"type":  "Article",
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
				}},
			messageProducer: NewDummyProducer(t, testUUID, testTID, kafka.FTMessage{
				Headers: map[string]string{
					"Message-Type":     CombinerMessageType,
					"X-Request-Id":     testTID,
					"Origin-System-Id": CombinerOrigin,
					"Content-Type":     ContentType,
				},
				Body: `{"uuid":"some_uuid","contentUri":"","lastModified":"","deleted":false,
							"content":{"uuid":"some_uuid","title":"simple title","type":"Article"},
							"internalContent":{"uuid":"some_uuid","title":"simple title","type":"Article"},
							"metadata":[{"thing":{"id":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
							"prefLabel":"Barclays","types":[
							"http://base-url/core/Thing",
							"http://base-url/concept/Concept",
							"http://base-url/organisation/Organisation",
							"http://base-url/company/Company",
							"http://base-url/company/PublicCompany"],
							"predicate":"http://base-url/about",
							"apiUrl":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995"}}]}`,
			}),
		},
		{
			name:       "Content without transaction ID is published",
			publishTID: "",
			dataCombiner: DummyDataCombiner{
				t:            t,
				expectedUUID: testUUID,
				data: CombinedModel{
					UUID: testUUID,
					Content: ContentModel{
						"uuid":  testUUID,
						"title": "simple title",
						"type":  "Article",
					},
					InternalContent: ContentModel{
						"uuid":  testUUID,
						"title": "simple title",
						"type":  "Article",
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
				}},
			messageProducer: NewDummyProducer(t, testUUID, "", kafka.FTMessage{
				Headers: map[string]string{
					"Message-Type":     CombinerMessageType,
					"X-Request-Id":     "[ignore]",
					"Origin-System-Id": CombinerOrigin,
					"Content-Type":     ContentType,
				},
				Body: `{"uuid":"some_uuid","contentUri":"","lastModified":"","deleted":false,
						"content":{"uuid":"some_uuid","title":"simple title","type":"Article"},
						"internalContent":{"uuid":"some_uuid","title":"simple title","type":"Article"},
						"metadata":[{"thing":{"id":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995",
						"prefLabel":"Barclays","types":[
						"http://base-url/core/Thing",
						"http://base-url/concept/Concept",
						"http://base-url/organisation/Organisation",
						"http://base-url/company/Company",
						"http://base-url/company/PublicCompany"],
						"predicate":"http://base-url/about",
						"apiUrl":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995"}}]}`,
			}),
		},
		{
			name: "Content is not successfully fetched",
			dataCombiner: DummyDataCombiner{
				t:            t,
				expectedUUID: testUUID,
				err:          testErr,
			},
			err: testErr,
		},
		{
			name: "Content is not found",
			dataCombiner: DummyDataCombiner{
				t:            t,
				expectedUUID: testUUID,
			},
			err: ErrNotFound,
		},
		{
			name: "Content is filtered out",
			dataCombiner: DummyDataCombiner{
				t:            t,
				expectedUUID: testUUID,
				data: CombinedModel{
					UUID: testUUID,
					Content: ContentModel{
						"uuid": testUUID,
						"type": "Content",
					},
					Metadata: []Annotation{},
				}},
			err: ErrInvalidContentType,
		},
		{
			name: "Content is not forwarded to queue",
			dataCombiner: DummyDataCombiner{
				t:            t,
				expectedUUID: testUUID,
				data: CombinedModel{
					UUID: testUUID,
					Content: ContentModel{
						"uuid": testUUID,
						"type": "Article",
					},
					Metadata: []Annotation{},
				}},
			messageProducer: DummyProducer{
				t:        t,
				expError: testErr,
			},
			err: testErr,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log, hook := testLogger()
			opaAgent := mockOpaAgent{
				returnResult: &policy.ContentPolicyResult{},
			}

			requestProcessor := NewRequestProcessor(test.dataCombiner, test.messageProducer, allowedContentTypes, log, opaAgent)

			err := requestProcessor.ForcePublication(testUUID, test.publishTID)
			assert.ErrorIs(t, err, test.err)
			assert.Nil(t, hook.LastEntry())
		})
	}
}
