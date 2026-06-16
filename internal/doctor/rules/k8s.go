package rules

import (
	"fmt"

	"github.com/gshepptech/mozza/internal/doctor"
	"github.com/gshepptech/mozza/internal/plan"
)

// K8sRBACRule checks Kubernetes RBAC permissions required for deployment.
type K8sRBACRule struct{}

// Name returns the rule identifier.
func (K8sRBACRule) Name() string { return "k8s-rbac" }

// Evaluate checks K8s permissions from the collected signals.
func (K8sRBACRule) Evaluate(_ *plan.AppPlan, sig *doctor.Signal) []doctor.Finding {
	if !sig.K8sReachable {
		if sig.K8sError != nil {
			return []doctor.Finding{{
				Rule:        "k8s-rbac",
				Severity:    doctor.SeverityWarning,
				Message:     "Kubernetes cluster is not reachable",
				Explanation: "Mozza cannot connect to a Kubernetes cluster. If you intend to deploy to Kubernetes, check that your kubeconfig is set up correctly and the cluster is running.",
				Fix:         "Check your kubeconfig or KUBECONFIG environment variable",
			}}
		}
		return nil
	}

	var findings []doctor.Finding

	requiredPerms := []struct {
		key     string
		label   string
		fix     string
		warning bool // true = warning instead of error (optional capability)
	}{
		{"deployments/create", "Can create Deployments", "Grant create permission for Deployments", false},
		{"services/create", "Can create Services", "Grant create permission for Services", false},
		{"ingresses/create", "Can create Ingresses", "Grant create permission for Ingresses", false},
		{"persistentvolumeclaims/create", "Can create PersistentVolumeClaims", "Grant create permission for PVCs", false},
		{"namespaces/create", "Can create Namespaces", "Namespace auto-creation will not work. Pre-create namespaces manually.", true},
		{"secrets/get", "Can read Secrets", "Grant get permission for Secrets (needed for pre-validation)", false},
	}

	for _, perm := range requiredPerms {
		allowed, checked := sig.K8sPermissions[perm.key]
		if !checked {
			continue
		}

		if allowed {
			findings = append(findings, doctor.Finding{
				Rule:     "k8s-rbac",
				Severity: doctor.SeverityOK,
				Message:  perm.label,
			})
		} else {
			severity := doctor.SeverityError
			explanation := fmt.Sprintf("Your Kubernetes user does not have permission to %s. Deployment will fail because Mozza needs this permission to create the required resources.", perm.label)
			if perm.warning {
				severity = doctor.SeverityWarning
				explanation = "Your Kubernetes user cannot create namespaces. Mozza can still deploy if the target namespace already exists, but it cannot auto-create new namespaces for you."
			}
			findings = append(findings, doctor.Finding{
				Rule:        "k8s-rbac",
				Severity:    severity,
				Message:     fmt.Sprintf("Cannot: %s", perm.label),
				Explanation: explanation,
				Fix:         perm.fix,
			})
		}
	}

	return findings
}
