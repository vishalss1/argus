package repository

import (
	"context"
	"errors"

	"github.com/vishalss1/argus/src/internal/model"
)

var ErrDeviceNotFound = errors.New("device not found")

type DeviceRepository interface {
	Create(ctx context.Context, device model.Device) (*model.Device, error)
	List(ctx context.Context) ([]model.Device, error)
	GetByID(ctx context.Context, id string) (*model.Device, error)
	Update(ctx context.Context, id string, req model.UpdateDeviceRequest) (*model.Device, error)
	Delete(ctx context.Context, id string) error
}
