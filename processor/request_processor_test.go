package processor

import (
	"fmt"
	"testing"

	"github.com/Financial-Times/message-queue-go-producer/producer"
	"github.com/stretchr/testify/assert"
)

func TestForceMessageWithTID(t *testing.T) {
	allowedContentTypes := []string{"Article", "Video"}
	testUUID := "some_uuid"
	dummyDataCombiner := DummyDataCombiner{
		t:            t,
		expectedUUID: testUUID,
		data: CombinedModel{
			UUID:            testUUID,
			Content:         ContentModel{"uuid": testUUID, "title": "simple title", "type": "Article"},
			InternalContent: ContentModel{"uuid": testUUID, "title": "simple title", "type": "Article"},
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
		}}
	tid := "transaction_id_1"
	expMsg := producer.Message{
		Headers: map[string]string{"Message-Type": CombinerMessageType, "X-Request-Id": tid, "Origin-System-Id": CombinerOrigin, "Content-Type": ContentType},
		Body:    `{"uuid":"some_uuid","contentUri":"","lastModified":"","deleted":false,"content":{"uuid":"some_uuid","title":"simple title","type":"Article"},"internalContent":{"uuid":"some_uuid","title":"simple title","type":"Article"},"metadata":[{"thing":{"id":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995","prefLabel":"Barclays","types":["http://base-url/core/Thing","http://base-url/concept/Concept","http://base-url/organisation/Organisation","http://base-url/company/Company","http://base-url/company/PublicCompany"],"predicate":"http://base-url/about","apiUrl":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995"}}]}`,
	}

	log, hook := testLogger()
	dummyMsgProducer := DummyMsgProducer{t: t, expUUID: testUUID, expTID: tid, expMsg: expMsg}
	p := &RequestProcessor{dataCombiner: dummyDataCombiner, forwarder: NewForwarder(log, dummyMsgProducer, allowedContentTypes)}

	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	err := p.ForceMessagePublish(testUUID, tid)
	assert.NoError(t, err)

	assert.Equal(t, "info", hook.LastEntry().Level.String())
	assert.Contains(t, hook.LastEntry().Message, fmt.Sprintf("%v - Mapped and sent for uuid: %v", expMsg.Headers["X-Request-Id"], dummyDataCombiner.data.UUID))
	assert.Equal(t, 1, len(hook.Entries))
}

func TestForceMessageWithoutTID(t *testing.T) {
	allowedContentTypes := []string{"Article", "Video"}
	testUUID := "some_uuid"
	dummyDataCombiner := DummyDataCombiner{
		t:            t,
		expectedUUID: testUUID,
		data: CombinedModel{
			UUID:            testUUID,
			Content:         ContentModel{"uuid": testUUID, "title": "simple title", "type": "Article"},
			InternalContent: ContentModel{"uuid": testUUID, "title": "simple title", "type": "Article"},
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
		}}

	emptyTID := ""
	expMsg := producer.Message{
		Headers: map[string]string{"Message-Type": CombinerMessageType, "X-Request-Id": "[ignore]", "Origin-System-Id": CombinerOrigin, "Content-Type": ContentType},
		Body:    `{"uuid":"some_uuid","contentUri":"","lastModified":"","deleted":false,"content":{"uuid":"some_uuid","title":"simple title","type":"Article"},"internalContent":{"uuid":"some_uuid","title":"simple title","type":"Article"},"metadata":[{"thing":{"id":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995","prefLabel":"Barclays","types":["http://base-url/core/Thing","http://base-url/concept/Concept","http://base-url/organisation/Organisation","http://base-url/company/Company","http://base-url/company/PublicCompany"],"predicate":"http://base-url/about","apiUrl":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995"}}]}`,
	}

	log, hook := testLogger()
	dummyMsgProducer := DummyMsgProducer{t: t, expUUID: testUUID, expMsg: expMsg}
	p := &RequestProcessor{dataCombiner: dummyDataCombiner, forwarder: NewForwarder(log, dummyMsgProducer, allowedContentTypes), log: log}

	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	err := p.ForceMessagePublish(testUUID, emptyTID)
	assert.NoError(t, err)

	assert.Equal(t, "info", hook.LastEntry().Level.String())
	assert.Contains(t, hook.LastEntry().Message, fmt.Sprintf("%v - Mapped and sent for uuid: %v", expMsg.Headers["X-Request-Id"], dummyDataCombiner.data.UUID))
	assert.Equal(t, 2, len(hook.Entries))
}

func TestForceMessageCombinerError(t *testing.T) {
	allowedContentTypes := []string{"Article", "Video"}
	combiner := DummyDataCombiner{t: t, err: fmt.Errorf("some error")}
	expMsg := producer.Message{
		Headers: map[string]string{"Message-Type": CombinerMessageType, "X-Request-Id": "[ignore]", "Origin-System-Id": CombinerOrigin, "Content-Type": ContentType},
		Body:    `{"uuid":"some_uuid","contentUri":"","lastModified":"","deleted":false,"content":{"uuid":"some_uuid","title":"simple title","type":"Article"},"metadata":[{"thing":{"id":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995","prefLabel":"Barclays","types":["http://base-url/core/Thing","http://base-url/concept/Concept","http://base-url/organisation/Organisation","http://base-url/company/Company","http://base-url/company/PublicCompany"],"predicate":"http://base-url/about","apiUrl":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995"}}]}`,
	}

	log, hook := testLogger()
	dummyMsgProducer := DummyMsgProducer{t: t, expUUID: combiner.data.UUID, expMsg: expMsg}
	p := &RequestProcessor{dataCombiner: combiner, forwarder: NewForwarder(log, dummyMsgProducer, allowedContentTypes), log: log}

	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	err := p.ForceMessagePublish(combiner.data.UUID, "")
	assert.Equal(t, combiner.err, err)

	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Contains(t, hook.LastEntry().Message, "Error obtaining the combined message, it will be skipped.")
	assert.Equal(t, hook.LastEntry().Data["error"].(error).Error(), "some error")
	assert.Equal(t, 2, len(hook.Entries))
}

func TestForceMessageNotFoundError(t *testing.T) {
	allowedContentTypes := []string{"Article", "Video"}
	expMsg := producer.Message{
		Headers: map[string]string{"Message-Type": CombinerMessageType, "X-Request-Id": "[ignore]", "Origin-System-Id": CombinerOrigin, "Content-Type": ContentType},
		Body:    `{"uuid":"some_uuid","contentUri":"","lastModified":"","deleted":false,"content":{"uuid":"some_uuid","title":"simple title","type":"Article"},"metadata":[{"thing":{"id":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995","prefLabel":"Barclays","types":["http://base-url/core/Thing","http://base-url/concept/Concept","http://base-url/organisation/Organisation","http://base-url/company/Company","http://base-url/company/PublicCompany"],"predicate":"http://base-url/about","apiUrl":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995"}}]}`,
	}
	testUUID := "some_uuid"
	combiner := DummyDataCombiner{t: t, expectedUUID: testUUID}

	log, hook := testLogger()
	dummyMsgProducer := DummyMsgProducer{t: t, expUUID: testUUID, expMsg: expMsg}
	p := &RequestProcessor{dataCombiner: combiner, forwarder: NewForwarder(log, dummyMsgProducer, allowedContentTypes), log: log}

	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	err := p.ForceMessagePublish(testUUID, "")
	assert.Equal(t, NotFoundError, err)

	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Contains(t, hook.LastEntry().Message, fmt.Sprintf("Could not find content with uuid %s.", testUUID))
	assert.Equal(t, hook.LastEntry().Data["error"].(error).Error(), "content not found")
	assert.Equal(t, 2, len(hook.Entries))
}

func TestForceMessage_FilteringError(t *testing.T) {
	allowedContentTypes := []string{"Article", "Video"}
	testUUID := "80fb3e57-8d3b-4f07-bbb6-8788452d63cb"
	combiner := DummyDataCombiner{
		t:            t,
		expectedUUID: testUUID,
		data: CombinedModel{
			UUID: testUUID,
			// Content Placeholders - marked with Content type - shouldn't get into the Combined queue
			Content: ContentModel{"uuid": testUUID, "title": "simple title", "type": "Content"},
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
		}}
	expMsg := producer.Message{
		Headers: map[string]string{"Message-Type": CombinerMessageType, "X-Request-Id": "[ignore]", "Origin-System-Id": CombinerOrigin, "Content-Type": ContentType},
		Body:    `{"uuid":"some_uuid","contentUri":"","lastModified":"","deleted":false,"content":{"uuid":"some_uuid","title":"simple title","type":"Article"},"metadata":[{"thing":{"id":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995","prefLabel":"Barclays","types":["http://base-url/core/Thing","http://base-url/concept/Concept","http://base-url/organisation/Organisation","http://base-url/company/Company","http://base-url/company/PublicCompany"],"predicate":"http://base-url/about","apiUrl":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995"}}]}`,
	}

	log, hook := testLogger()
	dummyMsgProducer := DummyMsgProducer{t: t, expUUID: testUUID, expMsg: expMsg}
	p := &RequestProcessor{dataCombiner: combiner, forwarder: NewForwarder(log, dummyMsgProducer, allowedContentTypes), log: log}

	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	err := p.ForceMessagePublish(testUUID, "")
	assert.Equal(t, InvalidContentTypeError, err)

	assert.Equal(t, "info", hook.LastEntry().Level.String())
	assert.Contains(t, hook.LastEntry().Message, "Skipped unsupported content with type: Content")
	assert.Equal(t, 2, len(hook.Entries))
}

func TestForceMessageProducerError(t *testing.T) {
	allowedContentTypes := []string{"Article", "Video"}
	testUUID := "some_uuid"
	dummyDataCombiner := DummyDataCombiner{
		t:            t,
		expectedUUID: testUUID,
		data: CombinedModel{
			UUID:    testUUID,
			Content: ContentModel{"uuid": testUUID, "title": "simple title", "type": "Article"},
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
		}}

	log, hook := testLogger()
	dummyMsgProducer := DummyMsgProducer{t: t, expError: fmt.Errorf("some error")}
	p := &RequestProcessor{dataCombiner: dummyDataCombiner, forwarder: NewForwarder(log, dummyMsgProducer, allowedContentTypes), log: log}

	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	err := p.ForceMessagePublish(testUUID, "")
	assert.Equal(t, dummyMsgProducer.expError, err)

	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Contains(t, hook.LastEntry().Message, "Error sending transformed message to queue.")
	assert.Equal(t, hook.LastEntry().Data["error"].(error).Error(), "some error")
	assert.Equal(t, 2, len(hook.Entries))
}
