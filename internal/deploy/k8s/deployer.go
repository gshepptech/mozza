package k8s

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/gshepptech/mozza/internal/deploy"
	k8scompiler "github.com/gshepptech/mozza/internal/k8s"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/recipe"
	"github.com/gshepptech/mozza/internal/store"
)

// fieldManager is the server-side apply field manager name.
const fieldManager = "mozza"

// defaultTimeout is the default readiness wait timeout.
const defaultTimeout = 5 * time.Minute

// pollInterval is how often we check deployment readiness.
const pollInterval = 2 * time.Second

// ConfirmFunc is a callback that asks the user for confirmation.
// Returns true if the user confirms. Used for namespace creation and cleanup prompts.
type ConfirmFunc func(prompt string) bool

// ProgressFunc is a callback for reporting deploy progress to the user.
type ProgressFunc func(phase string, current, total int, message string)

// Deployer implements deploy.Deployer for Kubernetes clusters.
type Deployer struct {
	store    *store.Store
	compiler *k8scompiler.Compiler
	confirm  ConfirmFunc
	progress ProgressFunc
}

// New creates a new Kubernetes deployer.
func New(s *store.Store) *Deployer {
	return &Deployer{
		store:    s,
		compiler: k8scompiler.New(),
	}
}

// WithConfirm sets a confirmation callback for interactive prompts.
func (d *Deployer) WithConfirm(fn ConfirmFunc) *Deployer {
	d.confirm = fn
	return d
}

// WithProgress sets a progress callback for deploy status updates.
func (d *Deployer) WithProgress(fn ProgressFunc) *Deployer {
	d.progress = fn
	return d
}

// reportProgress calls the progress callback if set.
func (d *Deployer) reportProgress(phase string, current, total int, message string) {
	if d.progress != nil {
		d.progress(phase, current, total, message)
	}
}

// askConfirm calls the confirm callback if set, defaulting to true (non-interactive).
func (d *Deployer) askConfirm(prompt string) bool {
	if d.confirm != nil {
		return d.confirm(prompt)
	}
	return true
}

// Deploy compiles the plan into K8s objects and applies them via server-side apply.
func (d *Deployer) Deploy(ctx context.Context, p *plan.AppPlan, opts deploy.DeployOptions) (*deploy.DeployResult, error) {
	start := time.Now()
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	triggeredBy := opts.TriggeredBy
	if triggeredBy == "" {
		triggeredBy = "cli"
	}

	// Compile plan to typed objects.
	objects, err := d.compiler.CompileObjects(p)
	if err != nil {
		return nil, fmt.Errorf("Deploy: %w", err)
	}

	namespace := p.Namespace
	if namespace == "" {
		namespace = p.Name
	}

	// Create deploy record.
	rec, err := d.store.CreateDeploy(p.Name, "kubernetes", opts.Context, namespace, opts.RecipeContent, triggeredBy)
	if err != nil {
		return nil, fmt.Errorf("Deploy: record: %w", err)
	}

	// Record images.
	for _, s := range p.Slices {
		if s.Image != "" {
			_ = d.store.RecordDeployImage(rec.ID, s.Name, s.Image)
		}
	}

	// Connect to cluster.
	clientset, k8sCtx, err := newClientset(opts.Context)
	if err != nil {
		d.completeDeploy(rec.ID, deploy.StatusFailed, err.Error(), time.Since(start))
		return nil, fmt.Errorf("Deploy: %w", err)
	}

	slog.Info("deploying to kubernetes", "app", p.Name, "namespace", namespace, "context", k8sCtx)

	d.reportProgress("connecting", 0, len(objects), "Connected to cluster")

	// Ensure namespace exists.
	created, err := EnsureNamespace(ctx, clientset, namespace)
	if err != nil {
		// Namespace doesn't exist and we can't create it — check if user wants to create.
		if !created {
			if !d.askConfirm(fmt.Sprintf("Namespace %q does not exist. Create it?", namespace)) {
				d.completeDeploy(rec.ID, deploy.StatusFailed, "namespace creation declined", time.Since(start))
				return nil, fmt.Errorf("Deploy: namespace %q does not exist", namespace)
			}
		}
		d.completeDeploy(rec.ID, deploy.StatusFailed, err.Error(), time.Since(start))
		return nil, fmt.Errorf("Deploy: namespace: %w", err)
	}
	if created {
		slog.Info("created namespace", "namespace", namespace)
	}

	// Apply CRDs before anything else (cluster-scoped, order-sensitive).
	if len(p.CRDs) > 0 {
		d.reportProgress("crds", 0, len(objects), "Applying CRDs...")
		if err := ApplyCRDs(ctx, p.CRDs, func(msg string) {
			d.reportProgress("crds", 0, len(objects), msg)
		}); err != nil {
			d.completeDeploy(rec.ID, deploy.StatusFailed, err.Error(), time.Since(start))
			return nil, fmt.Errorf("Deploy: %w", err)
		}
	}

	// Pre-validate secrets.
	d.reportProgress("validating", 0, len(objects), "Validating secrets...")
	if err := ValidateSecrets(ctx, clientset, namespace, p); err != nil {
		d.completeDeploy(rec.ID, deploy.StatusFailed, err.Error(), time.Since(start))
		return nil, err
	}

	// Detect and clean up removed slices from previous deploy.
	removed, _ := DetectRemovedSlices(d.store, p.Name, p.Slices)
	if len(removed) > 0 {
		for _, name := range removed {
			if d.askConfirm(fmt.Sprintf("Slice %q was removed from the recipe. Delete its K8s resources?", name)) {
				CleanupRemovedSlices(ctx, clientset, namespace, p.Name, []string{name})
			}
		}
	}

	d.reportProgress("applying", 0, len(objects), "Applying resources...")

	// Apply objects and track for rollback.
	var applied []deploy.DeployedResource
	for _, obj := range objects {
		resource, err := applyObject(ctx, clientset, namespace, obj)
		if err != nil {
			slog.Error("apply failed, rolling back", "error", err)
			rollbackResources(ctx, clientset, applied)
			d.completeDeploy(rec.ID, deploy.StatusRolledBack, err.Error(), time.Since(start))
			return &deploy.DeployResult{
				DeployID:  rec.ID,
				Resources: applied,
				Duration:  time.Since(start),
				Status:    deploy.StatusRolledBack,
			}, fmt.Errorf("Deploy: apply: %w", err)
		}
		applied = append(applied, *resource)
		_ = d.store.RecordDeployResource(rec.ID, resource.Kind, resource.Name, resource.Namespace, "applied")
		d.reportProgress("applying", len(applied), len(objects),
			fmt.Sprintf("%s/%s applied", resource.Kind, resource.Name))
	}

	d.reportProgress("waiting", len(applied), len(objects), "Waiting for readiness...")

	// Wait for readiness.
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := waitForReady(waitCtx, clientset, namespace, p); err != nil {
		slog.Error("readiness timeout, rolling back", "error", err)
		rollbackResources(ctx, clientset, applied)
		d.completeDeploy(rec.ID, deploy.StatusRolledBack, err.Error(), time.Since(start))
		return &deploy.DeployResult{
			DeployID:  rec.ID,
			Resources: applied,
			Duration:  time.Since(start),
			Status:    deploy.StatusRolledBack,
		}, fmt.Errorf("Deploy: readiness: %w", err)
	}

	d.completeDeploy(rec.ID, deploy.StatusSuccess, "", time.Since(start))

	return &deploy.DeployResult{
		DeployID:  rec.ID,
		Resources: applied,
		Duration:  time.Since(start),
		Status:    deploy.StatusSuccess,
	}, nil
}

// Rollback reverts to the previous successful deploy by re-applying its recipe.
// It retrieves the stored recipe content, parses it through the full pipeline,
// and deploys it as a new deploy with rollback_of reference.
func (d *Deployer) Rollback(ctx context.Context, appName string) error {
	prev, err := d.store.PreviousSuccessfulDeploy(appName)
	if err != nil {
		return fmt.Errorf("Rollback: no previous deploy to roll back to: %w", err)
	}

	if prev.RecipeContent == "" {
		return fmt.Errorf("Rollback: previous deploy %s has no stored recipe content", prev.ID)
	}

	// Parse the stored recipe through the full pipeline.
	r, err := recipe.NewParser(prev.RecipeContent).Parse()
	if err != nil {
		return fmt.Errorf("Rollback: parse stored recipe: %w", err)
	}

	p, err := plan.Build(r)
	if err != nil {
		return fmt.Errorf("Rollback: build plan: %w", err)
	}

	if err := plan.Validate(p); err != nil {
		return fmt.Errorf("Rollback: validate plan: %w", err)
	}

	// Find the current deploy we're rolling back from.
	current, _ := d.store.LatestSuccessfulDeploy(appName)
	var rollbackOfID string
	if current != nil {
		rollbackOfID = current.ID
	}

	// Deploy with the previous recipe.
	result, err := d.Deploy(ctx, p, deploy.DeployOptions{
		Context:       prev.K8sContext,
		RecipeContent: prev.RecipeContent,
		TriggeredBy:   "rollback",
		RollbackOf:    rollbackOfID,
	})
	if err != nil {
		return fmt.Errorf("Rollback: %w", err)
	}

	// Record the rollback reference.
	if rollbackOfID != "" {
		_ = d.store.SetRollbackOf(result.DeployID, rollbackOfID)
	}

	slog.Info("rollback complete", "app", appName, "deploy_id", result.DeployID, "rolled_back_from", rollbackOfID)
	return nil
}

// Status returns the current health of a deployed application by querying
// K8s Deployments with the managed-by label.
func (d *Deployer) Status(ctx context.Context, appName string) (*deploy.AppStatus, error) {
	// Look up the latest successful deploy to find context and namespace.
	rec, err := d.store.LatestSuccessfulDeploy(appName)
	if err != nil {
		return nil, fmt.Errorf("Status: no deploy found for %q: %w", appName, err)
	}

	clientset, k8sCtx, err := newClientset(rec.K8sContext)
	if err != nil {
		return nil, fmt.Errorf("Status: %w", err)
	}

	namespace := rec.Namespace

	// List all Deployments managed by mozza for this app.
	labelSelector := fmt.Sprintf("app.kubernetes.io/managed-by=mozza,app.kubernetes.io/name=%s", appName)
	deps, err := clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("Status: list deployments: %w", err)
	}

	var slices []deploy.SliceStatus
	for _, dep := range deps.Items {
		ss := deploy.SliceStatus{
			Name:    dep.Name,
			Ready:   int(dep.Status.ReadyReplicas),
			Desired: int(dep.Status.Replicas),
			Age:     time.Since(dep.CreationTimestamp.Time),
		}

		// Determine status.
		switch {
		case dep.Status.ReadyReplicas == dep.Status.Replicas && dep.Status.Replicas > 0:
			ss.Status = "running"
		case dep.Status.ReadyReplicas > 0:
			ss.Status = "degraded"
		case dep.Status.Replicas == 0:
			ss.Status = "down"
		default:
			ss.Status = "pending"
		}

		// Extract image from the first container.
		if len(dep.Spec.Template.Spec.Containers) > 0 {
			ss.Image = dep.Spec.Template.Spec.Containers[0].Image
		}

		// Sum restarts from pod status.
		pods, podErr := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s,app.kubernetes.io/component=%s", appName, dep.Name),
		})
		if podErr == nil {
			for _, pod := range pods.Items {
				for _, cs := range pod.Status.ContainerStatuses {
					ss.Restarts += int(cs.RestartCount)
				}
			}
		}

		slices = append(slices, ss)
	}

	return &deploy.AppStatus{
		AppName:   appName,
		Namespace: namespace,
		Context:   k8sCtx,
		Slices:    slices,
	}, nil
}

// Logs streams pod logs for an application by multiplexing logs from all matching
// pods. Each log line is prefixed with the pod name for identification.
func (d *Deployer) Logs(ctx context.Context, appName string, opts deploy.LogOptions) (io.ReadCloser, error) {
	rec, err := d.store.LatestSuccessfulDeploy(appName)
	if err != nil {
		return nil, fmt.Errorf("Logs: no deploy found for %q: %w", appName, err)
	}

	clientset, _, err := newClientset(rec.K8sContext)
	if err != nil {
		return nil, fmt.Errorf("Logs: %w", err)
	}

	namespace := rec.Namespace

	// Build label selector.
	labelSelector := fmt.Sprintf("app.kubernetes.io/managed-by=mozza,app.kubernetes.io/name=%s", appName)
	if opts.SliceName != "" {
		labelSelector += fmt.Sprintf(",app.kubernetes.io/component=%s", opts.SliceName)
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("Logs: list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("Logs: no pods found for %q", appName)
	}

	// Create a pipe for multiplexed output.
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()

		var wg sync.WaitGroup
		for _, pod := range pods.Items {
			podName := pod.Name

			logOpts := &corev1.PodLogOptions{
				Follow: opts.Follow,
			}
			if opts.Since > 0 {
				sinceSeconds := int64(opts.Since.Seconds())
				logOpts.SinceSeconds = &sinceSeconds
			}

			stream, err := clientset.CoreV1().Pods(namespace).GetLogs(podName, logOpts).Stream(ctx)
			if err != nil {
				slog.Error("failed to stream pod logs", "pod", podName, "error", err)
				continue
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				defer stream.Close()

				scanner := bufio.NewScanner(stream)
				for scanner.Scan() {
					line := fmt.Sprintf("[%s] %s\n", podName, scanner.Text())
					if _, err := pw.Write([]byte(line)); err != nil {
						return
					}
				}
			}()
		}

		wg.Wait()
	}()

	return pr, nil
}

// Down tears down all K8s resources for an application. It deletes Ingresses,
// Services, and Deployments. PVCs are only deleted if opts.DeletePVCs is true.
func (d *Deployer) Down(ctx context.Context, appName string, opts deploy.DownOptions) error {
	rec, err := d.store.LatestSuccessfulDeploy(appName)
	if err != nil {
		return fmt.Errorf("Down: no deploy found for %q: %w", appName, err)
	}

	clientset, _, err := newClientset(rec.K8sContext)
	if err != nil {
		return fmt.Errorf("Down: %w", err)
	}

	namespace := rec.Namespace
	labelSelector := fmt.Sprintf("app.kubernetes.io/managed-by=mozza,app.kubernetes.io/name=%s", appName)
	delOpts := metav1.DeleteOptions{}

	// Delete Ingresses.
	ingresses, err := clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err == nil {
		for _, ing := range ingresses.Items {
			if err := clientset.NetworkingV1().Ingresses(namespace).Delete(ctx, ing.Name, delOpts); err != nil && !apierrors.IsNotFound(err) {
				slog.Error("failed to delete ingress", "name", ing.Name, "error", err)
			} else {
				slog.Info("deleted ingress", "name", ing.Name)
			}
		}
	}

	// Delete Services.
	services, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err == nil {
		for _, svc := range services.Items {
			if err := clientset.CoreV1().Services(namespace).Delete(ctx, svc.Name, delOpts); err != nil && !apierrors.IsNotFound(err) {
				slog.Error("failed to delete service", "name", svc.Name, "error", err)
			} else {
				slog.Info("deleted service", "name", svc.Name)
			}
		}
	}

	// Delete Deployments.
	deps, err := clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err == nil {
		for _, dep := range deps.Items {
			if err := clientset.AppsV1().Deployments(namespace).Delete(ctx, dep.Name, delOpts); err != nil && !apierrors.IsNotFound(err) {
				slog.Error("failed to delete deployment", "name", dep.Name, "error", err)
			} else {
				slog.Info("deleted deployment", "name", dep.Name)
			}
		}
	}

	// Delete PVCs only if explicitly requested.
	if opts.DeletePVCs {
		pvcs, err := clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err == nil {
			for _, pvc := range pvcs.Items {
				if err := clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvc.Name, delOpts); err != nil && !apierrors.IsNotFound(err) {
					slog.Error("failed to delete pvc", "name", pvc.Name, "error", err)
				} else {
					slog.Info("deleted pvc", "name", pvc.Name)
				}
			}
		}
	}

	return nil
}

func (d *Deployer) completeDeploy(id string, status deploy.DeployStatus, errMsg string, duration time.Duration) {
	if err := d.store.CompleteDeploy(id, string(status), errMsg, duration.Milliseconds()); err != nil {
		slog.Error("failed to record deploy completion", "error", err)
	}
}

// applyObject applies a single K8s object using server-side apply.
func applyObject(ctx context.Context, cs kubernetes.Interface, namespace string, obj runtime.Object) (*deploy.DeployedResource, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("applyObject: marshal: %w", err)
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	meta, ok := obj.(interface{ GetName() string })
	if !ok {
		return nil, fmt.Errorf("applyObject: object has no name")
	}
	name := meta.GetName()

	resource := &deploy.DeployedResource{
		Kind:      gvk.Kind,
		Name:      name,
		Namespace: namespace,
		Status:    "applied",
	}

	switch gvk.Kind {
	case "Deployment":
		_, err = cs.AppsV1().Deployments(namespace).Patch(ctx, name,
			types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	case "Service":
		_, err = cs.CoreV1().Services(namespace).Patch(ctx, name,
			types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	case "Ingress":
		_, err = cs.NetworkingV1().Ingresses(namespace).Patch(ctx, name,
			types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	case "PersistentVolumeClaim":
		_, err = cs.CoreV1().PersistentVolumeClaims(namespace).Patch(ctx, name,
			types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	case "Namespace":
		_, err = cs.CoreV1().Namespaces().Patch(ctx, name,
			types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	case "StatefulSet":
		_, err = cs.AppsV1().StatefulSets(namespace).Patch(ctx, name,
			types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	case "Job":
		_, err = cs.BatchV1().Jobs(namespace).Patch(ctx, name,
			types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	case "CronJob":
		_, err = cs.BatchV1().CronJobs(namespace).Patch(ctx, name,
			types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	case "Secret":
		_, err = cs.CoreV1().Secrets(namespace).Patch(ctx, name,
			types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	case "ConfigMap":
		_, err = cs.CoreV1().ConfigMaps(namespace).Patch(ctx, name,
			types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	case "ServiceAccount":
		_, err = cs.CoreV1().ServiceAccounts(namespace).Patch(ctx, name,
			types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	case "Role":
		_, err = cs.RbacV1().Roles(namespace).Patch(ctx, name,
			types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	case "RoleBinding":
		_, err = cs.RbacV1().RoleBindings(namespace).Patch(ctx, name,
			types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	case "ClusterRole":
		_, err = cs.RbacV1().ClusterRoles().Patch(ctx, name,
			types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	case "ClusterRoleBinding":
		_, err = cs.RbacV1().ClusterRoleBindings().Patch(ctx, name,
			types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	case "DaemonSet":
		_, err = cs.AppsV1().DaemonSets(namespace).Patch(ctx, name,
			types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	case "NetworkPolicy":
		_, err = cs.NetworkingV1().NetworkPolicies(namespace).Patch(ctx, name,
			types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	case "HorizontalPodAutoscaler":
		_, err = cs.AutoscalingV2().HorizontalPodAutoscalers(namespace).Patch(ctx, name,
			types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	case "PodDisruptionBudget":
		_, err = cs.PolicyV1().PodDisruptionBudgets(namespace).Patch(ctx, name,
			types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	default:
		return nil, fmt.Errorf("applyObject: unsupported kind %q", gvk.Kind)
	}

	if err != nil {
		resource.Status = "failed"
		return nil, fmt.Errorf("applyObject: %s %q: %w", gvk.Kind, name, err)
	}

	slog.Info("resource applied", "kind", gvk.Kind, "name", name, "namespace", namespace)
	return resource, nil
}

// rollbackResources deletes previously applied resources in reverse order.
func rollbackResources(ctx context.Context, cs kubernetes.Interface, resources []deploy.DeployedResource) {
	for i := len(resources) - 1; i >= 0; i-- {
		r := resources[i]
		var err error

		switch r.Kind {
		case "Deployment":
			err = cs.AppsV1().Deployments(r.Namespace).Delete(ctx, r.Name, metav1.DeleteOptions{})
		case "Service":
			err = cs.CoreV1().Services(r.Namespace).Delete(ctx, r.Name, metav1.DeleteOptions{})
		case "Ingress":
			err = cs.NetworkingV1().Ingresses(r.Namespace).Delete(ctx, r.Name, metav1.DeleteOptions{})
		case "PersistentVolumeClaim":
			err = cs.CoreV1().PersistentVolumeClaims(r.Namespace).Delete(ctx, r.Name, metav1.DeleteOptions{})
		case "Namespace":
			// Don't delete namespaces during rollback — too destructive.
			continue
		}

		if err != nil && !apierrors.IsNotFound(err) {
			slog.Error("rollback: failed to delete resource", "kind", r.Kind, "name", r.Name, "error", err)
		} else {
			slog.Info("rolled back resource", "kind", r.Kind, "name", r.Name)
		}
	}
}

// waitForReady blocks until all Deployments in the plan are Available.
func waitForReady(ctx context.Context, cs kubernetes.Interface, namespace string, p *plan.AppPlan) error {
	// Collect deployment names.
	var deployNames []string
	for _, s := range p.Slices {
		deployNames = append(deployNames, s.Name)
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("waitForReady: %w", ctx.Err())
		case <-ticker.C:
			allReady := true
			for _, name := range deployNames {
				dep, err := cs.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						allReady = false
						continue
					}
					return fmt.Errorf("waitForReady: get %q: %w", name, err)
				}

				if !isDeploymentAvailable(dep) {
					allReady = false
				}
			}

			if allReady {
				return nil
			}
		}
	}
}

// isDeploymentAvailable checks if a Deployment has the Available condition True.
func isDeploymentAvailable(dep *appsv1.Deployment) bool {
	for _, c := range dep.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// WaitForHealthy polls K8s workloads in the given namespace until all are
// ready or the 5-minute timeout expires. logFn receives progress messages.
func (d *Deployer) WaitForHealthy(ctx context.Context, namespace string, logFn func(string)) error {
	const healthTimeout = 5 * time.Minute
	const healthPoll = 5 * time.Second

	waitCtx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()

	// Connect to cluster using default context.
	clientset, _, err := newClientset("")
	if err != nil {
		return fmt.Errorf("WaitForHealthy: %w", err)
	}

	labelSelector := "app.kubernetes.io/managed-by=mozza"

	ticker := time.NewTicker(healthPoll)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("WaitForHealthy: timed out after %s waiting for pods in %q", healthTimeout, namespace)
		case <-ticker.C:
			ready, total, details, err := d.checkWorkloadsReady(waitCtx, clientset, namespace, labelSelector)
			if err != nil {
				return fmt.Errorf("WaitForHealthy: %w", err)
			}
			if logFn != nil {
				logFn(fmt.Sprintf("Waiting for pods... (%d/%d services ready)", ready, total))
			}
			if total > 0 && ready == total {
				return nil
			}
			_ = details // used for error context below
		}
	}
}

// checkWorkloadsReady counts ready vs total workloads in a namespace.
func (d *Deployer) checkWorkloadsReady(ctx context.Context, cs kubernetes.Interface, namespace, labelSelector string) (ready, total int, details []string, err error) {
	// Check Deployments.
	deps, err := cs.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return 0, 0, nil, fmt.Errorf("list deployments: %w", err)
	}
	for _, dep := range deps.Items {
		total++
		if dep.Status.ReadyReplicas >= *dep.Spec.Replicas && *dep.Spec.Replicas > 0 {
			ready++
		} else {
			details = append(details, fmt.Sprintf("Deployment/%s: %d/%d ready", dep.Name, dep.Status.ReadyReplicas, *dep.Spec.Replicas))
		}
	}

	// Check StatefulSets.
	stss, err := cs.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return 0, 0, nil, fmt.Errorf("list statefulsets: %w", err)
	}
	for _, sts := range stss.Items {
		total++
		if sts.Status.ReadyReplicas >= *sts.Spec.Replicas && *sts.Spec.Replicas > 0 {
			ready++
		} else {
			details = append(details, fmt.Sprintf("StatefulSet/%s: %d/%d ready", sts.Name, sts.Status.ReadyReplicas, *sts.Spec.Replicas))
		}
	}

	// Check DaemonSets.
	dss, err := cs.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return 0, 0, nil, fmt.Errorf("list daemonsets: %w", err)
	}
	for _, ds := range dss.Items {
		total++
		if ds.Status.NumberReady >= ds.Status.DesiredNumberScheduled && ds.Status.DesiredNumberScheduled > 0 {
			ready++
		} else {
			details = append(details, fmt.Sprintf("DaemonSet/%s: %d/%d ready", ds.Name, ds.Status.NumberReady, ds.Status.DesiredNumberScheduled))
		}
	}

	return ready, total, details, nil
}

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

// DetectAccessURL inspects Ingresses and Services in the given namespace to
// find how the deployed application can be reached externally. It checks in
// order: Ingress host, LoadBalancer IP, NodePort. Returns "" if the app is
// internal-only.
func (d *Deployer) DetectAccessURL(ctx context.Context, namespace string) string {
	clientset, _, err := newClientset("")
	if err != nil {
		slog.Warn("DetectAccessURL: failed to connect", "error", err)
		return ""
	}

	labelSelector := "app.kubernetes.io/managed-by=mozza"

	// 1. Check Ingresses for a host rule.
	ingresses, err := clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err == nil && len(ingresses.Items) > 0 {
		for _, ing := range ingresses.Items {
			if len(ing.Spec.Rules) > 0 && ing.Spec.Rules[0].Host != "" {
				scheme := "https"
				if len(ing.Spec.TLS) == 0 {
					scheme = "http"
				}
				return fmt.Sprintf("%s://%s", scheme, ing.Spec.Rules[0].Host)
			}
		}
	}

	// 2. Check for LoadBalancer Services.
	services, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err == nil {
		for _, svc := range services.Items {
			if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
				if len(svc.Status.LoadBalancer.Ingress) > 0 {
					lbIngress := svc.Status.LoadBalancer.Ingress[0]
					host := lbIngress.IP
					if host == "" {
						host = lbIngress.Hostname
					}
					if host != "" {
						port := int32(80)
						if len(svc.Spec.Ports) > 0 {
							port = svc.Spec.Ports[0].Port
						}
						if port == 443 {
							return fmt.Sprintf("https://%s", host)
						}
						if port == 80 {
							return fmt.Sprintf("http://%s", host)
						}
						return fmt.Sprintf("http://%s:%d", host, port)
					}
				}
			}
		}

		// 3. Check for NodePort Services.
		for _, svc := range services.Items {
			if svc.Spec.Type == corev1.ServiceTypeNodePort {
				if len(svc.Spec.Ports) > 0 {
					nodePort := svc.Spec.Ports[0].NodePort
					nodes, nErr := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
					if nErr == nil && len(nodes.Items) > 0 {
						for _, addr := range nodes.Items[0].Status.Addresses {
							if addr.Type == corev1.NodeExternalIP || addr.Type == corev1.NodeInternalIP {
								return fmt.Sprintf("http://%s:%d", addr.Address, nodePort)
							}
						}
					}
				}
			}
		}
	}

	return ""
}

// Ensure Deployer implements deploy.Deployer at compile time.
var _ deploy.Deployer = (*Deployer)(nil)

// Suppress unused import warnings for types used in applyObject switch.
var (
	_ = (*appsv1.Deployment)(nil)
	_ = (*corev1.Service)(nil)
	_ = (*networkingv1.Ingress)(nil)
)
