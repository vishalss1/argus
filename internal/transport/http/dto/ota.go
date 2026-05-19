package dto

type DeployOTARequest struct {
	ArtifactID string `json:"artifact_id" validate:"required"`
}

type OTAResultRequest struct {
	Message string `json:"message,omitempty"`
}
