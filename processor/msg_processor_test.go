package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/v4"
	"github.com/open-policy-agent/opa/rego"
	hooks "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() (*logger.UPPLogger, *hooks.Hook) {
	log := logger.NewUPPLogger("TEST", "INFO")
	log.Out = io.Discard
	hook := hooks.NewLocal(log.Logger)
	return log, hook
}

func TestMsgProcessor_ProcessMessages_Stays_Open_While_Channel_Is_Open(t *testing.T) {
	ch := make(chan *kafka.FTMessage)
	processor := MsgProcessor{src: ch}

	timeout := time.After(3 * time.Second)
	done := make(chan bool)

	go func() {
		processor.ProcessMessages()
		done <- true
	}()

	select {
	case <-done:
		assert.Fail(t, "Goroutine closed")
	case <-timeout:
	}
}

func TestMsgProcessor_ProcessMessages_Closes_When_Channel_Closes(t *testing.T) {
	ch := make(chan *kafka.FTMessage)
	processor := MsgProcessor{src: ch}

	timeout := time.After(3 * time.Second)
	done := make(chan bool)

	go func() {
		processor.ProcessMessages()
		done <- true
	}()
	close(ch)

	select {
	case <-done:
	case <-timeout:
		assert.Fail(t, "Failed to close goroutine")
	}
}

func TestProcessContentMsg_Unmarshal_Error(t *testing.T) {
	m := kafka.FTMessage{
		Headers: map[string]string{"X-Request-Id": "some-tid1"},
		Body:    `body`,
	}

	log, hook := testLogger()
	p := &MsgProcessor{log: log}

	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processContentMsg(m)

	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Equal(t, "some-tid1", hook.LastEntry().Data["transaction_id"])
	assert.Equal(t, "Could not unmarshal message", hook.LastEntry().Message)
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessContentMsg_UnSupportedContent(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1"}, "./testData/content-with-unsupported-uri.json")
	require.NoError(t, err)

	allowedUris := []string{"next-video-mapper", "upp-content-validator"}
	config := MsgProcessorConfig{SupportedContentURIs: allowedUris}

	log, hook := testLogger()
	p := &MsgProcessor{config: config, log: log}

	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processContentMsg(m)

	assert.Equal(t, "info", hook.LastEntry().Level.String())
	assert.Equal(t, "some-tid1", hook.LastEntry().Data["transaction_id"])
	assert.Equal(t, "http://unsupported-content-uri/content/0cef259d-030d-497d-b4ef-e8fa0ee6db6b", hook.LastEntry().Data["contentUri"])
	assert.Equal(t, "Skipped content with unsupported contentUri", hook.LastEntry().Message)
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessContentMsg_SupportedContent_EmptyUUID(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1"}, "./testData/content-without-uuid.json")
	require.NoError(t, err)

	allowedUris := []string{"next-video-mapper", "upp-content-validator"}
	config := MsgProcessorConfig{SupportedContentURIs: allowedUris}

	log, hook := testLogger()
	defaultEvalQuery, err := rego.New(
		rego.Query("data.specialContent.msg"),
		rego.Load([]string{"../opa_modules/special_content.rego"}, nil),
	).PrepareForEval(context.TODO())
	assert.NoError(t, err)
	evaluator := &Evaluator{evalQuery: &defaultEvalQuery}

	p := &MsgProcessor{config: config, log: log, evaluator: evaluator}

	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processContentMsg(m)

	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Equal(t, "some-tid1", hook.LastEntry().Data["transaction_id"])
	assert.Equal(t, "http://next-video-mapper.svc.ft.com/video/model/0cef259d-030d-497d-b4ef-e8fa0ee6db6b", hook.LastEntry().Data["contentUri"])
	assert.Equal(t, "Content UUID was not found. Message will be skipped.", hook.LastEntry().Message)
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessContentMsg_Combiner_Errors(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1"}, "./testData/content-null-type.json")
	require.NoError(t, err)

	cm := &ContentMessage{}
	require.NoError(t, json.Unmarshal([]byte(m.Body), cm))

	allowedUris := []string{"next-video-mapper", "upp-content-validator"}
	config := MsgProcessorConfig{SupportedContentURIs: allowedUris}
	dummyDataCombiner := DummyDataCombiner{
		t:               t,
		expectedContent: cm.ContentModel,
		err:             fmt.Errorf("some error"),
	}

	log, hook := testLogger()
	defaultEvalQuery, err := rego.New(
		rego.Query("data.specialContent.msg"),
		rego.Load([]string{"../opa_modules/special_content.rego"}, nil),
	).PrepareForEval(context.TODO())
	assert.NoError(t, err)
	evaluator := &Evaluator{evalQuery: &defaultEvalQuery}

	p := &MsgProcessor{config: config, dataCombiner: dummyDataCombiner, log: log, evaluator: evaluator}

	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processContentMsg(m)

	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Equal(t, "some-tid1", hook.LastEntry().Data["transaction_id"])
	assert.Equal(t, "Error obtaining the combined message. Metadata could not be read. Message will be skipped.", hook.LastEntry().Message)
	assert.Equal(t, dummyDataCombiner.err, hook.LastEntry().Data["error"])
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessContentMsg_Forwarder_Errors(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1"}, "./testData/content.json")
	require.NoError(t, err)

	cm := &ContentMessage{}
	require.NoError(t, json.Unmarshal([]byte(m.Body), cm))

	allowedUris := []string{"next-video-mapper", "upp-content-validator"}
	allowedContentTypes := []string{"Article", "Video"}
	config := MsgProcessorConfig{SupportedContentURIs: allowedUris}
	dummyDataCombiner := DummyDataCombiner{
		t:               t,
		expectedContent: cm.ContentModel,
		data: CombinedModel{
			UUID:    "0cef259d-030d-497d-b4ef-e8fa0ee6db6b",
			Content: cm.ContentModel,
		},
	}
	dummyMsgProducer := DummyProducer{t: t, expError: fmt.Errorf("some producer error")}

	log, hook := testLogger()
	defaultEvalQuery, err := rego.New(
		rego.Query("data.specialContent.msg"),
		rego.Load([]string{"../opa_modules/special_content.rego"}, nil),
	).PrepareForEval(context.TODO())
	assert.NoError(t, err)
	evaluator := &Evaluator{evalQuery: &defaultEvalQuery}

	p := &MsgProcessor{
		config:       config,
		dataCombiner: dummyDataCombiner,
		forwarder:    newForwarder(dummyMsgProducer, allowedContentTypes),
		log:          log,
		evaluator:    evaluator,
	}

	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processContentMsg(m)

	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Equal(t, "some-tid1", hook.LastEntry().Data["transaction_id"])
	assert.Equal(t, "Failed to forward message to Kafka", hook.LastEntry().Message)
	assert.ErrorIs(t, hook.LastEntry().Data["error"].(error), dummyMsgProducer.expError)
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessContentMsg_Successfully_Forwarded(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1"}, "./testData/content.json")
	require.NoError(t, err)

	cm := &ContentMessage{}
	require.NoError(t, json.Unmarshal([]byte(m.Body), cm))

	allowedUris := []string{"next-video-mapper", "upp-content-validator"}
	allowedContentTypes := []string{"Article", "Video"}
	config := MsgProcessorConfig{SupportedContentURIs: allowedUris}
	dummyDataCombiner := DummyDataCombiner{
		t:               t,
		expectedContent: cm.ContentModel,
		data: CombinedModel{
			UUID:         "0cef259d-030d-497d-b4ef-e8fa0ee6db6b",
			Deleted:      false,
			LastModified: "2017-03-30T13:09:06.48Z",
			ContentURI:   "http://upp-content-validator.svc.ft.com/content/0cef259d-030d-497d-b4ef-e8fa0ee6db6b",
			Content: ContentModel{
				"uuid":  "0cef259d-030d-497d-b4ef-e8fa0ee6db6b",
				"title": "simple title",
				"type":  "Article",
			},
		},
	}

	expMsg := kafka.FTMessage{
		Headers: m.Headers,
		Body:    `{"uuid":"0cef259d-030d-497d-b4ef-e8fa0ee6db6b","content":{"title":"simple title","type":"Article","uuid":"0cef259d-030d-497d-b4ef-e8fa0ee6db6b"},"internalContent":null,"metadata":null,"contentUri":"http://upp-content-validator.svc.ft.com/content/0cef259d-030d-497d-b4ef-e8fa0ee6db6b","lastModified":"2017-03-30T13:09:06.48Z","deleted":false}`,
	}

	dummyMsgProducer := DummyProducer{
		t:       t,
		expTID:  "some-tid1",
		expUUID: dummyDataCombiner.data.UUID,
		expMsg:  expMsg,
	}

	log, hook := testLogger()
	defaultEvalQuery, err := rego.New(
		rego.Query("data.specialContent.msg"),
		rego.Load([]string{"../opa_modules/special_content.rego"}, nil),
	).PrepareForEval(context.TODO())
	assert.NoError(t, err)
	evaluator := &Evaluator{evalQuery: &defaultEvalQuery}

	p := &MsgProcessor{
		config:       config,
		dataCombiner: dummyDataCombiner,
		forwarder:    newForwarder(dummyMsgProducer, allowedContentTypes),
		log:          log,
		evaluator:    evaluator,
	}

	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processContentMsg(m)

	assert.Equal(t, "info", hook.LastEntry().Level.String())
	assert.Equal(t, "some-tid1", hook.LastEntry().Data["transaction_id"])
	assert.Equal(t, "Message successfully forwarded", hook.LastEntry().Message)
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessContentMsg_DeleteEvent_Successfully_Forwarded(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1"}, "./testData/content-delete.json")
	require.NoError(t, err)

	allowedUris := []string{"next-video-mapper", "upp-content-validator"}
	allowedContentTypes := []string{"Article", "Video"}
	config := MsgProcessorConfig{SupportedContentURIs: allowedUris}
	dummyDataCombiner := DummyDataCombiner{
		t: t,
		data: CombinedModel{
			UUID:         "0cef259d-030d-497d-b4ef-e8fa0ee6db6b",
			ContentURI:   "http://next-video-mapper.svc.ft.com/video/model/0cef259d-030d-497d-b4ef-e8fa0ee6db6b",
			Deleted:      true,
			LastModified: "2017-03-30T13:09:06.48Z",
		}}

	expMsg := kafka.FTMessage{
		Headers: m.Headers,
		Body:    `{"uuid":"0cef259d-030d-497d-b4ef-e8fa0ee6db6b","contentUri":"http://next-video-mapper.svc.ft.com/video/model/0cef259d-030d-497d-b4ef-e8fa0ee6db6b","deleted":true,"lastModified":"2017-03-30T13:09:06.48Z","content":null,"internalContent":null,"metadata":null}`,
	}

	dummyMsgProducer := DummyProducer{
		t:       t,
		expTID:  "some-tid1",
		expUUID: dummyDataCombiner.data.UUID,
		expMsg:  expMsg,
	}

	log, hook := testLogger()
	defaultEvalQuery, err := rego.New(
		rego.Query("data.specialContent.msg"),
		rego.Load([]string{"../opa_modules/special_content.rego"}, nil),
	).PrepareForEval(context.TODO())
	assert.NoError(t, err)
	evaluator := &Evaluator{evalQuery: &defaultEvalQuery}

	p := &MsgProcessor{
		config:       config,
		dataCombiner: dummyDataCombiner,
		forwarder:    newForwarder(dummyMsgProducer, allowedContentTypes),
		log:          log,
		evaluator:    evaluator,
	}

	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processContentMsg(m)

	assert.Equal(t, "info", hook.LastEntry().Level.String())
	assert.Equal(t, "some-tid1", hook.LastEntry().Data["transaction_id"])
	assert.Equal(t, "0cef259d-030d-497d-b4ef-e8fa0ee6db6b", hook.LastEntry().Data["uuid"])
	assert.Equal(t, "Message successfully forwarded", hook.LastEntry().Message)
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessMetadataMsg_UnSupportedOrigins(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1", "Origin-System-Id": "origin"}, "./testData/annotations.json")
	require.NoError(t, err)

	allowedOrigins := []string{"http://cmdb.ft.com/systems/binding-service"}
	config := MsgProcessorConfig{SupportedHeaders: allowedOrigins}

	log, hook := testLogger()
	p := &MsgProcessor{config: config, log: log}

	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processMetadataMsg(m)

	assert.Equal(t, "info", hook.LastEntry().Level.String())
	assert.Equal(t, "origin", hook.LastEntry().Data["originSystem"])
	assert.Equal(t, "some-tid1", hook.LastEntry().Data["transaction_id"])
	assert.Equal(t, "Skipped annotations with unsupported Origin-System-Id", hook.LastEntry().Message)
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessMetadataMsg_SupportedOrigin_Unmarshal_Error(t *testing.T) {
	m := kafka.FTMessage{
		Headers: map[string]string{"X-Request-Id": "some-tid1", "Origin-System-Id": "http://cmdb.ft.com/systems/binding-service"},
		Body:    `some body`,
	}

	allowedOrigins := []string{"http://cmdb.ft.com/systems/binding-service"}
	config := MsgProcessorConfig{SupportedHeaders: allowedOrigins}

	log, hook := testLogger()
	p := &MsgProcessor{config: config, log: log}

	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processMetadataMsg(m)

	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Equal(t, "some-tid1", hook.LastEntry().Data["transaction_id"])
	assert.Equal(t, "Could not unmarshal message", hook.LastEntry().Message)
	assert.EqualError(t, hook.LastEntry().Data["error"].(error), "invalid character 's' looking for beginning of value")
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessMetadataMsg_Combiner_Errors(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1", "Origin-System-Id": "http://cmdb.ft.com/systems/binding-service"}, "./testData/annotations.json")
	require.NoError(t, err)

	am := &AnnotationsMessage{}
	require.NoError(t, json.Unmarshal([]byte(m.Body), am))

	allowedOrigins := []string{"http://cmdb.ft.com/systems/binding-service"}
	config := MsgProcessorConfig{SupportedHeaders: allowedOrigins}
	dummyDataCombiner := DummyDataCombiner{
		t:                t,
		expectedMetadata: *am,
		err:              fmt.Errorf("some error"),
	}

	log, hook := testLogger()
	p := &MsgProcessor{config: config, dataCombiner: dummyDataCombiner, log: log}

	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processMetadataMsg(m)

	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Equal(t, "some-tid1", hook.LastEntry().Data["transaction_id"])
	assert.Equal(t, "Error obtaining the combined message. Content couldn't get read. Message will be skipped.", hook.LastEntry().Message)
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessMetadataMsg_Forwarder_Errors(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1", "Origin-System-Id": "http://cmdb.ft.com/systems/binding-service"}, "./testData/annotations.json")
	require.NoError(t, err)

	am := &AnnotationsMessage{}
	require.NoError(t, json.Unmarshal([]byte(m.Body), am))

	allowedOrigins := []string{"http://cmdb.ft.com/systems/binding-service"}
	allowedContentTypes := []string{"Article", "Video", ""}
	config := MsgProcessorConfig{SupportedHeaders: allowedOrigins}
	dummyDataCombiner := DummyDataCombiner{t: t, expectedMetadata: *am, data: CombinedModel{
		UUID:    "some_uuid",
		Content: ContentModel{"uuid": "some_uuid", "title": "simple title", "type": "Article"},
	}}
	dummyMsgProducer := DummyProducer{t: t, expError: fmt.Errorf("some producer error")}

	log, hook := testLogger()
	p := &MsgProcessor{config: config, dataCombiner: dummyDataCombiner, forwarder: newForwarder(dummyMsgProducer, allowedContentTypes), log: log}

	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processMetadataMsg(m)

	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Equal(t, "some-tid1", hook.LastEntry().Data["transaction_id"])
	assert.Equal(t, "some_uuid", hook.LastEntry().Data["uuid"])
	assert.Equal(t, "Failed to forward message to Kafka", hook.LastEntry().Message)
	assert.ErrorIs(t, hook.LastEntry().Data["error"].(error), dummyMsgProducer.expError)
	assert.Equal(t, 1, len(hook.Entries))
}
func TestProcessMetadataMsg_Forward_Skipped(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1", "Origin-System-Id": "http://cmdb.ft.com/systems/binding-service"}, "./testData/annotations.json")
	require.NoError(t, err)

	am := &AnnotationsMessage{}
	require.NoError(t, json.Unmarshal([]byte(m.Body), am))

	allowedOrigins := []string{"http://cmdb.ft.com/systems/binding-service"}
	allowedContentTypes := []string{"Article", "Video", ""}
	config := MsgProcessorConfig{SupportedHeaders: allowedOrigins}
	dummyDataCombiner := DummyDataCombiner{t: t, expectedMetadata: *am, data: CombinedModel{UUID: "some_uuid"}}
	// The producer should return an error so that the test won't pass if the message forward is attempted
	dummyMsgProducer := DummyProducer{t: t, expError: fmt.Errorf("some producer error")}

	log, hook := testLogger()
	p := &MsgProcessor{config: config, dataCombiner: dummyDataCombiner, forwarder: newForwarder(dummyMsgProducer, allowedContentTypes), log: log}

	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processMetadataMsg(m)

	assert.Equal(t, "warning", hook.LastEntry().Level.String())
	assert.Equal(t, "some-tid1", hook.LastEntry().Data["transaction_id"])
	assert.Equal(t, "Skipped. Could not find content when processing an annotations publish event.", hook.LastEntry().Message)
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessMetadataMsg_Successfully_Forwarded(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1", "Origin-System-Id": "http://cmdb.ft.com/systems/binding-service"}, "./testData/annotations.json")
	require.NoError(t, err)

	am := &AnnotationsMessage{}
	require.NoError(t, json.Unmarshal([]byte(m.Body), am))

	allowedOrigins := []string{"http://cmdb.ft.com/systems/binding-service"}
	allowedContentTypes := []string{"Article", "Video"}
	config := MsgProcessorConfig{SupportedHeaders: allowedOrigins}

	dummyDataCombiner := DummyDataCombiner{
		t:                t,
		expectedMetadata: *am,
		data: CombinedModel{
			UUID:            "some_uuid",
			Content:         ContentModel{"uuid": "some_uuid", "title": "simple title", "type": "Article"},
			InternalContent: ContentModel{"uuid": "some_uuid", "title": "simple title", "type": "Article"},
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
	expMsg := kafka.FTMessage{
		Headers: m.Headers,
		Body:    `{"uuid":"some_uuid","contentUri":"","lastModified":"","deleted":false,"content":{"uuid":"some_uuid","title":"simple title","type":"Article"},"internalContent":{"uuid":"some_uuid","title":"simple title","type":"Article"},"metadata":[{"thing":{"id":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995","prefLabel":"Barclays","types":["http://base-url/core/Thing","http://base-url/concept/Concept","http://base-url/organisation/Organisation","http://base-url/company/Company","http://base-url/company/PublicCompany"],"predicate":"http://base-url/about","apiUrl":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995"}}]}`,
	}

	dummyMsgProducer := DummyProducer{
		t:       t,
		expTID:  "some-tid1",
		expUUID: dummyDataCombiner.data.UUID,
		expMsg:  expMsg,
	}

	log, hook := testLogger()
	p := &MsgProcessor{
		config:       config,
		dataCombiner: dummyDataCombiner,
		forwarder:    newForwarder(dummyMsgProducer, allowedContentTypes),
		log:          log,
	}

	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processMetadataMsg(m)

	assert.Equal(t, "info", hook.LastEntry().Level.String())
	assert.Equal(t, "some-tid1", hook.LastEntry().Data["transaction_id"])
	assert.Equal(t, "some_uuid", hook.LastEntry().Data["uuid"])
	assert.Equal(t, "Message successfully forwarded", hook.LastEntry().Message)
	assert.Equal(t, 1, len(hook.Entries))
}

func TestForwardMsg(t *testing.T) {
	tests := []struct {
		headers map[string]string
		uuid    string
		body    string
		err     error
	}{
		{
			headers: map[string]string{
				"X-Request-Id": "some-tid1",
			},
			uuid: "uuid1",
			body: `{"uuid":"uuid1","content":{"uuid":"","title":"","body":"","identifiers":null,"publishedDate":"","lastModified":"","firstPublishedDate":"","mediaType":"","byline":"","standfirst":"","description":"","mainImage":"","publishReference":"","type":""},"internalContent":null,"metadata":null,"contentUri":"","lastModified":"","deleted":false}`,
			err:  nil,
		},
		{
			headers: map[string]string{
				"X-Request-Id": "some-tid2",
			},
			uuid: "uuid-returning-error",
			body: `{"uuid":"uuid-returning-error","content":{"uuid":"","title":"","body":"","identifiers":null,"publishedDate":"","lastModified":"","firstPublishedDate":"","mediaType":"","byline":"","standfirst":"","description":"","mainImage":"","publishReference":"","type":""},"metadata":null}`,
			err:  fmt.Errorf("some error"),
		},
	}

	for _, testCase := range tests {
		var model CombinedModel
		require.NoError(t, json.Unmarshal([]byte(testCase.body), &model))

		p := MsgProcessor{
			forwarder: &forwarder{
				producer: DummyProducer{
					t:        t,
					expTID:   testCase.headers["X-Request-Id"],
					expUUID:  testCase.uuid,
					expError: testCase.err,
					expMsg: kafka.FTMessage{
						Headers: testCase.headers,
						Body:    testCase.body,
					},
				},
			},
		}

		err := p.forwarder.forwardMsg(testCase.headers, &model)
		assert.Equal(t, testCase.err, err)
	}
}

func TestExtractTID(t *testing.T) {
	assertion := assert.New(t)

	log, _ := testLogger()
	p := &MsgProcessor{log: log}

	tests := []struct {
		headers     map[string]string
		expectedTID string
	}{
		{
			headers: map[string]string{
				"X-Request-Id": "some-tid1",
			},
			expectedTID: "some-tid1",
		},
		{
			headers: map[string]string{
				"X-Request-Id":      "some-tid2",
				"Some-Other-Header": "some-value",
			},
			expectedTID: "some-tid2",
		},
	}

	for _, testCase := range tests {
		actualTID := p.extractTID(testCase.headers)
		assertion.Equal(testCase.expectedTID, actualTID)
	}
}

func TestExtractTIDForEmptyHeader(t *testing.T) {
	assertion := assert.New(t)

	log, _ := testLogger()
	p := &MsgProcessor{log: log}

	headers := map[string]string{
		"Some-Other-Header": "some-value",
	}

	actualTID := p.extractTID(headers)
	assertion.NotEmpty(actualTID)
	assertion.Contains(actualTID, "tid_")
	assertion.Contains(actualTID, "_post_publication_combiner")
}

func TestSupports(t *testing.T) {
	tests := []struct {
		element   string
		array     []string
		expResult bool
	}{
		{
			element:   "some value",
			array:     []string{"some value", "some other value"},
			expResult: true,
		},
		{
			element:   "some value",
			array:     []string{"some other value"},
			expResult: false,
		},
		{
			element:   "some value",
			array:     []string{},
			expResult: false,
		},
		{
			element:   "",
			array:     []string{"some value", "some other value"},
			expResult: false,
		},
		{
			element:   "",
			array:     []string{},
			expResult: false,
		},
	}

	for _, testCase := range tests {
		result := containsSubstringOf(testCase.array, testCase.element)
		assert.Equal(t, testCase.expResult, result, fmt.Sprintf("Element %v was not found in %v", testCase.array, testCase.element))
	}
}

type DummyProducer struct {
	t        *testing.T
	expUUID  string
	expTID   string
	expMsg   kafka.FTMessage
	expError error
}

func NewDummyProducer(t *testing.T, uuid, tid string, message kafka.FTMessage) DummyProducer {
	return DummyProducer{
		t:       t,
		expUUID: uuid,
		expTID:  tid,
		expMsg:  message,
	}
}

func (p DummyProducer) SendMessage(m kafka.FTMessage) error {
	if p.expError != nil {
		return p.expError
	}
	assert.Equal(p.t, m.Headers["Message-Type"], CombinerMessageType)
	if p.expMsg.Headers["X-Request-Id"] == "[ignore]" {
		p.expMsg.Headers["X-Request-Id"] = m.Headers["X-Request-Id"]
	}

	assert.Equal(p.t, p.expTID, m.Headers["X-Request-Id"])

	assert.True(p.t, reflect.DeepEqual(p.expMsg.Headers, m.Headers), "Expected: %v \nActual: %v", p.expMsg.Headers, m.Headers)
	assert.JSONEq(p.t, p.expMsg.Body, m.Body, "Expected: %v \nActual: %v", p.expMsg.Body, m.Body)

	return nil
}

type DummyDataCombiner struct {
	t                *testing.T
	expectedContent  ContentModel
	expectedMetadata AnnotationsMessage
	expectedUUID     string
	data             CombinedModel
	err              error
}

func (c DummyDataCombiner) GetCombinedModelForContent(content ContentModel) (CombinedModel, error) {
	assert.Equal(c.t, c.expectedContent, content)
	return c.data, c.err
}

func (c DummyDataCombiner) GetCombinedModelForAnnotations(metadata AnnotationsMessage) (CombinedModel, error) {
	assert.Equal(c.t, c.expectedMetadata, metadata)
	return c.data, c.err
}

func (c DummyDataCombiner) GetCombinedModel(uuid string) (CombinedModel, error) {
	assert.Equal(c.t, c.expectedUUID, uuid)
	return c.data, c.err
}

func createMessage(headers map[string]string, fixture string) (kafka.FTMessage, error) {
	f, err := os.Open(fixture)
	if err != nil {
		return kafka.FTMessage{}, err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return kafka.FTMessage{}, err
	}

	return kafka.FTMessage{
		Headers: headers,
		Body:    string(data),
	}, nil
}
