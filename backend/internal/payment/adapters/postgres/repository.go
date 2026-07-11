package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/example/cross-border-logistics/internal/payment/domain"
	"github.com/example/cross-border-logistics/internal/payment/ports"
	paymentmigration "github.com/example/cross-border-logistics/migrations/payment"
	sharedkafka "github.com/example/cross-border-logistics/pkg/kafka"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]sharedkafka.OutboxEvent, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text,aggregate_id::text,event_type,payload FROM outbox_events WHERE published_at IS NULL ORDER BY created_at,id LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch payment outbox: %w", err)
	}
	defer rows.Close()
	var events []sharedkafka.OutboxEvent
	for rows.Next() {
		var item sharedkafka.OutboxEvent
		if err := rows.Scan(&item.ID, &item.AggregateID, &item.EventType, &item.Payload); err != nil {
			return nil, fmt.Errorf("scan payment outbox: %w", err)
		}
		events = append(events, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate payment outbox: %w", err)
	}
	return events, nil
}

func (r *Repository) MarkPublished(ctx context.Context, id string, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE outbox_events SET published_at=$2,attempts=attempts+1,last_error=NULL WHERE id=$1 AND published_at IS NULL`, id, at)
	if err != nil {
		return fmt.Errorf("mark payment outbox published: %w", err)
	}
	return nil
}

func (r *Repository) MarkFailed(ctx context.Context, id, message string) error {
	_, err := r.pool.Exec(ctx, `UPDATE outbox_events SET attempts=attempts+1,last_error=$2 WHERE id=$1 AND published_at IS NULL`, id, message)
	if err != nil {
		return fmt.Errorf("mark payment outbox failed: %w", err)
	}
	return nil
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, paymentmigration.SQL); err != nil {
		return fmt.Errorf("migrate payments: %w", err)
	}
	return nil
}

func (r *Repository) Create(ctx context.Context, payment domain.Payment) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO payments (id,order_id,type,amount_vnd,currency,status,payment_url,provider_reference,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, payment.ID, payment.OrderID, payment.Type, payment.AmountVND, payment.Currency, payment.Status, payment.PaymentURL, payment.ProviderReference, payment.CreatedAt, payment.UpdatedAt)
	if err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) && pgError.Code == "23505" && pgError.ConstraintName == "payments_one_deposit_per_order_idx" {
			return domain.ErrPaymentConflict
		}
		return fmt.Errorf("create payment: %w", err)
	}
	return nil
}

func (r *Repository) FindByID(ctx context.Context, id string) (domain.Payment, error) {
	return findByID(ctx, r.pool, id, false)
}

func (r *Repository) FindByOrderID(ctx context.Context, id string) (domain.Payment, error) {
	return findByColumn(ctx, r.pool, "order_id", id)
}
func (r *Repository) FindByProviderReference(ctx context.Context, ref string) (domain.Payment, error) {
	return findByColumn(ctx, r.pool, "provider_reference", ref)
}
func findByColumn(ctx context.Context, q rowQuerier, column, value string) (domain.Payment, error) {
	var p domain.Payment
	err := q.QueryRow(ctx, `SELECT id::text,order_id::text,type,amount_vnd,currency,status,payment_url,provider_reference,created_at,updated_at FROM payments WHERE `+column+`=$1`, value).Scan(&p.ID, &p.OrderID, &p.Type, &p.AmountVND, &p.Currency, &p.Status, &p.PaymentURL, &p.ProviderReference, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payment{}, domain.ErrPaymentNotFound
	}
	if err != nil {
		return domain.Payment{}, fmt.Errorf("find payment by %s: %w", column, err)
	}
	return p, nil
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func findByID(ctx context.Context, queryer rowQuerier, id string, forUpdate bool) (domain.Payment, error) {
	query := `SELECT id::text,order_id::text,type,amount_vnd,currency,status,payment_url,provider_reference,created_at,updated_at FROM payments WHERE id=$1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var payment domain.Payment
	err := queryer.QueryRow(ctx, query, id).Scan(&payment.ID, &payment.OrderID, &payment.Type, &payment.AmountVND, &payment.Currency, &payment.Status, &payment.PaymentURL, &payment.ProviderReference, &payment.CreatedAt, &payment.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payment{}, domain.ErrPaymentNotFound
	}
	if err != nil {
		return domain.Payment{}, fmt.Errorf("find payment: %w", err)
	}
	return payment, nil
}

func (r *Repository) Succeed(ctx context.Context, id string, outbox ports.OutboxEvent) (domain.Payment, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Payment{}, false, fmt.Errorf("begin succeed payment: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	payment, err := findByID(ctx, tx, id, true)
	if err != nil {
		return domain.Payment{}, false, err
	}
	if payment.Status == domain.StatusSucceeded {
		if err := tx.Commit(ctx); err != nil {
			return domain.Payment{}, false, fmt.Errorf("commit idempotent payment success: %w", err)
		}
		return payment, false, nil
	}
	if !domain.CanTransition(payment.Status, domain.StatusSucceeded) {
		return domain.Payment{}, false, domain.ErrInvalidState
	}
	if _, err := tx.Exec(ctx, `UPDATE payments SET status=$1,updated_at=$2 WHERE id=$3`, domain.StatusSucceeded, outbox.CreatedAt, id); err != nil {
		return domain.Payment{}, false, fmt.Errorf("update payment status: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO outbox_events (id,aggregate_id,event_type,payload,created_at) VALUES ($1,$2,$3,$4,$5)`, outbox.ID, outbox.AggregateID, outbox.EventType, outbox.Payload, outbox.CreatedAt); err != nil {
		return domain.Payment{}, false, fmt.Errorf("insert payment outbox event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Payment{}, false, fmt.Errorf("commit payment success: %w", err)
	}
	payment.Status = domain.StatusSucceeded
	payment.UpdatedAt = outbox.CreatedAt
	return payment, true, nil
}

func (r *Repository) SucceedCallback(ctx context.Context, id, eventID string, outbox ports.OutboxEvent) (domain.Payment, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Payment{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM provider_callbacks WHERE event_id=$1)`, eventID).Scan(&exists); err != nil {
		return domain.Payment{}, false, err
	}
	if exists {
		p, err := findByID(ctx, tx, id, false)
		if err != nil {
			return domain.Payment{}, false, err
		}
		return p, false, tx.Commit(ctx)
	}
	p, err := findByID(ctx, tx, id, true)
	if err != nil {
		return domain.Payment{}, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO provider_callbacks(event_id,payment_id,received_at) VALUES($1,$2,$3)`, eventID, id, outbox.CreatedAt); err != nil {
		return domain.Payment{}, false, err
	}
	changed := p.Status != domain.StatusSucceeded
	if changed {
		if !domain.CanTransition(p.Status, domain.StatusSucceeded) {
			return domain.Payment{}, false, domain.ErrInvalidState
		}
		if _, err = tx.Exec(ctx, `UPDATE payments SET status=$1,updated_at=$2 WHERE id=$3`, domain.StatusSucceeded, outbox.CreatedAt, id); err != nil {
			return domain.Payment{}, false, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(id,aggregate_id,event_type,payload,created_at) VALUES($1,$2,$3,$4,$5)`, outbox.ID, outbox.AggregateID, outbox.EventType, outbox.Payload, outbox.CreatedAt); err != nil {
			return domain.Payment{}, false, err
		}
		p.Status = domain.StatusSucceeded
		p.UpdatedAt = outbox.CreatedAt
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Payment{}, false, err
	}
	return p, changed, nil
}
