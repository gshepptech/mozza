package plan

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/recipe"
)

func TestBuild_EnvPassthrough(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		wantEnv map[string]string
	}{
		{
			name:    "single env var",
			env:     map[string]string{"DATABASE_URL": "postgres://localhost/db"},
			wantEnv: map[string]string{"DATABASE_URL": "postgres://localhost/db"},
		},
		{
			name: "multiple env vars",
			env: map[string]string{
				"PORT":     "3000",
				"NODE_ENV": "production",
				"API_KEY":  "secret-123",
			},
			wantEnv: map[string]string{
				"PORT":     "3000",
				"NODE_ENV": "production",
				"API_KEY":  "secret-123",
			},
		},
		{
			name:    "nil env",
			env:     nil,
			wantEnv: nil,
		},
		{
			name:    "empty env",
			env:     map[string]string{},
			wantEnv: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &recipe.Recipe{
				Name: "env-test",
				Slices: []recipe.Slice{
					{
						Name:  "api",
						Image: "api:latest",
						Port:  8080,
						Env:   tt.env,
					},
				},
			}

			plan, err := Build(r)
			require.NoError(t, err)
			require.Len(t, plan.Slices, 1)

			assert.Equal(t, tt.wantEnv, plan.Slices[0].Env)
		})
	}
}

func TestBuild_AutoWirePostgres(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "my-app",
		Slices: []recipe.Slice{
			{Name: "api", Image: "api:latest", Port: 8080, Needs: []string{"db"}},
			{Name: "db", Engine: "postgres", Version: "16"},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	api := plan.Slices[0]
	require.NotNil(t, api.Env)
	assert.Equal(t, "postgres://postgres:mozza@db:5432/db", api.Env["DATABASE_URL"])
}

func TestBuild_AutoWireRedis(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "my-app",
		Slices: []recipe.Slice{
			{Name: "api", Image: "api:latest", Port: 8080, Needs: []string{"cache"}},
			{Name: "cache", Engine: "redis", Version: "7"},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	api := plan.Slices[0]
	require.NotNil(t, api.Env)
	assert.Equal(t, "redis://cache:6379", api.Env["REDIS_URL"])
}

func TestBuild_AutoWireMongo(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "my-app",
		Slices: []recipe.Slice{
			{Name: "api", Image: "api:latest", Port: 8080, Needs: []string{"mongo"}},
			{Name: "mongo", Engine: "mongo", Version: "7"},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	api := plan.Slices[0]
	require.NotNil(t, api.Env)
	assert.Equal(t, "mongodb://mozza:mozza@mongo:27017/mongo?authSource=admin", api.Env["MONGO_URL"])
}

func TestBuild_AutoWireMySQL(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "my-app",
		Slices: []recipe.Slice{
			{Name: "api", Image: "api:latest", Port: 8080, Needs: []string{"mydb"}},
			{Name: "mydb", Engine: "mysql", Version: "8"},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	api := plan.Slices[0]
	require.NotNil(t, api.Env)
	assert.Equal(t, "mysql://root:mozza@mydb:3306/mydb", api.Env["DATABASE_URL"])
}

func TestBuild_AutoWireMemcached(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "my-app",
		Slices: []recipe.Slice{
			{Name: "api", Image: "api:latest", Port: 8080, Needs: []string{"mc"}},
			{Name: "mc", Engine: "memcached", Version: "1.6"},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	api := plan.Slices[0]
	require.NotNil(t, api.Env)
	assert.Equal(t, "mc:11211", api.Env["MEMCACHED_URL"])
}

func TestBuild_AutoWireGenericService(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "my-app",
		Slices: []recipe.Slice{
			{Name: "frontend", Image: "fe:latest", Port: 3000, Public: true, Needs: []string{"auth-service"}},
			{Name: "auth-service", Image: "auth:latest", Port: 9090},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	fe := plan.Slices[0]
	require.NotNil(t, fe.Env)
	assert.Equal(t, "http://auth-service:9090", fe.Env["AUTH_SERVICE_URL"])
}

func TestBuild_AutoWireDoesNotOverrideExisting(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "my-app",
		Slices: []recipe.Slice{
			{
				Name:  "api",
				Image: "api:latest",
				Port:  8080,
				Needs: []string{"db"},
				Env:   map[string]string{"DATABASE_URL": "postgres://custom:5432/custom"},
			},
			{Name: "db", Engine: "postgres", Version: "16"},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	api := plan.Slices[0]
	assert.Equal(t, "postgres://custom:5432/custom", api.Env["DATABASE_URL"])
}
