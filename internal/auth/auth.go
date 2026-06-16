package auth

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gshepptech/mozza/internal/store"
)

// Service provides authentication operations.
type Service struct {
	store *store.Store
}

// New creates a new auth Service.
func New(s *store.Store) *Service {
	return &Service{store: s}
}

// ErrInvalidCredentials is returned when login credentials are wrong.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrWeakPassword is returned when the password doesn't meet requirements.
var ErrWeakPassword = errors.New("password must be at least 8 characters")

// ErrInvalidEmail is returned when the email format is invalid.
var ErrInvalidEmail = errors.New("invalid email address")

// Register creates a new user account. Returns the user and a session.
func (svc *Service) Register(email, name, password string) (*store.User, *store.Session, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	name = strings.TrimSpace(name)

	if !isValidEmail(email) {
		return nil, nil, ErrInvalidEmail
	}
	if len(password) < 8 {
		return nil, nil, ErrWeakPassword
	}
	if name == "" {
		name = strings.Split(email, "@")[0]
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, nil, fmt.Errorf("Register: %w", err)
	}

	// First user gets admin role; subsequent users get viewer.
	// Use CreateUser first with "viewer", then promote if first user.
	// This avoids TOCTOU race between count check and insert.
	role := "viewer"
	user, err := svc.store.CreateUser(email, name, hash, role)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			return nil, nil, fmt.Errorf("Register: email already registered")
		}
		return nil, nil, fmt.Errorf("Register: %w", err)
	}

	// Promote to admin if this is the only user in the system.
	// Checking AFTER insert avoids TOCTOU race — only one user can be "the only one."
	users, listErr := svc.store.ListUsers()
	if listErr == nil && len(users) == 1 && users[0].ID == user.ID {
		if updateErr := svc.store.UpdateUser(user.ID, user.Name, "admin"); updateErr == nil {
			user.Role = "admin"
			slog.Info("first user promoted to admin", "id", user.ID)
		}
	}

	sess, err := svc.store.CreateSession(user.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("Register: %w", err)
	}

	slog.Info("user registered", "email", email, "id", user.ID, "role", user.Role)
	return user, sess, nil
}

// Login authenticates a user and creates a session.
func (svc *Service) Login(email, password string) (*store.User, *store.Session, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	user, err := svc.store.UserByEmail(email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil, ErrInvalidCredentials
		}
		return nil, nil, fmt.Errorf("Login: %w", err)
	}

	if err := CheckPassword(user.Password, password); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	sess, err := svc.store.CreateSession(user.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("Login: %w", err)
	}

	slog.Info("user logged in", "email", email, "id", user.ID)
	return user, sess, nil
}

// ValidateSession checks if a session token is valid and returns the associated user.
func (svc *Service) ValidateSession(sessionID string) (*store.User, *store.Session, error) {
	sess, err := svc.store.SessionByID(sessionID)
	if err != nil {
		return nil, nil, fmt.Errorf("ValidateSession: %w", err)
	}

	user, err := svc.store.UserByID(sess.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("ValidateSession: %w", err)
	}

	return user, sess, nil
}

// Logout invalidates a session.
func (svc *Service) Logout(sessionID string) error {
	if err := svc.store.DeleteSession(sessionID); err != nil {
		return fmt.Errorf("Logout: %w", err)
	}
	return nil
}

// isValidEmail performs basic email validation.
func isValidEmail(email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	local, domain := parts[0], parts[1]
	if local == "" || domain == "" {
		return false
	}
	if !strings.Contains(domain, ".") {
		return false
	}
	return true
}
