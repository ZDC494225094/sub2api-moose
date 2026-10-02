package mediagateway

import "net/http"

// Endpoint is the protocol registry shared by the host adapters. Only explicit
// lookup/cancellation operations drain; unknown endpoints cannot bypass admission.
type Endpoint string

const (
	ImagesGenerations Endpoint = "images_generations"
	ImagesEdits       Endpoint = "images_edits"
	VideosGenerations Endpoint = "videos_generations"
	VideosEdits       Endpoint = "videos_edits"
	VideosExtensions  Endpoint = "videos_extensions"
	VideoStatus       Endpoint = "video_status"
	VideoContent      Endpoint = "video_content"
	SeedanceCreate    Endpoint = "seedance_create"
	SeedanceStatus    Endpoint = "seedance_status"
	SeedanceDelete    Endpoint = "seedance_delete"
)

func (e Endpoint) IsSeedance() bool {
	return e == SeedanceCreate || e == SeedanceStatus || e == SeedanceDelete
}
func (e Endpoint) IsVideoLookupRequest() bool {
	return e == VideoStatus || e == VideoContent || e == SeedanceStatus || e == SeedanceDelete
}
func (e Endpoint) RequiresRequestBody() bool { return !e.IsVideoLookupRequest() }
func (e Endpoint) IsGenerationRequest() bool {
	switch e {
	case ImagesGenerations, ImagesEdits, VideosGenerations, VideosEdits, VideosExtensions, SeedanceCreate:
		return true
	default:
		return false
	}
}
func (e Endpoint) Operation() Operation {
	if e == SeedanceDelete {
		return CancelExisting
	}
	if e.IsVideoLookupRequest() {
		return ReadExisting
	}
	if e.IsGenerationRequest() {
		return Submit
	}
	return Operation("unknown")
}
func (e Endpoint) HTTPMethod() string {
	if e == SeedanceDelete {
		return http.MethodDelete
	}
	if e.IsVideoLookupRequest() {
		return http.MethodGet
	}
	return http.MethodPost
}
