package dto

type DeployOTARequest struct {
	ArtifactID string `json:"artifact_id" validate:"required"`
}

type OTAResultRequest struct {
	Message string `json:"message,omitempty"`
}

type OTAProgressRequest struct {
	DeploymentID string `json:"deployment_id"`
	Status       string `json:"status"`
	Progress     *int   `json:"progress,omitempty"`
	Message      string `json:"message,omitempty"`
}

