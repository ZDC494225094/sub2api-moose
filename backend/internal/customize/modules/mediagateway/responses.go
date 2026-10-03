package mediagateway

import "encoding/json"

// Responses image generation tools are native upstream capabilities, not owned
// by this extension. Classify them explicitly so they never read its flag.
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
			return Native
		}
	}
	return ReadExisting
}
