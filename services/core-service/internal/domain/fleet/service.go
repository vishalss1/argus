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
	if val, ok := common.GetWorkspaceID(ctx); ok && strings.TrimSpace(val) != "" {
		wID = &val
	} else {
		// ponytail: guard against nil workspace ID panic
		return nil, ErrWorkspaceRequired
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

	zipData, err := s.provisionNodes(ctx, createdFleet, *wID, 1, input.NodeCount, prefix, input.WiFiSSID, input.WiFiPassword)
	if err != nil {
		return nil, err
	}

	return &FleetProvisionResult{
		Fleet:   *createdFleet,
		ZipData: zipData,
	}, nil
}

type AddDevicesInput struct {
	FleetID      string
	NodeCount    int
	NodePrefix   string
	WiFiSSID     string
	WiFiPassword string
}

func (s *Service) AddDevicesToFleet(ctx context.Context, input AddDevicesInput) (*FleetProvisionResult, error) {
	if input.NodeCount < 1 || input.NodeCount > 500 {
		return nil, ErrNodeCountInvalid
	}

	fleetWithStats, err := s.repo.GetWithDevices(ctx, input.FleetID)
	if err != nil {
		return nil, fmt.Errorf("fleet not found: %w", err)
	}
	fleetObj := &fleetWithStats.Fleet

	prefix := strings.TrimSpace(input.NodePrefix)
	if prefix == "" {
		prefix = fleetObj.NodePrefix
		if prefix == "" {
			prefix = "Node"
		}
	}

	startIndex := len(fleetWithStats.Devices) + 1

	var wID *string
	if val, ok := common.GetWorkspaceID(ctx); ok {
		wID = &val
	} else {
		wID = &fleetObj.WorkspaceID
	}

	zipData, err := s.provisionNodes(ctx, fleetObj, *wID, startIndex, input.NodeCount, prefix, input.WiFiSSID, input.WiFiPassword)
	if err != nil {
		return nil, err
	}

	return &FleetProvisionResult{
		Fleet:   *fleetObj,
		ZipData: zipData,
	}, nil
}

// provisionNodes creates count devices numbered from startIndex, links them to
// the fleet, issues certificates, and returns a zip of per-device provisioning
// sketches plus the fleet firmware sketch.
func (s *Service) provisionNodes(ctx context.Context, f *Fleet, wID string, startIndex, count int, prefix, wifiSSID, wifiPassword string) ([]byte, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	for i := startIndex; i < startIndex+count; i++ {
		nodeName := fmt.Sprintf("%s %d", prefix, i)

		dev, err := s.deviceService.Create(ctx, device.CreateInput{
			Name:            nodeName,
			Type:            f.HardwareType,
			FirmwareVersion: f.FirmwareVersion,
			Status:          "offline",
			Metadata:        json.RawMessage(`{}`),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create node %d: %w", i, err)
		}

		_, err = s.deviceService.Update(ctx, dev.ID, device.UpdateInput{
			FleetID: &f.ID,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to link node %d to fleet: %w", i, err)
		}

		cert, err := s.ca.IssueDeviceCertificate(dev.ID, wID)
		if err != nil {
			return nil, fmt.Errorf("failed to issue cert for node %d: %w", i, err)
		}

		apiKey := ""
		if dev.RawAPIKey != nil {
			apiKey = *dev.RawAPIKey
		}

		fwBytes, err := s.fwGen.GenerateProvision(firmware.GenerateOptions{
			DeviceID:        dev.ID,
			WorkspaceID:     wID,
			APIKey:          apiKey,
			FirmwareVersion: f.FirmwareVersion,
			CertPEM:         cert.CertPEM,
			PrivKeyPEM:      cert.PrivateKeyPEM,
			WiFiSSID:        wifiSSID,
			WiFiPassword:    wifiPassword,
			UserCode:        f.FirmwareTemplate,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to generate firmware for node %d: %w", i, err)
		}

		baseName := fmt.Sprintf("config_%s", dev.ID)
		fWriter, err := zipWriter.Create(fmt.Sprintf("%s/%s.ino", baseName, baseName))
		if err != nil {
			return nil, fmt.Errorf("failed to create zip entry for node %d: %w", i, err)
		}
		if _, err := fWriter.Write(fwBytes); err != nil {
			return nil, fmt.Errorf("failed to write firmware for node %d to zip: %w", i, err)
		}
	}

	fleetFWBytes, err := s.fwGen.GenerateFleetFirmware(f.FirmwareTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to generate fleet firmware: %w", err)
	}
	fleetFWBase := fmt.Sprintf("fleet_firmware_%s", f.ID)
	fleetFWWriter, err := zipWriter.Create(fmt.Sprintf("%s/%s.ino", fleetFWBase, fleetFWBase))
	if err != nil {
		return nil, fmt.Errorf("failed to create zip entry for fleet firmware: %w", err)
	}
	if _, err := fleetFWWriter.Write(fleetFWBytes); err != nil {
		return nil, fmt.Errorf("failed to write fleet firmware to zip: %w", err)
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize zip: %w", err)
	}

	return buf.Bytes(), nil
}


func (s *Service) List(ctx context.Context) ([]FleetWithStats, error) {
	return s.repo.List(ctx)
}

func (s *Service) GetByID(ctx context.Context, id string) (*Fleet, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) GenerateFleetFirmware(ctx context.Context, fleetID string) ([]byte, error) {
	fleet, err := s.repo.GetByID(ctx, fleetID)
	if err != nil {
		return nil, fmt.Errorf("fleet not found: %w", err)
	}

	return s.fwGen.GenerateFleetFirmware(fleet.FirmwareTemplate)
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
		// Log and record deployment error per device without aborting entire fleet rollout
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
