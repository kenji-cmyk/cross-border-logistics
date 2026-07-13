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
	if payment.Provider == "" {
		payment.Provider = "mock"
	}
	if payment.ProviderRequestID == "" {
		payment.ProviderRequestID = payment.ID
	}
	_, err := r.pool.Exec(ctx, `INSERT INTO payments (id,order_id,type,amount_vnd,currency,status,payment_url,provider_reference,provider,provider_request_id,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, payment.ID, payment.OrderID, payment.Type, payment.AmountVND, payment.Currency, payment.Status, payment.PaymentURL, payment.ProviderReference, payment.Provider, payment.ProviderRequestID, payment.CreatedAt, payment.UpdatedAt)
	if err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) && pgError.Code == "23505" && pgError.ConstraintName == "payments_one_deposit_per_order_idx" {
			return domain.ErrPaymentConflict
		}
		return fmt.Errorf("create payment: %w", err)
	}
	return nil
}

func (r *Repository) FindByOrderAndType(ctx context.Context, orderID string, t domain.PaymentType) (domain.Payment, error) {
	return findByTwo(ctx, r.pool, orderID, t)
}
func findByTwo(ctx context.Context, q rowQuerier, orderID string, t domain.PaymentType) (domain.Payment, error) {
	var p domain.Payment
	err := q.QueryRow(ctx, `SELECT id::text,order_id::text,type,amount_vnd,currency,status,payment_url,provider_reference,provider,provider_request_id,COALESCE(provider_transaction_id,''),result_code,result_message,succeeded_at,failed_at,created_at,updated_at FROM payments WHERE order_id=$1 AND type=$2`, orderID, t).Scan(&p.ID, &p.OrderID, &p.Type, &p.AmountVND, &p.Currency, &p.Status, &p.PaymentURL, &p.ProviderReference, &p.Provider, &p.ProviderRequestID, &p.ProviderTransID, &p.ResultCode, &p.ResultMessage, &p.SucceededAt, &p.FailedAt, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, domain.ErrPaymentNotFound
	}
	return p, err
}

func (r *Repository) ListByOrder(ctx context.Context, orderID string) ([]domain.Payment, []domain.Refund, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text,order_id::text,type,amount_vnd,currency,status,payment_url,provider_reference,provider,provider_request_id,COALESCE(provider_transaction_id,''),result_code,result_message,succeeded_at,failed_at,created_at,updated_at FROM payments WHERE order_id=$1 ORDER BY created_at`, orderID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var ps []domain.Payment
	for rows.Next() {
		var p domain.Payment
		if err = rows.Scan(&p.ID, &p.OrderID, &p.Type, &p.AmountVND, &p.Currency, &p.Status, &p.PaymentURL, &p.ProviderReference, &p.Provider, &p.ProviderRequestID, &p.ProviderTransID, &p.ResultCode, &p.ResultMessage, &p.SucceededAt, &p.FailedAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, nil, err
		}
		ps = append(ps, p)
	}
	rr, err := r.pool.Query(ctx, `SELECT id::text,payment_id::text,order_id::text,amount_vnd,status,provider,provider_request_id,COALESCE(provider_transaction_id,''),result_code,result_message,created_at,updated_at,succeeded_at,failed_at FROM refunds WHERE order_id=$1 ORDER BY created_at`, orderID)
	if err != nil {
		return nil, nil, err
	}
	defer rr.Close()
	var rs []domain.Refund
	for rr.Next() {
		var x domain.Refund
		if err = rr.Scan(&x.ID, &x.PaymentID, &x.OrderID, &x.AmountVND, &x.Status, &x.Provider, &x.ProviderRequestID, &x.ProviderTransID, &x.ResultCode, &x.ResultMessage, &x.CreatedAt, &x.UpdatedAt, &x.SucceededAt, &x.FailedAt); err != nil {
			return nil, nil, err
		}
		rs = append(rs, x)
	}
	return ps, rs, nil
}

func (r *Repository) CompleteProviderResult(ctx context.Context, id, transID string, code int, message string, success bool, outbox ports.OutboxEvent) (domain.Payment, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Payment{}, false, err
	}
	defer tx.Rollback(ctx)
	p, err := findByID(ctx, tx, id, true)
	if err != nil {
		return p, false, err
	}
	if p.Status != domain.StatusPending {
		return p, false, tx.Commit(ctx)
	}
	status := domain.StatusFailed
	if success {
		status = domain.StatusSucceeded
	}
	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `UPDATE payments SET status=$1,provider_transaction_id=$2,result_code=$3,result_message=$4,succeeded_at=CASE WHEN $5 THEN $6 END,failed_at=CASE WHEN NOT $5 THEN $6 END,updated_at=$6 WHERE id=$7`, status, transID, code, message, success, now, id)
	if err != nil {
		return p, false, err
	}
	if success {
		if _, err = tx.Exec(ctx, `INSERT INTO outbox_events(id,aggregate_id,event_type,payload,created_at) VALUES($1,$2,$3,$4,$5)`, outbox.ID, outbox.AggregateID, outbox.EventType, outbox.Payload, outbox.CreatedAt); err != nil {
			return p, false, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return p, false, err
	}
	p.Status = status
	p.ProviderTransID = transID
	p.ResultCode = &code
	p.ResultMessage = message
	p.UpdatedAt = now
	if success {
		p.SucceededAt = &now
	} else {
		p.FailedAt = &now
	}
	return p, true, nil
}

func (r *Repository) CreateRefund(ctx context.Context, x domain.Refund) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO refunds(id,payment_id,order_id,amount_vnd,status,provider,provider_request_id,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(payment_id) DO NOTHING`, x.ID, x.PaymentID, x.OrderID, x.AmountVND, x.Status, x.Provider, x.ProviderRequestID, x.CreatedAt, x.UpdatedAt)
	return err
}
func (r *Repository) CompleteRefund(ctx context.Context, id, transID string, code int, message string, success bool, outbox ports.OutboxEvent) (domain.Refund, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Refund{}, false, err
	}
	defer tx.Rollback(ctx)
	var x domain.Refund
	err = tx.QueryRow(ctx, `SELECT id::text,payment_id::text,order_id::text,amount_vnd,status,provider,provider_request_id,created_at,updated_at FROM refunds WHERE id=$1 FOR UPDATE`, id).Scan(&x.ID, &x.PaymentID, &x.OrderID, &x.AmountVND, &x.Status, &x.Provider, &x.ProviderRequestID, &x.CreatedAt, &x.UpdatedAt)
	if err != nil {
		return x, false, err
	}
	if x.Status != domain.StatusPending {
		return x, false, tx.Commit(ctx)
	}
	status := domain.StatusFailed
	if success {
		status = domain.StatusSucceeded
	}
	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `UPDATE refunds SET status=$1,provider_transaction_id=$2,result_code=$3,result_message=$4,succeeded_at=CASE WHEN $5 THEN $6 END,failed_at=CASE WHEN NOT $5 THEN $6 END,updated_at=$6 WHERE id=$7`, status, transID, code, message, success, now, id)
	if err != nil {
		return x, false, err
	}
	if success {
		_, err = tx.Exec(ctx, `INSERT INTO outbox_events(id,aggregate_id,event_type,payload,created_at) VALUES($1,$2,$3,$4,$5)`, outbox.ID, outbox.AggregateID, outbox.EventType, outbox.Payload, outbox.CreatedAt)
		if err != nil {
			return x, false, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return x, false, err
	}
	x.Status = status
	x.ProviderTransID = transID
	x.ResultCode = &code
	x.ResultMessage = message
	x.UpdatedAt = now
	return x, true, nil
}
func (r *Repository) ListPending(ctx context.Context, before time.Time) ([]domain.Payment, []domain.Refund, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text FROM payments WHERE status='PENDING' AND updated_at<$1 ORDER BY updated_at LIMIT 100`, before)
	if err != nil {
		return nil, nil, err
	}
	var ps []domain.Payment
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			p, e := r.FindByID(ctx, id)
			if e == nil {
				ps = append(ps, p)
			}
		}
	}
	rows.Close()
	rr, err := r.pool.Query(ctx, `SELECT id::text,payment_id::text,order_id::text,amount_vnd,status,provider,provider_request_id,COALESCE(provider_transaction_id,''),result_code,result_message,created_at,updated_at,succeeded_at,failed_at FROM refunds WHERE status='PENDING' AND updated_at<$1 ORDER BY updated_at LIMIT 100`, before)
	if err != nil {
		return nil, nil, err
	}
	defer rr.Close()
	var rs []domain.Refund
	for rr.Next() {
		var x domain.Refund
		if err = rr.Scan(&x.ID, &x.PaymentID, &x.OrderID, &x.AmountVND, &x.Status, &x.Provider, &x.ProviderRequestID, &x.ProviderTransID, &x.ResultCode, &x.ResultMessage, &x.CreatedAt, &x.UpdatedAt, &x.SucceededAt, &x.FailedAt); err != nil {
			return nil, nil, err
		}
		rs = append(rs, x)
	}
	return ps, rs, nil
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
	err := scanPayment(q.QueryRow(ctx, `SELECT id::text,order_id::text,type,amount_vnd,currency,status,payment_url,provider_reference,provider,provider_request_id,COALESCE(provider_transaction_id,''),result_code,result_message,succeeded_at,failed_at,created_at,updated_at FROM payments WHERE `+column+`=$1`, value), &p)
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
	query := `SELECT id::text,order_id::text,type,amount_vnd,currency,status,payment_url,provider_reference,provider,provider_request_id,COALESCE(provider_transaction_id,''),result_code,result_message,succeeded_at,failed_at,created_at,updated_at FROM payments WHERE id=$1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var payment domain.Payment
	err := scanPayment(queryer.QueryRow(ctx, query, id), &payment)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payment{}, domain.ErrPaymentNotFound
	}
	if err != nil {
		return domain.Payment{}, fmt.Errorf("find payment: %w", err)
	}
	return payment, nil
}

func scanPayment(row pgx.Row, p *domain.Payment) error {
	return row.Scan(&p.ID, &p.OrderID, &p.Type, &p.AmountVND, &p.Currency, &p.Status, &p.PaymentURL, &p.ProviderReference, &p.Provider, &p.ProviderRequestID, &p.ProviderTransID, &p.ResultCode, &p.ResultMessage, &p.SucceededAt, &p.FailedAt, &p.CreatedAt, &p.UpdatedAt)
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
