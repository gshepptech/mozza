package auth_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/auth"
	"github.com/gshepptech/mozza/internal/store"
)

func testService(t *testing.T) *auth.Service {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return auth.New(s)
}

func TestRegister(t *testing.T) {
	t.Parallel()
	svc := testService(t)

	user, sess, err := svc.Register("alice@example.com", "Alice", "password123")
	require.NoError(t, err)
	assert.NotEmpty(t, user.ID)
	assert.Equal(t, "alice@example.com", user.Email)
	assert.Equal(t, "Alice", user.Name)
	assert.NotEmpty(t, sess.ID)
}

func TestRegister_WeakPassword(t *testing.T) {
	t.Parallel()
	svc := testService(t)

	_, _, err := svc.Register("weak@example.com", "Weak", "short")
	assert.ErrorIs(t, err, auth.ErrWeakPassword)
}

func TestRegister_InvalidEmail(t *testing.T) {
	t.Parallel()
	svc := testService(t)

	_, _, err := svc.Register("not-an-email", "Bad", "password123")
	assert.ErrorIs(t, err, auth.ErrInvalidEmail)
}

func TestRegister_DuplicateEmail(t *testing.T) {
	t.Parallel()
	svc := testService(t)

	_, _, err := svc.Register("dup@example.com", "First", "password123")
	require.NoError(t, err)

	_, _, err = svc.Register("dup@example.com", "Second", "password123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegister_EmptyName(t *testing.T) {
	t.Parallel()
	svc := testService(t)

	user, _, err := svc.Register("noname@example.com", "", "password123")
	require.NoError(t, err)
	assert.Equal(t, "noname", user.Name)
}

func TestLogin(t *testing.T) {
	t.Parallel()
	svc := testService(t)

	_, _, err := svc.Register("login@example.com", "Login", "password123")
	require.NoError(t, err)

	user, sess, err := svc.Login("login@example.com", "password123")
	require.NoError(t, err)
	assert.Equal(t, "login@example.com", user.Email)
	assert.NotEmpty(t, sess.ID)
}

func TestLogin_WrongPassword(t *testing.T) {
	t.Parallel()
	svc := testService(t)

	_, _, err := svc.Register("wrong@example.com", "Wrong", "password123")
	require.NoError(t, err)

	_, _, err = svc.Login("wrong@example.com", "wrongpassword")
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestLogin_NonexistentUser(t *testing.T) {
	t.Parallel()
	svc := testService(t)

	_, _, err := svc.Login("ghost@example.com", "password123")
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestValidateSession(t *testing.T) {
	t.Parallel()
	svc := testService(t)

	_, sess, err := svc.Register("validate@example.com", "Validate", "password123")
	require.NoError(t, err)

	user, foundSess, err := svc.ValidateSession(sess.ID)
	require.NoError(t, err)
	assert.Equal(t, "validate@example.com", user.Email)
	assert.Equal(t, sess.ID, foundSess.ID)
}

func TestValidateSession_Invalid(t *testing.T) {
	t.Parallel()
	svc := testService(t)

	_, _, err := svc.ValidateSession("nonexistent-session")
	assert.Error(t, err)
}

func TestLogout(t *testing.T) {
	t.Parallel()
	svc := testService(t)

	_, sess, err := svc.Register("logout@example.com", "Logout", "password123")
	require.NoError(t, err)

	err = svc.Logout(sess.ID)
	require.NoError(t, err)

	_, _, err = svc.ValidateSession(sess.ID)
	assert.Error(t, err)
}

func TestHashPassword(t *testing.T) {
	t.Parallel()

	hash, err := auth.HashPassword("testpassword")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "testpassword", hash)

	err = auth.CheckPassword(hash, "testpassword")
	assert.NoError(t, err)

	err = auth.CheckPassword(hash, "wrongpassword")
	assert.Error(t, err)
}

func TestLogin_CaseInsensitiveEmail(t *testing.T) {
	t.Parallel()
	svc := testService(t)

	_, _, err := svc.Register("Case@Example.COM", "Case", "password123")
	require.NoError(t, err)

	user, _, err := svc.Login("case@example.com", "password123")
	require.NoError(t, err)
	assert.Equal(t, "case@example.com", user.Email)
}
