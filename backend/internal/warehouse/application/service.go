package application

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/example/cross-border-logistics/internal/warehouse/domain"
	"github.com/example/cross-border-logistics/internal/warehouse/ports"
	sharedevent "github.com/example/cross-border-logistics/pkg/event"
	"github.com/google/uuid"
)

const eligibleOrderStatus = "WAITING_PURCHASE"

type ReceivePackageInput struct {
	OrderID, SourceTrackingNumber, WarehouseCode string
	WeightKg, LengthCm, WidthCm, HeightCm        float64
	RequestID                                    string
}

type Service struct {
	repository ports.PackageRepository
	orders     ports.OrderReader
	now        func() time.Time
}

func NewService(repository ports.PackageRepository, orders ports.OrderReader) *Service {
	return &Service{repository: repository, orders: orders, now: time.Now}
}

func (s *Service) Get(ctx context.Context, id string) (domain.Package, error) {
	packageID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return domain.Package{}, domain.ErrInvalidPackageInput
	}
	pkg, err := s.repository.FindByID(ctx, packageID)
	if err != nil {
		return domain.Package{}, err
	}
	return *pkg, nil
}

func (s *Service) ReceivePackage(ctx context.Context, input ReceivePackageInput) (domain.Package, error) {
	orderID, err := uuid.Parse(strings.TrimSpace(input.OrderID))
	if err != nil {
		return domain.Package{}, domain.ErrInvalidPackageInput
	}
	tracking := strings.ToUpper(strings.TrimSpace(input.SourceTrackingNumber))
	warehouse := strings.ToUpper(strings.TrimSpace(input.WarehouseCode))
	if tracking == "" {
		return domain.Package{}, domain.ErrInvalidTrackingNumber
	}
	if warehouse == "" {
		return domain.Package{}, domain.ErrInvalidWarehouseCode
	}
	if !validMeasurement(input.WeightKg, 500) || !validMeasurement(input.LengthCm, 500) || !validMeasurement(input.WidthCm, 500) || !validMeasurement(input.HeightCm, 500) {
		return domain.Package{}, domain.ErrInvalidPackageDimensions
	}
	exists, err := s.repository.ExistsBySourceTrackingNumber(ctx, tracking)
	if err != nil {
		return domain.Package{}, err
	}
	if exists {
		return domain.Package{}, domain.ErrPackageAlreadyExists
	}
	summary, err := s.orders.GetWarehouseSummary(ctx, orderID, input.RequestID)
	if err != nil {
		return domain.Package{}, err
	}
	if summary.OrderID != orderID {
		return domain.Package{}, domain.ErrOrderNotFound
	}
	if summary.Status != eligibleOrderStatus {
		return domain.Package{}, domain.ErrOrderNotEligibleForWarehouse
	}
	now := s.now().UTC()
	pkg := domain.Package{ID: uuid.New(), OrderID: orderID, SourceTrackingNumber: tracking, WarehouseCode: warehouse, WeightKg: input.WeightKg, LengthCm: input.LengthCm, WidthCm: input.WidthCm, HeightCm: input.HeightCm, Status: domain.StatusReceivedAtForeignWarehouse, ReceivedAt: now, CreatedAt: now, UpdatedAt: now}
	envelope, err := sharedevent.New(sharedevent.PackageReceived, orderID, "warehouse-service", now, sharedevent.PackageReceivedData{PackageID: pkg.ID, OrderID: orderID, SourceTrackingNumber: tracking, WarehouseCode: warehouse, WeightKg: pkg.WeightKg, LengthCm: pkg.LengthCm, WidthCm: pkg.WidthCm, HeightCm: pkg.HeightCm})
	if err != nil {
		return domain.Package{}, err
	}
	if err := s.repository.CreateWithOutbox(ctx, &pkg, envelope); err != nil {
		return domain.Package{}, err
	}
	return pkg, nil
}

func validMeasurement(value, maximum float64) bool {
	return value > 0 && value <= maximum && !math.IsNaN(value) && !math.IsInf(value, 0)
}
