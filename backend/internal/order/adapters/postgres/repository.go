package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/example/cross-border-logistics/internal/order/domain"
	"github.com/example/cross-border-logistics/internal/order/ports"
	ordermigration "github.com/example/cross-border-logistics/migrations/order"
	sharedkafka "github.com/example/cross-border-logistics/pkg/kafka"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, ordermigration.SQL); err != nil {
		return fmt.Errorf("migrate orders: %w", err)
	}
	return nil
}

func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]sharedkafka.OutboxEvent, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text,aggregate_id::text,event_type,payload FROM outbox_events WHERE published_at IS NULL ORDER BY created_at,id LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch order outbox: %w", err)
	}
	defer rows.Close()
	var events []sharedkafka.OutboxEvent
	for rows.Next() {
		var item sharedkafka.OutboxEvent
		if err := rows.Scan(&item.ID, &item.AggregateID, &item.EventType, &item.Payload); err != nil {
			return nil, fmt.Errorf("scan order outbox: %w", err)
		}
		events = append(events, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order outbox: %w", err)
	}
	return events, nil
}

func (r *Repository) MarkPublished(ctx context.Context, id string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE outbox_events SET published_at=$2,attempts=attempts+1,last_error=NULL WHERE id=$1 AND published_at IS NULL`, id, at)
	if err != nil {
		return fmt.Errorf("mark order outbox published: %w", err)
	}
	return nil
}

func (r *Repository) MarkFailed(ctx context.Context, id, message string) error {
	_, err := r.pool.Exec(ctx, `UPDATE outbox_events SET attempts=attempts+1,last_error=$2 WHERE id=$1 AND published_at IS NULL`, id, message)
	if err != nil {
		return fmt.Errorf("mark order outbox failed: %w", err)
	}
	return nil
}

func (r *Repository) ProcessPaymentSucceeded(ctx context.Context, input ports.ProcessPaymentSucceeded) (bool, error) {
	return r.processStatusEvent(ctx, input.EventID, input.EventType, input.OrderID, input.ProcessedAt, input.Tracking, input.Outbox, domain.StatusWaitingPurchase)
}

func (r *Repository) ProcessPackageReceived(ctx context.Context, input ports.ProcessPackageReceived) (bool, error) {
	return r.processStatusEvent(ctx, input.EventID, input.EventType, input.OrderID, input.ProcessedAt, input.Tracking, input.Outbox, domain.StatusArrivedForeignWarehouse)
}

func (r *Repository) processStatusEvent(ctx context.Context, eventID, eventType, orderID string, processedAt time.Time, tracking domain.TrackingEvent, outbox ports.OutboxEvent, target domain.OrderStatus) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin payment event: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var processed bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM processed_events WHERE event_id=$1)`, eventID).Scan(&processed); err != nil {
		return false, fmt.Errorf("check processed event: %w", err)
	}
	if processed {
		if err := tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("commit duplicate event: %w", err)
		}
		return false, nil
	}
	var current domain.OrderStatus
	if err := tx.QueryRow(ctx, `SELECT status FROM orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&current); errors.Is(err, pgx.ErrNoRows) {
		return false, domain.ErrOrderNotFound
	} else if err != nil {
		return false, fmt.Errorf("lock order: %w", err)
	}
	if !domain.CanTransition(current, target) {
		return false, domain.ErrInvalidTransition
	}
	if _, err := tx.Exec(ctx, `UPDATE orders SET status=$2,updated_at=$3 WHERE id=$1`, orderID, target, processedAt); err != nil {
		return false, fmt.Errorf("update order from payment event: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO tracking_events (id,order_id,status,description,source,occurred_at,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, tracking.ID, tracking.OrderID, tracking.Status, tracking.Description, tracking.Source, tracking.OccurredAt, tracking.CreatedAt); err != nil {
		return false, fmt.Errorf("insert payment tracking event: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO processed_events (event_id,event_type,processed_at) VALUES ($1,$2,$3)`, eventID, eventType, processedAt); err != nil {
		return false, fmt.Errorf("insert processed event: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO outbox_events (id,aggregate_id,event_type,payload,created_at) VALUES ($1,$2,$3,$4,$5)`, outbox.ID, outbox.AggregateID, outbox.EventType, outbox.Payload, outbox.CreatedAt); err != nil {
		return false, fmt.Errorf("insert status outbox event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit payment event: %w", err)
	}
	return true, nil
}

func (r *Repository) Create(ctx context.Context, order domain.Order, tracking domain.TrackingEvent, outbox ports.OutboxEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create order: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `INSERT INTO orders (id,customer_id,quotation_id,delivery_address,total_amount_vnd,deposit_amount_vnd,remaining_amount_vnd,status,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, order.ID, order.CustomerID, order.QuotationID, order.DeliveryAddress, order.TotalAmountVND, order.DepositAmountVND, order.RemainingAmountVND, order.Status, order.CreatedAt, order.UpdatedAt)
	if err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) && pgError.Code == "23505" && pgError.ConstraintName == "orders_quotation_id_key" {
			return domain.ErrQuotationConflict
		}
		return fmt.Errorf("insert order: %w", err)
	}
	for _, item := range order.Items {
		if _, err := tx.Exec(ctx, `INSERT INTO order_items (id,order_id,product_name,product_url,quantity,unit_price_vnd,total_price_vnd,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, item.ID, item.OrderID, item.ProductName, item.ProductURL, item.Quantity, item.UnitPriceVND, item.TotalPriceVND, item.CreatedAt); err != nil {
			return fmt.Errorf("insert order item: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO tracking_events (id,order_id,status,description,source,occurred_at,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, tracking.ID, tracking.OrderID, tracking.Status, tracking.Description, tracking.Source, tracking.OccurredAt, tracking.CreatedAt); err != nil {
		return fmt.Errorf("insert tracking event: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO outbox_events (id,aggregate_id,event_type,payload,created_at) VALUES ($1,$2,$3,$4,$5)`, outbox.ID, outbox.AggregateID, outbox.EventType, outbox.Payload, outbox.CreatedAt); err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create order: %w", err)
	}
	return nil
}

func (r *Repository) FindByID(ctx context.Context, id string) (domain.Order, error) {
	var order domain.Order
	err := r.pool.QueryRow(ctx, `SELECT id::text,customer_id,quotation_id::text,delivery_address,total_amount_vnd,deposit_amount_vnd,remaining_amount_vnd,status,created_at,updated_at FROM orders WHERE id=$1`, id).Scan(&order.ID, &order.CustomerID, &order.QuotationID, &order.DeliveryAddress, &order.TotalAmountVND, &order.DepositAmountVND, &order.RemainingAmountVND, &order.Status, &order.CreatedAt, &order.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	if err != nil {
		return domain.Order{}, fmt.Errorf("find order: %w", err)
	}
	rows, err := r.pool.Query(ctx, `SELECT id::text,order_id::text,product_name,product_url,quantity,unit_price_vnd,total_price_vnd,created_at FROM order_items WHERE order_id=$1 ORDER BY created_at,id`, id)
	if err != nil {
		return domain.Order{}, fmt.Errorf("find order items: %w", err)
	}
	defer rows.Close()
	order.Items = []domain.OrderItem{}
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.ProductName, &item.ProductURL, &item.Quantity, &item.UnitPriceVND, &item.TotalPriceVND, &item.CreatedAt); err != nil {
			return domain.Order{}, fmt.Errorf("scan order item: %w", err)
		}
		order.Items = append(order.Items, item)
	}
	if err := rows.Err(); err != nil {
		return domain.Order{}, fmt.Errorf("iterate order items: %w", err)
	}
	return order, nil
}

func (r *Repository) FindTimeline(ctx context.Context, id string) ([]domain.TrackingEvent, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM orders WHERE id=$1)`, id).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check order: %w", err)
	}
	if !exists {
		return nil, domain.ErrOrderNotFound
	}
	rows, err := r.pool.Query(ctx, `SELECT id::text,order_id::text,status,description,source,occurred_at,created_at FROM tracking_events WHERE order_id=$1 ORDER BY occurred_at,created_at,id`, id)
	if err != nil {
		return nil, fmt.Errorf("find timeline: %w", err)
	}
	defer rows.Close()
	events := []domain.TrackingEvent{}
	for rows.Next() {
		var event domain.TrackingEvent
		if err := rows.Scan(&event.ID, &event.OrderID, &event.Status, &event.Description, &event.Source, &event.OccurredAt, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan timeline: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate timeline: %w", err)
	}
	return events, nil
}
