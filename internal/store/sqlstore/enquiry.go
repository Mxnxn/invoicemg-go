package sqlstore

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
	"github.com/mxnxn/invoicemg-go/internal/textcase"
)

type enquiries struct{ pool *pgxpool.Pool }

func (s *Store) Enquiries() store.Enquiries { return &enquiries{pool: s.pool} }

func (e *enquiries) Create(ctx context.Context, in store.EnquiryWrite) (store.ID, error) {
	name := cap160(textcase.TitleCase(in.Name), 120)
	company := cap160(textcase.TitleCase(in.CompanyName), 160)
	note := in.Note
	if len(note) > 2000 {
		note = note[:2000]
	}
	var id string
	err := e.pool.QueryRow(ctx, `
		INSERT INTO enquiries (name, email, phone, company_name, note, source, user_agent)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		name, strings.ToLower(in.Email), in.Phone, company, textcase.SentenceCase(note), in.Source, in.UserAgent).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert enquiry: %w", err)
	}
	return store.ID(id), nil
}

func cap160(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func (e *enquiries) List(ctx context.Context) ([]store.Enquiry, error) {
	rows, err := e.pool.Query(ctx, `
		SELECT id, name, email, phone, company_name, note, source, user_agent, handled, created_at, updated_at
		  FROM enquiries ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		return nil, fmt.Errorf("list enquiries: %w", err)
	}
	defer rows.Close()
	out := []store.Enquiry{}
	for rows.Next() {
		var q store.Enquiry
		if err := rows.Scan(&q.ID, &q.Name, &q.Email, &q.Phone, &q.CompanyName, &q.Note, &q.Source, &q.UserAgent, &q.Handled, &q.CreatedAt, &q.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (e *enquiries) SetHandled(ctx context.Context, id store.ID, handled bool) error {
	if _, err := e.pool.Exec(ctx, `UPDATE enquiries SET handled=$2, updated_at=now() WHERE id=$1`, string(id), handled); err != nil {
		return fmt.Errorf("set enquiry handled: %w", err)
	}
	return nil
}
