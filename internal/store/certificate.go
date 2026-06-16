package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Certificate represents a TLS certificate for a domain.
type Certificate struct {
	ID        int64      `json:"id"`
	Domain    string     `json:"domain"`
	IssuedAt  *time.Time `json:"issued_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Provider  string     `json:"provider,omitempty"`
	CertPath  string     `json:"cert_path,omitempty"`
	KeyPath   string     `json:"key_path,omitempty"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
}

// CreateCertificate inserts a new certificate record.
func (s *Store) CreateCertificate(ctx context.Context, domain, provider string) (*Certificate, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO certificates (domain, provider, status, created_at)
		 VALUES (?, ?, 'pending', ?)`,
		domain, nullableString(provider), now,
	)
	if err != nil {
		return nil, fmt.Errorf("CreateCertificate: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("CreateCertificate: last insert id: %w", err)
	}

	return &Certificate{
		ID:        id,
		Domain:    domain,
		Provider:  provider,
		Status:    "pending",
		CreatedAt: mustParseTime(now),
	}, nil
}

// GetCertificate returns a certificate by ID.
func (s *Store) GetCertificate(ctx context.Context, id int64) (*Certificate, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, domain, issued_at, expires_at, provider, cert_path, key_path, status, created_at
		 FROM certificates WHERE id = ?`, id,
	)
	return scanCertificate(row)
}

// GetCertificateByDomain returns a certificate by domain name.
func (s *Store) GetCertificateByDomain(ctx context.Context, domain string) (*Certificate, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, domain, issued_at, expires_at, provider, cert_path, key_path, status, created_at
		 FROM certificates WHERE domain = ?`, domain,
	)
	return scanCertificate(row)
}

// ListCertificates returns all certificates, newest first.
func (s *Store) ListCertificates(ctx context.Context) ([]Certificate, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, domain, issued_at, expires_at, provider, cert_path, key_path, status, created_at
		 FROM certificates ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("ListCertificates: %w", err)
	}
	defer rows.Close()

	var certs []Certificate
	for rows.Next() {
		c, err := scanCertificateRow(rows)
		if err != nil {
			return nil, fmt.Errorf("ListCertificates: %w", err)
		}
		certs = append(certs, *c)
	}
	return certs, rows.Err()
}

// UpdateCertificate updates a certificate's fields after issuance.
func (s *Store) UpdateCertificate(ctx context.Context, id int64, status, certPath, keyPath string, issuedAt, expiresAt *time.Time) error {
	var issuedStr, expiresStr *string
	if issuedAt != nil {
		s := issuedAt.UTC().Format(time.RFC3339)
		issuedStr = &s
	}
	if expiresAt != nil {
		s := expiresAt.UTC().Format(time.RFC3339)
		expiresStr = &s
	}

	res, err := s.db.ExecContext(ctx,
		`UPDATE certificates SET status = ?, cert_path = ?, key_path = ?, issued_at = ?, expires_at = ?
		 WHERE id = ?`,
		status, nullableString(certPath), nullableString(keyPath), issuedStr, expiresStr, id,
	)
	if err != nil {
		return fmt.Errorf("UpdateCertificate: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("UpdateCertificate: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("UpdateCertificate: %w", ErrNotFound)
	}
	return nil
}

// ListExpiringCertificates returns certificates expiring before the given time.
func (s *Store) ListExpiringCertificates(ctx context.Context, before time.Time) ([]Certificate, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, domain, issued_at, expires_at, provider, cert_path, key_path, status, created_at
		 FROM certificates WHERE expires_at IS NOT NULL AND expires_at < ?
		 ORDER BY expires_at ASC`,
		before.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("ListExpiringCertificates: %w", err)
	}
	defer rows.Close()

	var certs []Certificate
	for rows.Next() {
		c, err := scanCertificateRow(rows)
		if err != nil {
			return nil, fmt.Errorf("ListExpiringCertificates: %w", err)
		}
		certs = append(certs, *c)
	}
	return certs, rows.Err()
}

func scanCertificate(row *sql.Row) (*Certificate, error) {
	var c Certificate
	var issuedAt, expiresAt, provider, certPath, keyPath sql.NullString
	var createdAt string

	err := row.Scan(&c.ID, &c.Domain, &issuedAt, &expiresAt, &provider,
		&certPath, &keyPath, &c.Status, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("scanCertificate: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("scanCertificate: %w", err)
	}

	c.CreatedAt = mustParseTime(createdAt)
	if issuedAt.Valid {
		t := mustParseTime(issuedAt.String)
		c.IssuedAt = &t
	}
	if expiresAt.Valid {
		t := mustParseTime(expiresAt.String)
		c.ExpiresAt = &t
	}
	if provider.Valid {
		c.Provider = provider.String
	}
	if certPath.Valid {
		c.CertPath = certPath.String
	}
	if keyPath.Valid {
		c.KeyPath = keyPath.String
	}
	return &c, nil
}

func scanCertificateRow(rows *sql.Rows) (*Certificate, error) {
	var c Certificate
	var issuedAt, expiresAt, provider, certPath, keyPath sql.NullString
	var createdAt string

	err := rows.Scan(&c.ID, &c.Domain, &issuedAt, &expiresAt, &provider,
		&certPath, &keyPath, &c.Status, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("scanCertificateRow: %w", err)
	}

	c.CreatedAt = mustParseTime(createdAt)
	if issuedAt.Valid {
		t := mustParseTime(issuedAt.String)
		c.IssuedAt = &t
	}
	if expiresAt.Valid {
		t := mustParseTime(expiresAt.String)
		c.ExpiresAt = &t
	}
	if provider.Valid {
		c.Provider = provider.String
	}
	if certPath.Valid {
		c.CertPath = certPath.String
	}
	if keyPath.Valid {
		c.KeyPath = keyPath.String
	}
	return &c, nil
}
