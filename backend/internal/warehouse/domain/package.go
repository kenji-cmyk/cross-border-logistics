package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type PackageStatus string

const StatusReceivedAtForeignWarehouse PackageStatus = "RECEIVED_AT_FOREIGN_WAREHOUSE"

var (
	ErrPackageNotFound              = errors.New("package not found")
	ErrPackageAlreadyExists         = errors.New("package already exists")
	ErrInvalidPackageInput          = errors.New("invalid package input")
	ErrInvalidTrackingNumber        = errors.New("invalid tracking number")
	ErrInvalidWarehouseCode         = errors.New("invalid warehouse code")
	ErrInvalidPackageDimensions     = errors.New("invalid package dimensions")
	ErrOrderNotFound                = errors.New("order not found")
	ErrOrderNotEligibleForWarehouse = errors.New("order is not eligible for warehouse receiving")
	ErrOrderServiceUnavailable      = errors.New("order service unavailable")
)

type Package struct {
	ID                   uuid.UUID     `json:"packageId"`
	OrderID              uuid.UUID     `json:"orderId"`
	SourceTrackingNumber string        `json:"sourceTrackingNumber"`
	WarehouseCode        string        `json:"warehouseCode"`
	WeightKg             float64       `json:"weightKg"`
	LengthCm             float64       `json:"lengthCm"`
	WidthCm              float64       `json:"widthCm"`
	HeightCm             float64       `json:"heightCm"`
	Status               PackageStatus `json:"status"`
	ReceivedAt           time.Time     `json:"receivedAt"`
	CreatedAt            time.Time     `json:"createdAt"`
	UpdatedAt            time.Time     `json:"updatedAt"`
}
