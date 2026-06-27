package fleet

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/vishalss1/argus/shared/common"
	"github.com/vishalss1/argus/core/internal/domain/certificate"
	"github.com/vishalss1/argus/core/internal/domain/device"
	"github.com/vishalss1/argus/core/internal/firmware"
	"github.com/vishalss1/argus/core/internal/domain/ota"
)

type Service struct {
	repo          Repository
	deviceService *device.Service
	ca            *certificate.CertificateAuthority
	fwGen         *firmware.Generator
	otaService    *ota.Service
}

func NewService(
	repo Repository,
	deviceService *device.Service,
	ca *certificate.CertificateAuthority,
	fwGen *firmware.Generator,
	otaService *ota.Service,
) *Service {
	return &Service{
		repo:          repo,
		deviceService: deviceService,
		ca:            ca,
		fwGen:         fwGen,
		otaService:    otaService,
	}
}

func (s *Service) CreateFleet(ctx context.Context, input CreateFleetInput) (*FleetProvisionResult, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, ErrFleetNameEmpty
	}
	if strings.TrimSpace(input.HardwareType) == "" {
		return nil, ErrHardwareTypeEmpty
	}
	if input.NodeCount < 1 || input.NodeCount > 500 {
		return nil, ErrNodeCountInvalid
	}

	prefix := strings.TrimSpace(input.NodePrefix)
	if prefix == "" {
		prefix = "Node"
	}

	var wID *string
	if val, ok := common.GetWorkspaceID(ctx); ok {
		wID = &val
	}

	f := Fleet{
		WorkspaceID:      *wID,
		Name:             strings.TrimSpace(input.Name),
		NodeRole:         strings.TrimSpace(input.NodeRole),
		HardwareType:     strings.TrimSpace(input.HardwareType),
		NodePrefix:       prefix,
		FirmwareVersion:  strings.TrimSpace(input.FirmwareVersion),
		FirmwareTemplate: input.FirmwareTemplate,
		NodeCount:        input.NodeCount,
	}

	createdFleet, err := s.repo.Create(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("failed to create fleet record: %w", err)
	}

	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	for i := 1; i <= input.NodeCount; i++ {
		nodeName := fmt.Sprintf("%s %d", prefix, i)

		// Create device
		dev, err := s.deviceService.Create(ctx, device.CreateInput{
			Name:            nodeName,
			Type:            f.HardwareType,
			FirmwareVersion: f.FirmwareVersion,
			Status:          "offline",
			Metadata:        json.RawMessage(`{}`),
		})
		if err != nil {
			// ponytail: intentional simplification: fail the whole request instead of partial cleanup
			return nil, fmt.Errorf("failed to create node %d: %w", i, err)
		}

		// Link device to fleet - doing it by updating it directly
		// (Wait, doing Update involves repo.Update. A helper CreateWithFleet is better, but this works for now)
		_, err = s.deviceService.Update(ctx, dev.ID, device.UpdateInput{
			FleetID: &createdFleet.ID,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to link node %d to fleet: %w", i, err)
		}

		// Issue cert
		cert, err := s.ca.IssueDeviceCertificate(dev.ID, *wID)
		if err != nil {
			return nil, fmt.Errorf("failed to issue cert for node %d: %w", i, err)
		}

		apiKey := ""
		if dev.RawAPIKey != nil {
			apiKey = *dev.RawAPIKey
		}

		// Generate firmware
		fwBytes, err := s.fwGen.GenerateProvision(firmware.GenerateOptions{
			DeviceID:        dev.ID,
			WorkspaceID:     *wID,
			APIKey:          apiKey,
			FirmwareVersion: f.FirmwareVersion,
			CertPEM:         cert.CertPEM,
			PrivKeyPEM:      cert.PrivateKeyPEM,
			WiFiSSID:        input.WiFiSSID,
			WiFiPassword:    input.WiFiPassword,
			UserCode:        input.FirmwareTemplate,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to generate firmware for node %d: %w", i, err)
		}

		// Add to zip
		fileName := fmt.Sprintf("config_%s.ino", dev.ID)
		fWriter, err := zipWriter.Create(fileName)
		if err != nil {
			return nil, fmt.Errorf("failed to create zip entry for node %d: %w", i, err)
		}
		if _, err := fWriter.Write(fwBytes); err != nil {
			return nil, fmt.Errorf("failed to write firmware for node %d to zip: %w", i, err)
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize zip: %w", err)
	}

	return &FleetProvisionResult{
		Fleet:   *createdFleet,
		ZipData: buf.Bytes(),
	}, nil
}

func (s *Service) List(ctx context.Context) ([]FleetWithStats, error) {
	return s.repo.List(ctx)
}

func (s *Service) GetByID(ctx context.Context, id string) (*Fleet, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) GetWithDevices(ctx context.Context, id string) (*FleetWithStats, error) {
	return s.repo.GetWithDevices(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) DeployToFleet(ctx context.Context, fleetID string, artifactID string) (*FleetDeployResult, error) {
	if fleetID == "" || artifactID == "" {
		return nil, fmt.Errorf("fleetID and artifactID are required")
	}

	fleetWithStats, err := s.repo.GetWithDevices(ctx, fleetID)
	if err != nil {
		return nil, err
	}

	_, err = s.otaService.GetFirmware(ctx, artifactID)
	if err != nil {
		return nil, fmt.Errorf("artifact not found: %w", err)
	}

	var errorsList []FleetDeployError
	deployedCount := 0

	for _, dev := range fleetWithStats.Devices {
		// ponytail: log and continue on failure
		_, err := s.otaService.Deploy(ctx, dev.ID, ota.DeployInput{ArtifactID: artifactID})
		if err != nil {
			log.Printf("[FLEET] OTA deploy failed device=%s error=%v", dev.ID, err)
			errorsList = append(errorsList, FleetDeployError{DeviceID: dev.ID, Error: err.Error()})
			continue
		}
		deployedCount++
	}

	return &FleetDeployResult{
		FleetID:       fleetID,
		ArtifactID:    artifactID,
		DeployedCount: deployedCount,
		TotalCount:    len(fleetWithStats.Devices),
		Errors:        errorsList,
	}, nil
}
