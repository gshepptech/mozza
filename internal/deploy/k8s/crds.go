package k8s

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
)

// crdFetchTimeout is the maximum time to fetch a CRD YAML from a URL.
const crdFetchTimeout = 30 * time.Second

// ApplyCRDs fetches CRD YAML files from the given URLs and applies them
// to the cluster. CRDs are cluster-scoped and applied before any namespaced
// resources so that custom resource types are available for workloads.
func ApplyCRDs(ctx context.Context, urls []string, progress func(string)) error {
	if len(urls) == 0 {
		return nil
	}

	cfg, _, err := clientConfig("")
	if err != nil {
		return fmt.Errorf("ApplyCRDs: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("ApplyCRDs: dynamic client: %w", err)
	}

	for _, url := range urls {
		if progress != nil {
			progress(fmt.Sprintf("Applying CRDs from %s", url))
		}

		docs, err := fetchYAML(ctx, url)
		if err != nil {
			return fmt.Errorf("ApplyCRDs: fetch %s: %w", url, err)
		}

		applied := 0
		for _, doc := range docs {
			obj := &unstructured.Unstructured{}
			if err := obj.UnmarshalJSON(doc); err != nil {
				slog.Warn("skipping non-JSON CRD document", "error", err)
				continue
			}

			gvk := obj.GroupVersionKind()
			name := obj.GetName()
			if name == "" {
				continue
			}

			gvr := schema.GroupVersionResource{
				Group:    gvk.Group,
				Version:  gvk.Version,
				Resource: strings.ToLower(gvk.Kind) + "s",
			}

			// CRDs are cluster-scoped — use the apiextensions resource path.
			if gvk.Kind == "CustomResourceDefinition" {
				gvr = schema.GroupVersionResource{
					Group:    "apiextensions.k8s.io",
					Version:  gvk.Version,
					Resource: "customresourcedefinitions",
				}
			}

			data, err := obj.MarshalJSON()
			if err != nil {
				return fmt.Errorf("ApplyCRDs: marshal %s: %w", name, err)
			}

			_, err = dynClient.Resource(gvr).Patch(ctx, name,
				types.ApplyPatchType, data,
				metav1.PatchOptions{FieldManager: "mozza"})
			if err != nil {
				// If server-side apply fails (e.g. old cluster), fall back to create-or-update.
				if errors.IsNotFound(err) || errors.IsMethodNotSupported(err) {
					_, err = dynClient.Resource(gvr).Create(ctx, obj, metav1.CreateOptions{})
					if errors.IsAlreadyExists(err) {
						err = nil // CRD already exists, that's fine
					}
				}
				if err != nil {
					return fmt.Errorf("ApplyCRDs: apply %s %q: %w", gvk.Kind, name, err)
				}
			}

			applied++
			slog.Info("CRD applied", "kind", gvk.Kind, "name", name)
		}

		if progress != nil {
			progress(fmt.Sprintf("Applied %d CRDs from %s", applied, url))
		}
	}

	return nil
}

// fetchYAML downloads a YAML file from a URL and splits it into individual
// JSON documents (one per YAML document separated by ---).
func fetchYAML(ctx context.Context, url string) ([][]byte, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, crdFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Split multi-document YAML and convert each to JSON.
	var docs [][]byte
	reader := yaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(body)))
	for {
		doc, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		doc = bytes.TrimSpace(doc)
		if len(doc) == 0 {
			continue
		}

		jsonDoc, err := yaml.ToJSON(doc)
		if err != nil {
			return nil, fmt.Errorf("YAML to JSON: %w", err)
		}

		if len(jsonDoc) > 2 { // skip empty JSON objects "{}"
			docs = append(docs, jsonDoc)
		}
	}

	return docs, nil
}
