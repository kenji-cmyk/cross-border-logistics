package ports

import (
	"context"
	"time"

	"github.com/example/cross-border-logistics/internal/warehouse/domain"
	"github.com/example/cross-border-logistics/pkg/event"
	"github.com/google/uuid"
)

type OrderWarehouseSummary struct {
	OrderID    uuid.UUID
	CustomerID string
	Status     string
	CreatedAt  time.Time
}

type OrderReader interface {
	GetWarehouseSummary(context.Context, uuid.UUID, string) (OrderWarehouseSummary, error)
}

type PackageRepository interface {
	ExistsBySourceTrackingNumber(context.Context, string) (bool, error)
	CreateWithOutbox(context.Context, *domain.Package, event.Envelope) error
	FindByID(context.Context, uuid.UUID) (*domain.Package, error)
}
