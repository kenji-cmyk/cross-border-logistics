package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kenji-cmyk/cross-border-logistics/internal/warehouse/application"
	"github.com/kenji-cmyk/cross-border-logistics/internal/warehouse/domain"
	"github.com/kenji-cmyk/cross-border-logistics/internal/warehouse/ports"
	"github.com/kenji-cmyk/cross-border-logistics/pkg/event"
	"github.com/google/uuid"
)

type repo struct {
	exists   bool
	pkg      *domain.Package
	envelope event.Envelope
}

func (r *repo) ExistsBySourceTrackingNumber(context.Context, string) (bool, error) {
	return r.exists, nil
}
func (r *repo) CreateWithOutbox(_ context.Context, p *domain.Package, e event.Envelope) error {
	r.pkg = p
	r.envelope = e
	return nil
}
func (r *repo) FindByID(context.Context, uuid.UUID) (*domain.Package, error) { return r.pkg, nil }

type orders struct {
	status string
	err    error
}

func (o orders) GetWarehouseSummary(_ context.Context, id uuid.UUID, _ string) (ports.OrderWarehouseSummary, error) {
	return ports.OrderWarehouseSummary{OrderID: id, Status: o.status}, o.err
}

func TestReceivePackageCreatesPackageAndOutbox(t *testing.T) {
	r := &repo{}
	s := application.NewService(r, orders{status: "WAITING_PURCHASE"})
	id := uuid.New()
	p, err := s.ReceivePackage(context.Background(), application.ReceivePackageInput{OrderID: id.String(), SourceTrackingNumber: " cn123 ", WarehouseCode: " cn-gz-01 ", WeightKg: 1.4, LengthCm: 30, WidthCm: 20, HeightCm: 15})
	if err != nil {
		t.Fatal(err)
	}
	if p.SourceTrackingNumber != "CN123" || p.Status != domain.StatusReceivedAtForeignWarehouse || r.envelope.EventType != event.PackageReceived || r.envelope.AggregateID != id {
		t.Fatalf("package=%+v envelope=%+v", p, r.envelope)
	}
}
func TestReceivePackageValidationAndConflict(t *testing.T) {
	id := uuid.New().String()
	tests := []struct {
		name  string
		input application.ReceivePackageInput
		r     *repo
		want  error
	}{{"invalid dimension", application.ReceivePackageInput{OrderID: id, SourceTrackingNumber: "x", WarehouseCode: "w", WeightKg: 0, LengthCm: 1, WidthCm: 1, HeightCm: 1}, &repo{}, domain.ErrInvalidPackageDimensions}, {"duplicate", application.ReceivePackageInput{OrderID: id, SourceTrackingNumber: "x", WarehouseCode: "w", WeightKg: 1, LengthCm: 1, WidthCm: 1, HeightCm: 1}, &repo{exists: true}, domain.ErrPackageAlreadyExists}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := application.NewService(tt.r, orders{status: "WAITING_PURCHASE"}).ReceivePackage(context.Background(), tt.input)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error=%v want=%v", err, tt.want)
			}
		})
	}
}
