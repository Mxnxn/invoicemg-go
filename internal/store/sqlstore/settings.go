package sqlstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type settings struct{ pool *pgxpool.Pool }

func (s *Store) Settings() store.Settings { return &settings{pool: s.pool} }

// settingsColumn maps a section to its fixed column name. It is a whitelist, not string
// interpolation of caller input: the returned value is one of two literals, so building the
// query text around it cannot inject SQL.
func settingsColumn(section store.SettingsSection) (string, error) {
	switch section {
	case store.SettingsAppearance:
		return "appearance", nil
	case store.SettingsTables:
		return "tables", nil
	default:
		return "", fmt.Errorf("unknown settings section %q", section)
	}
}

// Get reads one preference blob. No row is (nil, nil) - the first-login case Node answers with
// data:null. A row always has both columns (DEFAULT '{}'), so a never-written section reads as
// {} rather than null.
func (s *settings) Get(ctx context.Context, ownerID store.ID, section store.SettingsSection) (json.RawMessage, error) {
	col, err := settingsColumn(section)
	if err != nil {
		return nil, err
	}
	var raw []byte
	err = s.pool.QueryRow(ctx,
		`SELECT `+col+` FROM user_settings WHERE owner_id = $1`, string(ownerID)).
		Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading user settings: %w", err)
	}
	return json.RawMessage(raw), nil
}

// Set upserts one blob and returns the stored value. On the INSERT that creates the row the
// other column keeps its DEFAULT '{}', so a later read of it is {} not null - the relational
// twin of Mongoose's setDefaultsOnInsert. ON CONFLICT touches only this column and updated_at.
func (s *settings) Set(ctx context.Context, ownerID store.ID, section store.SettingsSection, value json.RawMessage) (json.RawMessage, error) {
	col, err := settingsColumn(section)
	if err != nil {
		return nil, err
	}
	var raw []byte
	err = s.pool.QueryRow(ctx,
		`INSERT INTO user_settings (owner_id, `+col+`) VALUES ($1, $2::jsonb)
		 ON CONFLICT (owner_id) DO UPDATE SET `+col+` = EXCLUDED.`+col+`, updated_at = now()
		 RETURNING `+col,
		string(ownerID), string(value)).
		Scan(&raw)
	if err != nil {
		return nil, fmt.Errorf("saving user settings: %w", err)
	}
	return json.RawMessage(raw), nil
}
