package stack

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"sort"

	"filippo.io/age"
	"gopkg.in/yaml.v3"
)

// defaultLengths are generator sizes when a stack does not give one: 32 bytes
// of entropy for keys, 24 characters for a password a person may type.
var defaultLengths = map[string]int{ //nolint:gochecknoglobals // immutable lookup table
	"hex":      32,
	"base64":   32,
	"password": 24,
}

const passwordAlphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// generate produces a fresh value for a generator.
func generate(kind string, length int) (string, error) {
	if length == 0 {
		length = defaultLengths[kind]
	}
	switch kind {
	case "hex", "base64":
		b := make([]byte, length)
		if _, err := rand.Read(b); err != nil {
			return "", fmt.Errorf("generate %s: %w", kind, err)
		}
		if kind == "hex" {
			return hex.EncodeToString(b), nil
		}
		return base64.StdEncoding.EncodeToString(b), nil
	case "password":
		out := make([]byte, length)
		limit := big.NewInt(int64(len(passwordAlphabet)))
		for i := range out {
			n, err := rand.Int(rand.Reader, limit)
			if err != nil {
				return "", fmt.Errorf("generate password: %w", err)
			}
			out[i] = passwordAlphabet[n.Int64()]
		}
		return string(out), nil
	case "age-key":
		id, err := age.GenerateX25519Identity()
		if err != nil {
			return "", fmt.Errorf("generate age-key: %w", err)
		}
		return id.String(), nil
	default:
		return "", fmt.Errorf("unknown generator %q", kind)
	}
}

// secretStore persists generated values per stack, keyed by
// namespace/name/key. A value is generated once and reused on every later
// `up`: an app that encrypted its data with a master key, or stored a database
// password, breaks if a re-run rotates the value underneath it.
type secretStore struct {
	path   string
	values map[string]string
}

func openSecretStore(dir string) (*secretStore, error) {
	s := &secretStore{path: filepath.Join(dir, "secrets.json"), values: map[string]string{}}
	b, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read secrets: %w", err)
	}
	if err := json.Unmarshal(b, &s.values); err != nil {
		return nil, fmt.Errorf("read secrets %s: %w", s.path, err)
	}
	return s, nil
}

func (s *secretStore) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("save secrets: %w", err)
	}
	b, err := json.MarshalIndent(s.values, "", "  ")
	if err != nil {
		return fmt.Errorf("save secrets: %w", err)
	}
	if err := os.WriteFile(s.path, b, 0o600); err != nil {
		return fmt.Errorf("save secrets: %w", err)
	}
	return nil
}

func secretKey(sec Secret, key string) string {
	return sec.Namespace + "/" + sec.Name + "/" + key
}

// resolve returns every key's value for one Secret, generating and recording
// any that are missing. fromEnv values are read fresh and never stored.
func (s *secretStore) resolve(sec Secret) (map[string]string, error) {
	out := make(map[string]string, len(sec.Data))
	for key, v := range sec.Data {
		switch {
		case v.Value != "":
			out[key] = v.Value
		case v.FromEnv != "":
			val, ok := os.LookupEnv(v.FromEnv)
			if !ok {
				return nil, fmt.Errorf("secret %s/%s key %s: environment variable %s is not set",
					sec.Namespace, sec.Name, key, v.FromEnv)
			}
			out[key] = val
		default:
			id := secretKey(sec, key)
			if existing, ok := s.values[id]; ok {
				out[key] = existing
				continue
			}
			val, err := generate(v.Generate, v.Length)
			if err != nil {
				return nil, err
			}
			s.values[id] = val
			out[key] = val
		}
	}
	return out, nil
}

// ShownSecret is a value a stack marked for display.
type ShownSecret struct {
	Ref   string
	Value string
}

// shown lists the values a person needs, in a stable order. A value that has
// not been generated yet is reported as such rather than invented.
func (s *secretStore) shown(secrets []Secret) []ShownSecret {
	var out []ShownSecret
	for _, sec := range secrets {
		keys := make([]string, 0, len(sec.Data))
		for k := range sec.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := sec.Data[k]
			if !v.Show {
				continue
			}
			val := v.Value
			if v.Generate != "" {
				val = s.values[secretKey(sec, k)]
			}
			if v.FromEnv != "" {
				val = "(from $" + v.FromEnv + ")"
			}
			if val == "" {
				val = "(not generated yet — run `mozza stack up`)"
			}
			out = append(out, ShownSecret{Ref: secretKey(sec, k), Value: val})
		}
	}
	return out
}

// secretManifest renders a Secret as YAML for `kubectl apply -f -`, so the
// values travel on stdin and never appear in a process argument list.
func secretManifest(sec Secret, data map[string]string) ([]byte, error) {
	typ := sec.Type
	if typ == "" {
		typ = "Opaque"
	}
	obj := map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name":      sec.Name,
			"namespace": sec.Namespace,
			"labels":    map[string]any{"app.kubernetes.io/managed-by": "mozza-stack"},
		},
		"type":       typ,
		"stringData": data,
	}
	b, err := yaml.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("render secret %s: %w", sec.Name, err)
	}
	return b, nil
}
