package processor

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetMapValueAsString(t *testing.T) {
	assert := assert.New(t)
	tests := []struct {
		key      string
		content  map[string]interface{}
		expValue string
	}{
		{"uuid", map[string]interface{}{"uuid": "value"}, "value"},
		{"missingKey", map[string]interface{}{"uuid": "value"}, ""},
		{"missingKey", map[string]interface{}{}, ""},
		{"missingKey", nil, ""},
	}

	for _, testCase := range tests {
		value := getMapValueAsString(testCase.key, testCase.content)
		assert.Equal(testCase.expValue, value)
	}
}

func TestGetIdentifiers(t *testing.T) {
	assert := assert.New(t)
	tests := []struct {
		c      ContentModel
		expIDs []Identifier
	}{
		{map[string]interface{}{"identifiers": []Identifier{}}, []Identifier{}},
		{map[string]interface{}{"uuid": "value",
			"identifiers": []Identifier{{"auth", "value"}}}, []Identifier{{"auth", "value"}}},
		{map[string]interface{}{}, []Identifier{}},
		{nil, []Identifier{}},
	}

	for _, testCase := range tests {
		arr := testCase.c.getIdentifiers()
		assert.Equal(testCase.expIDs, arr)
	}
}
