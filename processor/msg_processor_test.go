package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	testLogger "github.com/Financial-Times/go-logger/test"
	"github.com/Financial-Times/message-queue-go-producer/producer"
	"github.com/Financial-Times/message-queue-gonsumer/consumer"
	"github.com/stretchr/testify/assert"
)

func TestProcessContentMsg_Unmarshall_Error(t *testing.T) {

	m := consumer.Message{
		Headers: map[string]string{"X-Request-Id": "some-tid1"},
		Body:    `body`,
	}

	hook := testLogger.NewTestHook("combiner")
	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p := &MsgProcessor{}
	p.processContentMsg(m)

	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Contains(t, hook.LastEntry().Message, "Could not unmarshall message with TID=")
	assert.Equal(t, 1, len(hook.Entries))

}

func TestProcessContentMsg_UnSupportedContent(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1"}, "./testData/content-with-unsupported-uri.json")
	assert.NoError(t, err)

	allowedUris := []string{"methode-article-mapper", "wordpress-article-mapper", "next-video-mapper", "upp-content-validator"}
	config := MsgProcessorConfig{SupportedContentURIs: allowedUris}
	p := &MsgProcessor{config: config}

	hook := testLogger.NewTestHook("combiner")
	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processContentMsg(m)

	assert.Equal(t, "info", hook.LastEntry().Level.String())
	assert.Contains(t, hook.LastEntry().Message, fmt.Sprintf("%v - Skipped unsupported content with contentUri: %v.", m.Headers["X-Request-Id"], "http://unsupported-content-uri/content/0cef259d-030d-497d-b4ef-e8fa0ee6db6b"))
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessContentMsg_SupportedContent_EmptyUUID(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1"}, "./testData/content-no-uuid-wordpress-uri.json")
	assert.NoError(t, err)

	allowedUris := []string{"methode-article-mapper", "wordpress-article-mapper", "next-video-mapper", "upp-content-validator"}
	config := MsgProcessorConfig{SupportedContentURIs: allowedUris}
	p := &MsgProcessor{config: config}

	hook := testLogger.NewTestHook("combiner")
	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processContentMsg(m)

	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Contains(t, hook.LastEntry().Message, fmt.Sprintf("UUID not found after message marshalling, skipping message with contentUri=http://wordpress-article-mapper/content/0cef259d-030d-497d-b4ef-e8fa0ee6db6b."))
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessContentMsg_Combiner_Errors(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1"}, "./testData/content-null-type.json")
	assert.NoError(t, err)

	cm := &ContentMessage{}
	err = json.Unmarshal([]byte(m.Body), cm)
	assert.NoError(t, err)

	allowedUris := []string{"methode-article-mapper", "wordpress-article-mapper", "next-video-mapper", "upp-content-validator"}
	config := MsgProcessorConfig{SupportedContentURIs: allowedUris}
	dummyDataCombiner := DummyDataCombiner{
		t:               t,
		expectedContent: cm.ContentModel,
		err:             errors.New("some error"),
	}
	p := &MsgProcessor{config: config, DataCombiner: dummyDataCombiner}

	hook := testLogger.NewTestHook("dummyDataCombiner")
	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processContentMsg(m)

	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Contains(t, hook.LastEntry().Message, fmt.Sprintf("%v - Error obtaining the combined message. Metadata could not be read. Message will be skipped.", m.Headers["X-Request-Id"]))
	assert.Equal(t, hook.LastEntry().Data["error"].(error).Error(), dummyDataCombiner.err.Error())
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessContentMsg_Forwarder_Errors(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1"}, "./testData/content.json")
	assert.NoError(t, err)

	cm := &ContentMessage{}
	err = json.Unmarshal([]byte(m.Body), cm)
	assert.NoError(t, err)

	allowedUris := []string{"methode-article-mapper", "wordpress-article-mapper", "next-video-mapper", "upp-content-validator"}
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
	dummyMsgProducer := DummyMsgProducer{t: t, expError: errors.New("some dummyMsgProducer error")}

	p := &MsgProcessor{config: config, DataCombiner: dummyDataCombiner, Forwarder: NewForwarder(dummyMsgProducer, allowedContentTypes)}

	hook := testLogger.NewTestHook("dummyDataCombiner")
	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processContentMsg(m)

	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Contains(t, hook.LastEntry().Message, fmt.Sprintf("%v - Error sending transformed message to queue.", m.Headers["X-Request-Id"]))
	assert.Equal(t, hook.LastEntry().Data["error"].(error).Error(), dummyMsgProducer.expError.Error())
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessContentMsg_Successfully_Forwarded(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1"}, "./testData/content.json")
	assert.NoError(t, err)

	cm := &ContentMessage{}
	err = json.Unmarshal([]byte(m.Body), cm)
	assert.NoError(t, err)

	allowedUris := []string{"methode-article-mapper", "wordpress-article-mapper", "next-video-mapper", "upp-content-validator"}
	allowedContentTypes := []string{"Article", "Video"}
	config := MsgProcessorConfig{SupportedContentURIs: allowedUris}
	dummyDataCombiner := DummyDataCombiner{
		t:               t,
		expectedContent: cm.ContentModel,
		data: CombinedModel{
			UUID:         "0cef259d-030d-497d-b4ef-e8fa0ee6db6b",
			Deleted:      false,
			LastModified: "2017-03-30T13:09:06.48Z",
			ContentURI:   "http://wordpress-article-mapper/content/0cef259d-030d-497d-b4ef-e8fa0ee6db6b",
			Content: ContentModel{
				"uuid":  "0cef259d-030d-497d-b4ef-e8fa0ee6db6b",
				"title": "simple title",
				"type":  "Article",
			},
		},
	}

	expMsg := producer.Message{
		Headers: m.Headers,
		Body:    `{"uuid":"0cef259d-030d-497d-b4ef-e8fa0ee6db6b","content":{"title":"simple title","type":"Article","uuid":"0cef259d-030d-497d-b4ef-e8fa0ee6db6b"},"metadata":null,"contentUri":"http://wordpress-article-mapper/content/0cef259d-030d-497d-b4ef-e8fa0ee6db6b","lastModified":"2017-03-30T13:09:06.48Z","deleted":false}`,
	}

	dummyMsgProducer := DummyMsgProducer{t: t, expUUID: dummyDataCombiner.data.UUID, expMsg: expMsg}
	p := &MsgProcessor{config: config, DataCombiner: dummyDataCombiner, Forwarder: NewForwarder(dummyMsgProducer, allowedContentTypes)}

	hook := testLogger.NewTestHook("dummyDataCombiner")
	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processContentMsg(m)

	assert.Equal(t, "info", hook.LastEntry().Level.String())
	assert.Contains(t, hook.LastEntry().Message, fmt.Sprintf("%v - Mapped and sent for uuid: %v", m.Headers["X-Request-Id"], dummyDataCombiner.data.UUID))
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessContentMsg_DeleteEvent_Successfully_Forwarded(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1"}, "./testData/content-delete.json")
	assert.NoError(t, err)

	allowedUris := []string{"methode-article-mapper", "wordpress-article-mapper", "next-video-mapper", "upp-content-validator"}
	allowedContentTypes := []string{"Article", "Video"}
	config := MsgProcessorConfig{SupportedContentURIs: allowedUris}
	dummyDataCombiner := DummyDataCombiner{
		t: t,
		data: CombinedModel{
			UUID:         "0cef259d-030d-497d-b4ef-e8fa0ee6db6b",
			ContentURI:   "http://wordpress-article-mapper/content/0cef259d-030d-497d-b4ef-e8fa0ee6db6b",
			Deleted:      true,
			LastModified: "2017-03-30T13:09:06.48Z",
		}}

	expMsg := producer.Message{
		Headers: m.Headers,
		Body:    `{"uuid":"0cef259d-030d-497d-b4ef-e8fa0ee6db6b","contentUri":"http://wordpress-article-mapper/content/0cef259d-030d-497d-b4ef-e8fa0ee6db6b","deleted":true,"lastModified":"2017-03-30T13:09:06.48Z","content":null,"metadata":null}`,
	}

	dummyMsgProducer := DummyMsgProducer{t: t, expUUID: dummyDataCombiner.data.UUID, expMsg: expMsg}
	p := MsgProcessor{config: config, DataCombiner: dummyDataCombiner, Forwarder: NewForwarder(dummyMsgProducer, allowedContentTypes)}

	hook := testLogger.NewTestHook("dummyDataCombiner")
	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processContentMsg(m)

	assert.Equal(t, "info", hook.LastEntry().Level.String())
	assert.Contains(t, hook.LastEntry().Message, fmt.Sprintf("%v - Mapped and sent for uuid: %v", m.Headers["X-Request-Id"], dummyDataCombiner.data.UUID))
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessMetadataMsg_UnSupportedOrigins(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1", "Origin-System-Id": "origin"}, "./testData/annotations.json")
	assert.NoError(t, err)

	allowedOrigins := []string{"http://cmdb.ft.com/systems/binding-service", "http://cmdb.ft.com/systems/methode-web-pub"}
	config := MsgProcessorConfig{SupportedHeaders: allowedOrigins}
	p := &MsgProcessor{config: config}

	hook := testLogger.NewTestHook("combiner")
	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processMetadataMsg(m)

	assert.Equal(t, "info", hook.LastEntry().Level.String())
	assert.Contains(t, hook.LastEntry().Message, fmt.Sprintf("%v - Skipped unsupported annotations with Origin-System-Id: %v. ", m.Headers["X-Request-Id"], m.Headers["Origin-System-Id"]))
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessMetadataMsg_SupportedOrigin_Unmarshall_Error(t *testing.T) {

	m := consumer.Message{
		Headers: map[string]string{"X-Request-Id": "some-tid1", "Origin-System-Id": "http://cmdb.ft.com/systems/binding-service"},
		Body:    `some body`,
	}

	allowedOrigins := []string{"http://cmdb.ft.com/systems/binding-service", "http://cmdb.ft.com/systems/methode-web-pub"}
	config := MsgProcessorConfig{SupportedHeaders: allowedOrigins}
	p := &MsgProcessor{config: config}

	hook := testLogger.NewTestHook("combiner")
	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processMetadataMsg(m)

	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Contains(t, hook.LastEntry().Message, fmt.Sprintf("Could not unmarshall message with TID=%v", m.Headers["X-Request-Id"]))
	assert.Equal(t, hook.LastEntry().Data["error"].(error).Error(), "invalid character 's' looking for beginning of value")
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessMetadataMsg_Combiner_Errors(t *testing.T) {

	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1", "Origin-System-Id": "http://cmdb.ft.com/systems/binding-service"}, "./testData/annotations.json")
	assert.NoError(t, err)

	am := &AnnotationsMessage{}
	err = json.Unmarshal([]byte(m.Body), am)
	assert.NoError(t, err)

	allowedOrigins := []string{"http://cmdb.ft.com/systems/binding-service", "http://cmdb.ft.com/systems/methode-web-pub"}
	config := MsgProcessorConfig{SupportedHeaders: allowedOrigins}
	dummyDataCombiner := DummyDataCombiner{
		t:                t,
		expectedMetadata: *am,
		err:              errors.New("some error"),
	}
	p := &MsgProcessor{config: config, DataCombiner: dummyDataCombiner}

	hook := testLogger.NewTestHook("dummyDataCombiner")
	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processMetadataMsg(m)

	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Contains(t, hook.LastEntry().Message, fmt.Sprintf("%v - Error obtaining the combined message. Content couldn't get read. Message will be skipped.", m.Headers["X-Request-Id"]))
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessMetadataMsg_Forwarder_Errors(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1", "Origin-System-Id": "http://cmdb.ft.com/systems/binding-service"}, "./testData/annotations.json")
	assert.NoError(t, err)

	am := &AnnotationsMessage{}
	err = json.Unmarshal([]byte(m.Body), am)
	assert.NoError(t, err)

	allowedOrigins := []string{"http://cmdb.ft.com/systems/binding-service", "http://cmdb.ft.com/systems/methode-web-pub"}
	allowedContentTypes := []string{"Article", "Video", ""}
	config := MsgProcessorConfig{SupportedHeaders: allowedOrigins}
	dummyDataCombiner := DummyDataCombiner{t: t, expectedMetadata: *am, data: CombinedModel{UUID: "some_uuid"}}
	dummyMsgProducer := DummyMsgProducer{t: t, expError: errors.New("some dummyMsgProducer error")}

	p := &MsgProcessor{config: config, DataCombiner: dummyDataCombiner, Forwarder: NewForwarder(dummyMsgProducer, allowedContentTypes)}

	hook := testLogger.NewTestHook("dummyDataCombiner")
	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processMetadataMsg(m)

	assert.Equal(t, "error", hook.LastEntry().Level.String())
	assert.Contains(t, hook.LastEntry().Message, fmt.Sprintf("%v - Error sending transformed message to queue", m.Headers["X-Request-Id"]))
	assert.Equal(t, hook.LastEntry().Data["error"].(error).Error(), "some dummyMsgProducer error")
	assert.Equal(t, 1, len(hook.Entries))
}

func TestProcessMetadataMsg_Successfully_Forwarded(t *testing.T) {
	m, err := createMessage(map[string]string{"X-Request-Id": "some-tid1", "Origin-System-Id": "http://cmdb.ft.com/systems/binding-service"}, "./testData/annotations.json")
	assert.NoError(t, err)

	am := &AnnotationsMessage{}
	err = json.Unmarshal([]byte(m.Body), am)
	assert.NoError(t, err)

	allowedOrigins := []string{"http://cmdb.ft.com/systems/binding-service", "http://cmdb.ft.com/systems/methode-web-pub"}
	allowedContentTypes := []string{"Article", "Video"}
	config := MsgProcessorConfig{SupportedHeaders: allowedOrigins}

	dummyDataCombiner := DummyDataCombiner{
		t:                t,
		expectedMetadata: *am,
		data: CombinedModel{
			UUID:    "some_uuid",
			Content: ContentModel{"uuid": "some_uuid", "title": "simple title", "type": "Article"},
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
		}}
	expMsg := producer.Message{
		Headers: m.Headers,
		Body:    `{"uuid":"some_uuid","contentUri":"","lastModified":"","deleted":false,"content":{"uuid":"some_uuid","title":"simple title","type":"Article"},"metadata":[{"thing":{"id":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995","prefLabel":"Barclays","types":["http://base-url/core/Thing","http://base-url/concept/Concept","http://base-url/organisation/Organisation","http://base-url/company/Company","http://base-url/company/PublicCompany"],"predicate":"http://base-url/about","apiUrl":"http://base-url/80bec524-8c75-4d0f-92fa-abce3962d995"}}]}`,
	}

	dummyMsgProducer := DummyMsgProducer{t: t, expUUID: dummyDataCombiner.data.UUID, expMsg: expMsg}
	p := &MsgProcessor{config: config, DataCombiner: dummyDataCombiner, Forwarder: NewForwarder(dummyMsgProducer, allowedContentTypes)}

	hook := testLogger.NewTestHook("dummyDataCombiner")
	assert.Nil(t, hook.LastEntry())
	assert.Equal(t, 0, len(hook.Entries))

	p.processMetadataMsg(m)

	assert.Equal(t, "info", hook.LastEntry().Level.String())
	assert.Contains(t, hook.LastEntry().Message, fmt.Sprintf("%v - Mapped and sent for uuid: %v", m.Headers["X-Request-Id"], dummyDataCombiner.data.UUID))
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
			body: `{"uuid":"uuid1","content":{"uuid":"","title":"","body":"","identifiers":null,"publishedDate":"","lastModified":"","firstPublishedDate":"","mediaType":"","byline":"","standfirst":"","description":"","mainImage":"","publishReference":"","type":""},"metadata":null,"contentUri":"","lastModified":"","deleted":false}`,
			err:  nil,
		},
		{
			headers: map[string]string{
				"X-Request-Id": "some-tid2",
			},
			uuid: "uuid-returning-error",
			body: `{"uuid":"uuid-returning-error","content":{"uuid":"","title":"","body":"","identifiers":null,"publishedDate":"","lastModified":"","firstPublishedDate":"","mediaType":"","byline":"","standfirst":"","description":"","mainImage":"","publishReference":"","type":""},"metadata":null}`,
			err:  fmt.Errorf("Some error"),
		},
	}

	for _, testCase := range tests {

		var model CombinedModel
		err := json.Unmarshal([]byte(testCase.body), &model)
		assert.Nil(t, err)

		q := MsgProcessor{
			Forwarder: Forwarder{
				MsgProducer: DummyMsgProducer{
					t:        t,
					expUUID:  testCase.uuid,
					expError: testCase.err,
					expMsg: producer.Message{
						Headers: testCase.headers,
						Body:    testCase.body,
					},
				},
			},
		}

		err = q.Forwarder.forwardMsg(testCase.headers, &model)
		assert.Equal(t, testCase.err, err)
	}
}

func TestExtractTID(t *testing.T) {
	assertion := assert.New(t)
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
		actualTID := extractTID(testCase.headers)
		assertion.Equal(testCase.expectedTID, actualTID)
	}
}

func TestExtractTIDForEmptyHeader(t *testing.T) {
	assertion := assert.New(t)
	headers := map[string]string{
		"Some-Other-Header": "some-value",
	}

	actualTID := extractTID(headers)
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

type DummyMsgProducer struct {
	t        *testing.T
	expUUID  string
	expTID   string
	expMsg   producer.Message
	expError error
}

func (p DummyMsgProducer) SendMessage(uuid string, m producer.Message) error {

	if p.expError != nil {
		return p.expError
	}
	assert.Equal(p.t, m.Headers["Message-Type"], CombinerMessageType)
	assert.Equal(p.t, p.expUUID, uuid)
	if p.expMsg.Headers["X-Request-Id"] == "[ignore]" {
		p.expMsg.Headers["X-Request-Id"] = m.Headers["X-Request-Id"]
	}
	if p.expTID != "" {
		assert.Equal(p.t, p.expTID, m.Headers["X-Request-Id"])
	}

	assert.NotEmpty(p.t, m.Headers["X-Request-Id"])
	assert.True(p.t, reflect.DeepEqual(p.expMsg.Headers, m.Headers), "Expected: %v \nActual: %v", p.expMsg.Headers, m.Headers)
	assert.JSONEq(p.t, p.expMsg.Body, m.Body, "Expected: %v \nActual: %v", p.expMsg.Body, m.Body)

	return nil
}

func (p DummyMsgProducer) ConnectivityCheck() (string, error) {
	return "", nil
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

func createMessage(headers map[string]string, fixture string) (consumer.Message, error) {

	f, err := os.Open(fixture)
	if err != nil {
		return consumer.Message{}, err
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return consumer.Message{}, err
	}

	return consumer.Message{
		Headers: headers,
		Body:    string(data),
	}, nil
}
