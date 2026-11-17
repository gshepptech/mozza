package store_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/store"
)

const testKubeconfig = "apiVersion: v1\nkind: Config\nclusters: []\n"

// openStoreAt opens a store on path, with key set when it is non-nil.
func openStoreAt(t *testing.T, path string, key []byte) *store.Store {
	t.Helper()
	s, err := store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	if key != nil {
		s.SetEncryptionKey(key)
	}
	return s
}

func TestClusterKubeconfig(t *testing.T) {
	t.Parallel()
	keyA := bytes.Repeat([]byte{0xA}, 32)
	keyB := bytes.Repeat([]byte{0xB}, 32)

	tests := []struct {
		name     string
		writeKey []byte
		readKey  []byte
		wantErr  error
	}{
		{name: "plaintext without a key", writeKey: nil, readKey: nil},
		{name: "encrypted with the same key", writeKey: keyA, readKey: keyA},
		{name: "plaintext read with a key", writeKey: nil, readKey: keyA},
		{name: "encrypted read with a different key", writeKey: keyA, readKey: keyB, wantErr: store.ErrKubeconfigDecrypt},
		{name: "encrypted read without a key", writeKey: keyA, readKey: nil, wantErr: store.ErrKubeconfigEncrypted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "test.db")
			c, err := openStoreAt(t, path, tt.writeKey).CreateCluster("prod", testKubeconfig)
			require.NoError(t, err)

			got, err := openStoreAt(t, path, tt.readKey).ClusterKubeconfig(c.ID)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testKubeconfig, got)
		})
	}
}

// A plaintext kubeconfig written before a key was configured is encrypted the
// first time it is read with a key.
func TestClusterKubeconfig_EncryptsPlaintextRowOnRead(t *testing.T) {
	t.Parallel()
	key := bytes.Repeat([]byte{0xA}, 32)
	path := filepath.Join(t.TempDir(), "test.db")

	c, err := openStoreAt(t, path, nil).CreateCluster("prod", testKubeconfig)
	require.NoError(t, err)

	got, err := openStoreAt(t, path, key).ClusterKubeconfig(c.ID)
	require.NoError(t, err)
	assert.Equal(t, testKubeconfig, got)

	_, err = openStoreAt(t, path, nil).ClusterKubeconfig(c.ID)
	require.ErrorIs(t, err, store.ErrKubeconfigEncrypted, "the row should now hold ciphertext")

	got, err = openStoreAt(t, path, key).ClusterKubeconfig(c.ID)
	require.NoError(t, err)
	assert.Equal(t, testKubeconfig, got)
}
