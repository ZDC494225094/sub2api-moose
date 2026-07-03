package service

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPlaygroundRunServiceExecuteImageUsesGenerationsWithoutUploads(t *testing.T) {
	var gotPath string
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`))
	}))
	defer server.Close()

	svc := NewPlaygroundRunService()
	raw, err := svc.executeImage(context.Background(), "1:run", PlaygroundRunRequest{
		APIKey:       "sk-test",
		EndpointBase: "/v1",
		Model:        "gpt-image-2",
		Prompt:       "draw a cat",
		Size:         "1024x1024",
		N:            1,
		OutputFormat: "png",
	}, server.URL, time.Now())
	if err != nil {
		t.Fatalf("execute image: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("expected raw response")
	}
	if gotPath != "/v1/images/generations" {
		t.Fatalf("path = %q, want /v1/images/generations", gotPath)
	}
	if gotPayload["response_format"] != "b64_json" {
		t.Fatalf("response_format = %v, want b64_json", gotPayload["response_format"])
	}
}

func TestPlaygroundRunServiceExecuteImageUsesEditsWithUploads(t *testing.T) {
	var gotPath string
	fields := map[string]string{}
	var gotUploadName string
	var gotUploadType string
	var gotUploadBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("parse content type: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Fatalf("content type = %q, want multipart/form-data", mediaType)
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := reader.NextPart()
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("read part: %v", err)
			}
			name := part.FormName()
			data, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("read part body: %v", err)
			}
			if part.FileName() != "" {
				gotUploadName = part.FileName()
				gotUploadType = part.Header.Get("Content-Type")
				gotUploadBody = string(data)
			} else {
				fields[name] = string(data)
			}
			_ = part.Close()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"ZWRpdGVk"}]}`))
	}))
	defer server.Close()

	svc := NewPlaygroundRunService()
	raw, err := svc.executeImage(context.Background(), "1:run", PlaygroundRunRequest{
		APIKey:       "sk-test",
		EndpointBase: "/v1",
		Model:        "gpt-image-2",
		Prompt:       "replace background",
		Size:         "1536x1024",
		N:            1,
		OutputFormat: "png",
		Images: []PlaygroundRunImageInput{{
			Name:    "source.png",
			Type:    "image/png",
			DataURL: "data:image/png;base64,cG5nLWJ5dGVz",
		}},
	}, server.URL, time.Now())
	if err != nil {
		t.Fatalf("execute image edit: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("expected raw response")
	}
	if gotPath != "/v1/images/edits" {
		t.Fatalf("path = %q, want /v1/images/edits", gotPath)
	}
	if fields["prompt"] != "replace background" {
		t.Fatalf("prompt = %q", fields["prompt"])
	}
	if fields["response_format"] != "b64_json" {
		t.Fatalf("response_format = %q, want b64_json", fields["response_format"])
	}
	if gotUploadName != "source.png" || gotUploadType != "image/png" || !strings.Contains(gotUploadBody, "png-bytes") {
		t.Fatalf("upload = name:%q type:%q body:%q", gotUploadName, gotUploadType, gotUploadBody)
	}
}
