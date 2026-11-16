package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// NamespaceSnapshot holds serialized K8s resources for rollback.
type NamespaceSnapshot struct {
	Deployments  []json.RawMessage `json:"deployments,omitempty"`
	StatefulSets []json.RawMessage `json:"statefulsets,omitempty"`
	Services     []json.RawMessage `json:"services,omitempty"`
	ConfigMaps   []json.RawMessage `json:"configmaps,omitempty"`
}

// SnapshotNamespace captures the current state of mozza-managed resources in a
// namespace, serialized as JSON. This snapshot can later be re-applied to
// restore the namespace to its previous state. If clientset is nil, a new
// connection is created using the default kubeconfig context.
func (d *Deployer) SnapshotNamespace(ctx context.Context, clientset kubernetes.Interface, namespace string) (string, error) {
	if clientset == nil {
		var err error
		clientset, _, err = newClientset("")
		if err != nil {
			return "", fmt.Errorf("SnapshotNamespace: connect: %w", err)
		}
	}

	labelSelector := "app.kubernetes.io/managed-by=mozza"
	var snap NamespaceSnapshot

	deps, err := clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return "", fmt.Errorf("SnapshotNamespace: list deployments: %w", err)
	}
	for _, dep := range deps.Items {
		data, marshalErr := json.Marshal(dep)
		if marshalErr != nil {
			slog.Error("snapshot: failed to marshal resource", "kind", "Deployment", "name", dep.Name, "error", marshalErr)
			continue
		}
		snap.Deployments = append(snap.Deployments, data)
	}

	stss, err := clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return "", fmt.Errorf("SnapshotNamespace: list statefulsets: %w", err)
	}
	for _, sts := range stss.Items {
		data, marshalErr := json.Marshal(sts)
		if marshalErr != nil {
			slog.Error("snapshot: failed to marshal resource", "kind", "StatefulSet", "name", sts.Name, "error", marshalErr)
			continue
		}
		snap.StatefulSets = append(snap.StatefulSets, data)
	}

	svcs, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return "", fmt.Errorf("SnapshotNamespace: list services: %w", err)
	}
	for _, svc := range svcs.Items {
		data, marshalErr := json.Marshal(svc)
		if marshalErr != nil {
			slog.Error("snapshot: failed to marshal resource", "kind", "Service", "name", svc.Name, "error", marshalErr)
			continue
		}
		snap.Services = append(snap.Services, data)
	}

	cms, err := clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return "", fmt.Errorf("SnapshotNamespace: list configmaps: %w", err)
	}
	for _, cm := range cms.Items {
		data, marshalErr := json.Marshal(cm)
		if marshalErr != nil {
			slog.Error("snapshot: failed to marshal resource", "kind", "ConfigMap", "name", cm.Name, "error", marshalErr)
			continue
		}
		snap.ConfigMaps = append(snap.ConfigMaps, data)
	}

	result, err := json.Marshal(snap)
	if err != nil {
		return "", fmt.Errorf("SnapshotNamespace: marshal: %w", err)
	}
	return string(result), nil
}

// RollbackSnapshot restores a namespace to a previously captured snapshot by
// re-applying each resource via server-side apply.
func (d *Deployer) RollbackSnapshot(ctx context.Context, previousState string, namespace string) error {
	var snap NamespaceSnapshot
	if err := json.Unmarshal([]byte(previousState), &snap); err != nil {
		return fmt.Errorf("RollbackSnapshot: unmarshal: %w", err)
	}

	clientset, _, err := newClientset("")
	if err != nil {
		return fmt.Errorf("RollbackSnapshot: %w", err)
	}

	// Re-apply Deployments.
	for _, data := range snap.Deployments {
		var dep appsv1.Deployment
		if err := json.Unmarshal(data, &dep); err != nil {
			return fmt.Errorf("RollbackSnapshot: unmarshal deployment: %w", err)
		}
		if _, err := applyObject(ctx, clientset, namespace, &dep); err != nil {
			return fmt.Errorf("RollbackSnapshot: apply deployment %q: %w", dep.Name, err)
		}
	}

	// Re-apply StatefulSets.
	for _, data := range snap.StatefulSets {
		var sts appsv1.StatefulSet
		if err := json.Unmarshal(data, &sts); err != nil {
			return fmt.Errorf("RollbackSnapshot: unmarshal statefulset: %w", err)
		}
		// StatefulSets use the same apply mechanism.
		stsData, marshalErr := json.Marshal(sts)
		if marshalErr != nil {
			return fmt.Errorf("RollbackSnapshot: marshal statefulset %q: %w", sts.Name, marshalErr)
		}
		_, err := clientset.AppsV1().StatefulSets(namespace).Patch(ctx, sts.Name,
			types.ApplyPatchType, stsData, metav1.PatchOptions{FieldManager: fieldManager})
		if err != nil {
			return fmt.Errorf("RollbackSnapshot: apply statefulset %q: %w", sts.Name, err)
		}
	}

	// Re-apply Services.
	for _, data := range snap.Services {
		var svc corev1.Service
		if err := json.Unmarshal(data, &svc); err != nil {
			return fmt.Errorf("RollbackSnapshot: unmarshal service: %w", err)
		}
		if _, err := applyObject(ctx, clientset, namespace, &svc); err != nil {
			return fmt.Errorf("RollbackSnapshot: apply service %q: %w", svc.Name, err)
		}
	}

	// Re-apply ConfigMaps.
	for _, data := range snap.ConfigMaps {
		var cm corev1.ConfigMap
		if err := json.Unmarshal(data, &cm); err != nil {
			return fmt.Errorf("RollbackSnapshot: unmarshal configmap: %w", err)
		}
		cmData, marshalErr := json.Marshal(cm)
		if marshalErr != nil {
			return fmt.Errorf("RollbackSnapshot: marshal configmap %q: %w", cm.Name, marshalErr)
		}
		_, err := clientset.CoreV1().ConfigMaps(namespace).Patch(ctx, cm.Name,
			types.ApplyPatchType, cmData, metav1.PatchOptions{FieldManager: fieldManager})
		if err != nil {
			return fmt.Errorf("RollbackSnapshot: apply configmap %q: %w", cm.Name, err)
		}
	}

	slog.Info("rollback snapshot applied", "namespace", namespace)
	return nil
}
