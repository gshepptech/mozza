// Package k8s implements a Kubernetes manifest compiler that transforms an
// AppPlan into typed Kubernetes objects or YAML deployment artifacts.
package k8s

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gshepptech/mozza/internal/compile"
	"github.com/gshepptech/mozza/internal/plan"
)

// Compiler generates Kubernetes manifests from an AppPlan.
// It implements the compile.Compiler interface.
type Compiler struct{}

// New creates a new Kubernetes manifest compiler.
func New() *Compiler {
	return &Compiler{}
}

// Name returns the human-readable name of this compiler target.
func (c *Compiler) Name() string {
	return "kubernetes"
}

// CompileObjects generates typed Kubernetes objects for every slice in the plan.
// Objects are ordered globally by kind for correct dependency resolution:
// ServiceAccounts → Roles → RoleBindings → ConfigMaps → PVCs →
// Workloads → Services → NetworkPolicies → HPAs → PDBs → Ingresses.
// The workload kind is dispatched based on SliceKind.
func (c *Compiler) CompileObjects(p *plan.AppPlan) ([]runtime.Object, error) {
	if p == nil {
		return nil, fmt.Errorf("CompileObjects: plan must not be nil")
	}

	// Default namespace to app name (matches deployer behavior).
	namespace := p.Namespace
	if namespace == "" {
		namespace = p.Name
	}

	var (
		serviceAccounts []runtime.Object
		roles           []runtime.Object
		roleBindings    []runtime.Object
		configMaps      []runtime.Object
		pvcs            []runtime.Object
		workloads       []runtime.Object
		services        []runtime.Object
		netPolicies     []runtime.Object
		hpas            []runtime.Object
		pdbs            []runtime.Object
		ingresses       []runtime.Object
	)

	for _, s := range p.Slices {
		// ServiceAccount.
		if sa := BuildServiceAccount(p.Name, s, namespace); sa != nil {
			serviceAccounts = append(serviceAccounts, sa)
		}

		// RBAC: Role + RoleBinding.
		if role := BuildRole(p.Name, s, namespace); role != nil {
			roles = append(roles, role)
		}
		if rb := BuildRoleBinding(p.Name, s, namespace); rb != nil {
			roleBindings = append(roleBindings, rb)
		}

		// RBAC: ClusterRole + ClusterRoleBinding.
		if cr := BuildClusterRole(p.Name, s); cr != nil {
			roles = append(roles, cr)
		}
		if crb := BuildClusterRoleBinding(p.Name, s, namespace); crb != nil {
			roleBindings = append(roleBindings, crb)
		}

		// ConfigMaps from Mounts.
		for _, cm := range BuildConfigMaps(p.Name, s, namespace) {
			configMaps = append(configMaps, cm)
		}

		// PVC for slices with database/cache storage (not StatefulSet — those use volumeClaimTemplates).
		if !isStatefulKind(s.Kind) {
			if pvc := BuildPVC(s, namespace, p.Name); pvc != nil {
				pvcs = append(pvcs, pvc)
			}
		}

		// Dispatch workload by slice kind.
		workloads = append(workloads, buildWorkload(s, namespace, p.Name))

		// Service generation: StatefulSet gets headless, others get ClusterIP.
		if hasPort(s) {
			if isStatefulKind(s.Kind) {
				services = append(services, BuildHeadlessService(s, namespace, p.Name))
			} else if !isJobKind(s.Kind) {
				services = append(services, BuildService(s, namespace, p.Name))
			}
		}

		// NetworkPolicy.
		if np := BuildNetworkPolicy(p.Name, s, namespace); np != nil {
			netPolicies = append(netPolicies, np)
		}

		// HPA.
		if hpa := BuildHPA(p.Name, s, namespace); hpa != nil {
			hpas = append(hpas, hpa)
		}

		// PDB.
		if pdb := BuildPDB(p.Name, s, namespace); pdb != nil {
			pdbs = append(pdbs, pdb)
		}

		// Ingress for public slices (not jobs/crons/daemons).
		if (s.Public || s.Domain != "") && hasPort(s) && !isJobKind(s.Kind) && s.Kind != plan.SliceKindDaemon {
			ingresses = append(ingresses, BuildIngress(s, namespace, p.Name))
		}
	}

	var objects []runtime.Object
	objects = append(objects, serviceAccounts...)
	objects = append(objects, roles...)
	objects = append(objects, roleBindings...)
	objects = append(objects, configMaps...)
	objects = append(objects, pvcs...)
	objects = append(objects, workloads...)
	objects = append(objects, services...)
	objects = append(objects, netPolicies...)
	objects = append(objects, hpas...)
	objects = append(objects, pdbs...)
	objects = append(objects, ingresses...)

	return objects, nil
}

// Compile generates Kubernetes YAML manifests for every slice in the plan.
// This satisfies the compile.Compiler interface for backward compatibility.
func (c *Compiler) Compile(_ context.Context, p *plan.AppPlan) (*compile.Result, error) {
	objects, err := c.CompileObjects(p)
	if err != nil {
		return nil, fmt.Errorf("Compile: %w", err)
	}

	var files []compile.OutputFile
	for _, obj := range objects {
		data, err := marshalObject(obj)
		if err != nil {
			return nil, fmt.Errorf("Compile: %w", err)
		}
		gvk := obj.GetObjectKind().GroupVersionKind()
		meta, ok := obj.(interface{ GetName() string })
		if !ok {
			continue
		}
		files = append(files, compile.OutputFile{
			Path:    filePath(meta.GetName(), strings.ToLower(gvk.Kind)),
			Content: data,
		})
	}

	summary := fmt.Sprintf("generated %d Kubernetes manifests for %q", len(files), p.Name)

	return &compile.Result{
		Files:   files,
		Summary: summary,
	}, nil
}

// buildWorkload dispatches to the appropriate workload builder based on SliceKind.
func buildWorkload(s plan.Slice, namespace string, appName string) runtime.Object {
	switch s.Kind {
	case plan.SliceKindDatabase, plan.SliceKindStateful:
		return BuildStatefulSet(s, namespace, appName)
	case plan.SliceKindTask:
		return BuildJob(s, namespace, appName)
	case plan.SliceKindScheduled:
		return BuildCronJob(s, namespace, appName)
	case plan.SliceKindDaemon:
		return BuildDaemonSet(s, namespace, appName)
	case plan.SliceKindWeb, plan.SliceKindWorker, plan.SliceKindAPI, plan.SliceKindCache, plan.SliceKindGateway:
		return BuildDeployment(s, namespace, appName)
	}
	// Unknown kind — fall back to Deployment.
	return BuildDeployment(s, namespace, appName)
}

// isStatefulKind returns true for kinds that produce StatefulSets.
func isStatefulKind(k plan.SliceKind) bool {
	return k == plan.SliceKindDatabase || k == plan.SliceKindStateful
}

// isJobKind returns true for kinds that produce Jobs or CronJobs.
func isJobKind(k plan.SliceKind) bool {
	return k == plan.SliceKindTask || k == plan.SliceKindScheduled
}

// hasPort returns true if the slice exposes at least one port.
func hasPort(s plan.Slice) bool {
	return s.Port > 0 || len(s.Ports) > 0
}

// filePath returns the conventional output path for a Kubernetes manifest.
func filePath(name, kind string) string {
	return fmt.Sprintf("k8s/%s-%s.yaml", name, kind)
}
