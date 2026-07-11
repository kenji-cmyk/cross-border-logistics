package postgres

import (
	"context"
	"errors"
	"fmt"

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
	_, err := r.pool.Exec(ctx, `INSERT INTO quotations (id,customer_id,product_url,product_name,source_price,currency,quantity,exchange_rate,product_amount_vnd,service_fee_vnd,estimated_shipping_fee_vnd,total_amount_vnd,status,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`, q.ID, q.CustomerID, q.ProductURL, q.ProductName, domain.FormatSourcePrice(q.SourcePriceMicros), q.Currency, q.Quantity, q.ExchangeRate, q.ProductAmountVND, q.ServiceFeeVND, q.EstimatedShippingFeeVND, q.TotalAmountVND, q.Status, q.CreatedAt, q.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create quotation: %w", err)
	}
	return nil
}

func (r *Repository) FindByID(ctx context.Context, id string) (domain.Quotation, error) {
	var q domain.Quotation
	var price string
	err := r.pool.QueryRow(ctx, `SELECT id::text,customer_id,product_url,product_name,source_price::text,currency,quantity,exchange_rate,product_amount_vnd,service_fee_vnd,estimated_shipping_fee_vnd,total_amount_vnd,status,created_at,updated_at FROM quotations WHERE id=$1`, id).Scan(&q.ID, &q.CustomerID, &q.ProductURL, &q.ProductName, &price, &q.Currency, &q.Quantity, &q.ExchangeRate, &q.ProductAmountVND, &q.ServiceFeeVND, &q.EstimatedShippingFeeVND, &q.TotalAmountVND, &q.Status, &q.CreatedAt, &q.UpdatedAt)
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
