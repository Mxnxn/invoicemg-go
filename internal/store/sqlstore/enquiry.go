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
