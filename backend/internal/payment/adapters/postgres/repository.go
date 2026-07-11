package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/example/cross-border-logistics/internal/payment/domain"
	"github.com/example/cross-border-logistics/internal/payment/ports"
	paymentmigration "github.com/example/cross-border-logistics/migrations/payment"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

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
