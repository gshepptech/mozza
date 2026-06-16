package store

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/gshepptech/mozza/internal/crypto"
)

// Cluster represents a registered Kubernetes cluster with its kubeconfig.
type Cluster struct {
	ID         string
	Name       string
	Kubeconfig string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ClusterSummary is a Cluster without the kubeconfig field.
type ClusterSummary struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateCluster inserts a new cluster record. If an encryption key is set,
// the kubeconfig is encrypted at rest using AES-256-GCM.
func (s *Store) CreateCluster(name, kubeconfig string) (*Cluster, error) {
	now := time.Now().UTC()
	c := &Cluster{
		ID:         uuid.New().String(),
		Name:       name,
		Kubeconfig: kubeconfig,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	stored := kubeconfig
	if s.encryptionKey != nil {
		encrypted, err := crypto.Encrypt(s.encryptionKey, []byte(kubeconfig))
		if err != nil {
			return nil, fmt.Errorf("CreateCluster: encrypt: %w", err)
		}
		stored = base64.StdEncoding.EncodeToString(encrypted)
	}

	_, err := s.db.Exec(
		`INSERT INTO clusters (id, name, kubeconfig, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		c.ID, c.Name, stored,
		c.CreatedAt.Format(time.RFC3339), c.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("CreateCluster: %w", ErrConflict)
		}
		return nil, fmt.Errorf("CreateCluster: %w", err)
	}

	return c, nil
}

// ClusterKubeconfig returns the decrypted kubeconfig for a cluster.
func (s *Store) ClusterKubeconfig(id string) (string, error) {
	var stored string
	err := s.db.QueryRow(`SELECT kubeconfig FROM clusters WHERE id = ?`, id).Scan(&stored)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("ClusterKubeconfig: %w", ErrNotFound)
	}
	if err != nil {
		return "", fmt.Errorf("ClusterKubeconfig: %w", err)
	}

	if s.encryptionKey == nil {
		return stored, nil
	}

	ciphertext, err := base64.StdEncoding.DecodeString(stored)
	if err != nil {
		return stored, nil // Not encrypted, return as-is
	}

	plaintext, err := crypto.Decrypt(s.encryptionKey, ciphertext)
	if err != nil {
		return stored, nil // Decryption failed, might be plaintext
	}

	return string(plaintext), nil
}

// ListClusters returns all clusters without their kubeconfig.
func (s *Store) ListClusters() ([]ClusterSummary, error) {
	rows, err := s.db.Query(
		`SELECT id, name, created_at, updated_at FROM clusters ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("ListClusters: %w", err)
	}
	defer rows.Close()

	var clusters []ClusterSummary
	for rows.Next() {
		var c ClusterSummary
		var createdAt, updatedAt string
		if err := rows.Scan(&c.ID, &c.Name, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("ListClusters: %w", err)
		}
		c.CreatedAt = mustParseTime(createdAt)
		c.UpdatedAt = mustParseTime(updatedAt)
		clusters = append(clusters, c)
	}
	return clusters, rows.Err()
}

// ClusterByID returns a cluster by ID without the kubeconfig.
func (s *Store) ClusterByID(id string) (*ClusterSummary, error) {
	var c ClusterSummary
	var createdAt, updatedAt string
	err := s.db.QueryRow(
		`SELECT id, name, created_at, updated_at FROM clusters WHERE id = ?`, id,
	).Scan(&c.ID, &c.Name, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("ClusterByID: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("ClusterByID: %w", err)
	}
	c.CreatedAt = mustParseTime(createdAt)
	c.UpdatedAt = mustParseTime(updatedAt)
	return &c, nil
}

// DeleteCluster removes a cluster by ID.
func (s *Store) DeleteCluster(id string) error {
	res, err := s.db.Exec(`DELETE FROM clusters WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("DeleteCluster: %w", err)
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("DeleteCluster: rows affected: %w", raErr)
	}
	if n == 0 {
		return fmt.Errorf("DeleteCluster: %w", ErrNotFound)
	}
	return nil
}
