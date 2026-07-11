package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/example/cross-border-logistics/internal/warehouse/domain"
	warehousemigration "github.com/example/cross-border-logistics/migrations/warehouse"
	"github.com/example/cross-border-logistics/pkg/event"
	sharedkafka "github.com/example/cross-border-logistics/pkg/kafka"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, warehousemigration.SQL)
	if err != nil {
		return fmt.Errorf("migrate warehouse: %w", err)
	}
	return nil
}

func (r *Repository) ExistsBySourceTrackingNumber(ctx context.Context, tracking string) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM packages WHERE source_tracking_number=$1)`, tracking).Scan(&exists); err != nil {
		return false, fmt.Errorf("check package tracking number: %w", err)
	}
	return exists, nil
}
func (r *Repository) CreateWithOutbox(ctx context.Context, pkg *domain.Package, envelope event.Envelope) error {
	payload, err := envelope.Marshal()
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin receive package: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `INSERT INTO packages (id,order_id,source_tracking_number,warehouse_code,weight_kg,length_cm,width_cm,height_cm,status,received_at,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, pkg.ID, pkg.OrderID, pkg.SourceTrackingNumber, pkg.WarehouseCode, pkg.WeightKg, pkg.LengthCm, pkg.WidthCm, pkg.HeightCm, pkg.Status, pkg.ReceivedAt, pkg.CreatedAt, pkg.UpdatedAt)
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			return domain.ErrPackageAlreadyExists
		}
		return fmt.Errorf("insert package: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO outbox_events (id,aggregate_id,event_type,payload,created_at) VALUES ($1,$2,$3,$4,$5)`, envelope.EventID, envelope.AggregateID, envelope.EventType, payload, envelope.OccurredAt); err != nil {
		return fmt.Errorf("insert warehouse outbox: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit receive package: %w", err)
	}
	return nil
}
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Package, error) {
	var p domain.Package
	err := r.pool.QueryRow(ctx, `SELECT id,order_id,source_tracking_number,warehouse_code,weight_kg::float8,length_cm::float8,width_cm::float8,height_cm::float8,status,received_at,created_at,updated_at FROM packages WHERE id=$1`, id).Scan(&p.ID, &p.OrderID, &p.SourceTrackingNumber, &p.WarehouseCode, &p.WeightKg, &p.LengthCm, &p.WidthCm, &p.HeightCm, &p.Status, &p.ReceivedAt, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrPackageNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find package: %w", err)
	}
	return &p, nil
}
func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]sharedkafka.OutboxEvent, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text,aggregate_id::text,event_type,payload FROM outbox_events WHERE published_at IS NULL ORDER BY created_at,id LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch warehouse outbox: %w", err)
	}
	defer rows.Close()
	var items []sharedkafka.OutboxEvent
	for rows.Next() {
		var item sharedkafka.OutboxEvent
		if err := rows.Scan(&item.ID, &item.AggregateID, &item.EventType, &item.Payload); err != nil {
			return nil, fmt.Errorf("scan warehouse outbox: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (r *Repository) MarkPublished(ctx context.Context, id string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE outbox_events SET published_at=$2,attempts=attempts+1,last_error=NULL WHERE id=$1 AND published_at IS NULL`, id, at)
	return err
}
func (r *Repository) MarkFailed(ctx context.Context, id, message string) error {
	_, err := r.pool.Exec(ctx, `UPDATE outbox_events SET attempts=attempts+1,last_error=$2 WHERE id=$1 AND published_at IS NULL`, id, message)
	return err
}
