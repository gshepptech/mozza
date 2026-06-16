package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/store"
)

// testStore creates a temporary SQLite store for testing.
func testStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, err := store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpen(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	assert.NotNil(t, s)
}

func TestOpen_InvalidPath(t *testing.T) {
	t.Parallel()
	_, err := store.Open("/nonexistent/path/db.sqlite")
	assert.Error(t, err)
}

func TestUserCRUD(t *testing.T) {
	t.Parallel()
	s := testStore(t)

	// Create.
	u, err := s.CreateUser("alice@example.com", "Alice", "hashedpw", "member")
	require.NoError(t, err)
	assert.NotEmpty(t, u.ID)
	assert.Equal(t, "alice@example.com", u.Email)
	assert.Equal(t, "Alice", u.Name)
	assert.Equal(t, "member", u.Role)

	// Read by ID.
	found, err := s.UserByID(u.ID)
	require.NoError(t, err)
	assert.Equal(t, u.Email, found.Email)

	// Read by email.
	found, err = s.UserByEmail("alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, u.ID, found.ID)

	// Update.
	err = s.UpdateUser(u.ID, "Alice Updated", "admin")
	require.NoError(t, err)
	found, _ = s.UserByID(u.ID)
	assert.Equal(t, "Alice Updated", found.Name)
	assert.Equal(t, "admin", found.Role)

	// List.
	users, err := s.ListUsers()
	require.NoError(t, err)
	assert.Len(t, users, 1)

	// Delete.
	err = s.DeleteUser(u.ID)
	require.NoError(t, err)
	_, err = s.UserByID(u.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestUserDuplicateEmail(t *testing.T) {
	t.Parallel()
	s := testStore(t)

	_, err := s.CreateUser("dup@example.com", "A", "pw", "member")
	require.NoError(t, err)

	_, err = s.CreateUser("dup@example.com", "B", "pw", "member")
	assert.ErrorIs(t, err, store.ErrConflict)
}

func TestSessionCRUD(t *testing.T) {
	t.Parallel()
	s := testStore(t)

	u, _ := s.CreateUser("sess@example.com", "Sess", "pw", "member")

	// Create session.
	sess, err := s.CreateSession(u.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, sess.ID)
	assert.Equal(t, u.ID, sess.UserID)

	// Read session.
	found, err := s.SessionByID(sess.ID)
	require.NoError(t, err)
	assert.Equal(t, sess.ID, found.ID)

	// Delete session.
	err = s.DeleteSession(sess.ID)
	require.NoError(t, err)
	_, err = s.SessionByID(sess.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestTeamCRUD(t *testing.T) {
	t.Parallel()
	s := testStore(t)

	u, _ := s.CreateUser("team@example.com", "TeamUser", "pw", "member")

	// Create team (adds creator as owner).
	team, err := s.CreateTeam("My Team", "my-team", u.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, team.ID)
	assert.Equal(t, "My Team", team.Name)
	assert.Equal(t, "my-team", team.Slug)

	// Read by ID.
	found, err := s.TeamByID(team.ID)
	require.NoError(t, err)
	assert.Equal(t, team.Name, found.Name)

	// Read by slug.
	found, err = s.TeamBySlug("my-team")
	require.NoError(t, err)
	assert.Equal(t, team.ID, found.ID)

	// Teams for user.
	teams, err := s.TeamsForUser(u.ID)
	require.NoError(t, err)
	assert.Len(t, teams, 1)

	// Is member.
	isMember, err := s.IsTeamMember(team.ID, u.ID)
	require.NoError(t, err)
	assert.True(t, isMember)

	// Members list.
	members, err := s.TeamMembers(team.ID)
	require.NoError(t, err)
	assert.Len(t, members, 1)
	assert.Equal(t, "owner", members[0].Role)

	// Add another member.
	u2, _ := s.CreateUser("member@example.com", "Member", "pw", "member")
	err = s.AddTeamMember(team.ID, u2.ID, "member")
	require.NoError(t, err)

	members, _ = s.TeamMembers(team.ID)
	assert.Len(t, members, 2)

	// Remove member.
	err = s.RemoveTeamMember(team.ID, u2.ID)
	require.NoError(t, err)
	members, _ = s.TeamMembers(team.ID)
	assert.Len(t, members, 1)

	// Delete team.
	err = s.DeleteTeam(team.ID)
	require.NoError(t, err)
	_, err = s.TeamByID(team.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestRecipeCRUD(t *testing.T) {
	t.Parallel()
	s := testStore(t)

	u, _ := s.CreateUser("recipe@example.com", "RecipeUser", "pw", "member")
	team, _ := s.CreateTeam("Recipe Team", "recipe-team", u.ID)

	// Create.
	r, err := s.CreateRecipe(team.ID, "my-app", "App: my-app\n", "{}", u.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, r.ID)
	assert.Equal(t, "my-app", r.Name)

	// Read.
	found, err := s.RecipeByID(r.ID)
	require.NoError(t, err)
	assert.Equal(t, r.Source, found.Source)

	// List for team.
	recipes, err := s.RecipesForTeam(team.ID)
	require.NoError(t, err)
	assert.Len(t, recipes, 1)

	// Update.
	err = s.UpdateRecipe(r.ID, "updated-app", "App: updated\n", `{"blocks":[]}`)
	require.NoError(t, err)
	found, _ = s.RecipeByID(r.ID)
	assert.Equal(t, "updated-app", found.Name)

	// Delete.
	err = s.DeleteRecipe(r.ID)
	require.NoError(t, err)
	_, err = s.RecipeByID(r.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestDeploymentCRUD(t *testing.T) {
	t.Parallel()
	s := testStore(t)

	u, _ := s.CreateUser("deploy@example.com", "DeployUser", "pw", "member")
	team, _ := s.CreateTeam("Deploy Team", "deploy-team", u.ID)
	recipe, _ := s.CreateRecipe(team.ID, "deploy-app", "source", "{}", u.ID)

	// Create.
	d, err := s.CreateDeployment(recipe.ID, team.ID, "kubernetes", "production", u.ID, "")
	require.NoError(t, err)
	assert.NotEmpty(t, d.ID)
	assert.Equal(t, "pending", d.Status)

	// Read.
	found, err := s.DeploymentByID(d.ID)
	require.NoError(t, err)
	assert.Equal(t, d.Status, found.Status)

	// List for team.
	deployments, err := s.DeploymentsForTeam(team.ID, 10)
	require.NoError(t, err)
	assert.Len(t, deployments, 1)

	// Update status.
	err = s.UpdateDeploymentStatus(d.ID, "succeeded", "done\n", true)
	require.NoError(t, err)
	found, _ = s.DeploymentByID(d.ID)
	assert.Equal(t, "succeeded", found.Status)
	assert.NotNil(t, found.FinishedAt)

	// Append log.
	err = s.AppendDeploymentLog(d.ID, "extra line\n")
	require.NoError(t, err)
	found, _ = s.DeploymentByID(d.ID)
	assert.Contains(t, found.Log, "extra line")
}

func TestDeleteNonExistent(t *testing.T) {
	t.Parallel()
	s := testStore(t)

	tests := []struct {
		name string
		fn   func() error
	}{
		{"user", func() error { return s.DeleteUser("nonexistent") }},
		{"team", func() error { return s.DeleteTeam("nonexistent") }},
		{"recipe", func() error { return s.DeleteRecipe("nonexistent") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.fn()
			assert.ErrorIs(t, err, store.ErrNotFound)
		})
	}
}

func TestCleanExpiredSessions(t *testing.T) {
	t.Parallel()
	s := testStore(t)

	u, _ := s.CreateUser("clean@example.com", "Clean", "pw", "member")
	_, err := s.CreateSession(u.ID)
	require.NoError(t, err)

	// Clean should not remove non-expired sessions.
	err = s.CleanExpiredSessions()
	require.NoError(t, err)
}

func TestDeleteUserSessions(t *testing.T) {
	t.Parallel()
	s := testStore(t)

	u, _ := s.CreateUser("multi@example.com", "Multi", "pw", "member")
	s1, _ := s.CreateSession(u.ID)
	s2, _ := s.CreateSession(u.ID)

	err := s.DeleteUserSessions(u.ID)
	require.NoError(t, err)

	_, err = s.SessionByID(s1.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
	_, err = s.SessionByID(s2.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestMigrationIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "idempotent.db")

	s1, err := store.Open(path)
	require.NoError(t, err)
	s1.Close()

	// Open again — migrations should be idempotent.
	s2, err := store.Open(path)
	require.NoError(t, err)
	s2.Close()

	// Verify the DB file exists.
	_, err = os.Stat(path)
	assert.NoError(t, err)
}
