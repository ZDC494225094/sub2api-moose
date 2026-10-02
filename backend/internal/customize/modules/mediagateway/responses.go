package mediagateway

import "encoding/json"

// ResponsesOperation owns only server-side image generation tools. Client function
// namespaces (including passive image_gen), vision inputs and native text stay open.
func ResponsesOperation(body []byte) Operation {
	var request struct {
		Tools []struct {
			Type string `json:"type"`
		} `json:"tools"`
	}
	if json.Unmarshal(body, &request) != nil {
		return ReadExisting
	}
	for _, tool := range request.Tools {
		if tool.Type == "image_generation" {
			return Submit
		}
	}
	return ReadExisting
}
