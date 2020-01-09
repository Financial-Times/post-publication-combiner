package processor

import (
	consumer "github.com/Financial-Times/message-queue-gonsumer"
	"github.com/stretchr/testify/assert"
	"testing"
)

type DummyConsumer struct{}

func (DummyConsumer) Start() {
}

func (DummyConsumer) Stop() {
}

func (DummyConsumer) ConnectivityCheck() (string, error) {
	return "", nil
}

func TestQConsumerProcessMsg(t *testing.T) {

	ch := make(chan *KafkaQMessage, 5)

	mType := "someType"
	kqc := KafkaQConsumer{
		Consumer: DummyConsumer{},
		dest:     ch,
		msgType:  mType,
	}

	msgs := []consumer.Message{
		{
			Headers: map[string]string{"X-Request-Id": "some-tid1"},
			Body:    `{"uuid":"uuid1","content":{"uuid":"","title":"","body":"","identifiers":null,"publishedDate":"","lastModified":"","firstPublishedDate":"","mediaType":"","marked_deleted":"","byline":"","standfirst":"","description":"","mainImage":"","publishReference":""},"v1-metadata":null}`,
		},
		{
			Headers: map[string]string{"X-Request-Id": "some-tid2"},
			Body:    `{"uuid":"uuid1","content":{"uuid":"","title":"","body":"","identifiers":null,"publishedDate":"","lastModified":"","firstPublishedDate":"","mediaType":"","marked_deleted":"","byline":"","standfirst":"","description":"","mainImage":"","publishReference":""},"v1-metadata":null}`,
		},
	}

	for _, m := range msgs {
		kqc.ProcessMsg(m)
	}
	close(ch)

	assert.Equal(t, 5, cap(ch))
	assert.Equal(t, len(msgs), len(ch))

	i := 0
	for el := range ch {
		assert.Equal(t, msgs[i], el.msg)
		assert.Equal(t, mType, el.msgType)
		i++
	}
}
