package mediagateway

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestResponsesOnlyOwnsServerImageTools(t *testing.T) {
	for _, tc := range []struct {
		body string
		want Operation
	}{
		{`{"tools":[{"type":"image_generation"}]}`, Native},
		{`{"tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"text2im"}]}]}`, ReadExisting},
		{`{"tools":[{"type":"function","name":"image_generation"}]}`, ReadExisting},
		{`{"input":[{"type":"input_image","image_url":"https://example.invalid/x"}]}`, ReadExisting},
		{`{"input":"draw a cat"}`, ReadExisting}, {`invalid`, ReadExisting},
	} {
		require.Equal(t, tc.want, ResponsesOperation([]byte(tc.body)), tc.body)
	}
}
