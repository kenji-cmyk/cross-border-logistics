package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/example/cross-border-logistics/internal/quotation/domain"
	quotationmigration "github.com/example/cross-border-logistics/migrations/quotation"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, quotationmigration.SQL); err != nil {
		return fmt.Errorf("migrate quotations: %w", err)
	}
	return nil
}

func (r *Repository) Create(ctx context.Context, q domain.Quotation) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO quotations (id,customer_id,product_url,product_name,image_url,source_price,currency,quantity,exchange_rate,product_amount_vnd,service_fee_vnd,estimated_shipping_fee_vnd,total_amount_vnd,status,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`, q.ID, q.CustomerID, q.ProductURL, q.ProductName, q.ImageURL, domain.FormatSourcePrice(q.SourcePriceMicros), q.Currency, q.Quantity, q.ExchangeRate, q.ProductAmountVND, q.ServiceFeeVND, q.EstimatedShippingFeeVND, q.TotalAmountVND, q.Status, q.CreatedAt, q.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create quotation: %w", err)
	}
	return nil
}

func (r *Repository) FindByID(ctx context.Context, id string) (domain.Quotation, error) {
	var q domain.Quotation
	var price string
	err := r.pool.QueryRow(ctx, `SELECT id::text,customer_id,product_url,product_name,image_url,source_price::text,currency,quantity,exchange_rate,product_amount_vnd,service_fee_vnd,estimated_shipping_fee_vnd,total_amount_vnd,status,created_at,updated_at FROM quotations WHERE id=$1`, id).Scan(&q.ID, &q.CustomerID, &q.ProductURL, &q.ProductName, &q.ImageURL, &price, &q.Currency, &q.Quantity, &q.ExchangeRate, &q.ProductAmountVND, &q.ServiceFeeVND, &q.EstimatedShippingFeeVND, &q.TotalAmountVND, &q.Status, &q.CreatedAt, &q.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Quotation{}, domain.ErrQuotationNotFound
	}
	if err != nil {
		return domain.Quotation{}, fmt.Errorf("find quotation: %w", err)
	}
	q.SourcePriceMicros, err = domain.ParseSourcePrice(price)
	if err != nil {
		return domain.Quotation{}, fmt.Errorf("decode quotation source price: %w", err)
	}
	return q, nil
}

func (r *Repository) Confirm(ctx context.Context, id, orderID string, at time.Time) (domain.Quotation, error) {
	var existing string
	err := r.pool.QueryRow(ctx, `UPDATE quotations SET status='CONFIRMED',confirmed_order_id=$2,updated_at=$3 WHERE id=$1 AND (status='PENDING_CONFIRMATION' OR (status='CONFIRMED' AND confirmed_order_id=$2)) RETURNING COALESCE(confirmed_order_id::text,'')`, id, orderID, at).Scan(&existing)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Quotation{}, domain.ErrQuotationConflict
	}
	if err != nil {
		return domain.Quotation{}, fmt.Errorf("confirm quotation: %w", err)
	}
	return r.FindByID(ctx, id)
}
