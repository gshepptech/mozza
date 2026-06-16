package k8s

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gshepptech/mozza/internal/plan"
)

// BuildRole generates a namespace-scoped Role for the slice's non-cluster-wide permissions.
// Returns nil if no namespace-scoped permissions exist.
func BuildRole(appName string, s plan.Slice, namespace string) *rbacv1.Role {
	var rules []rbacv1.PolicyRule
	for _, p := range s.Permissions {
		if p.ClusterWide {
			continue
		}
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{""},
			Verbs:     p.Verbs,
			Resources: p.Resources,
		})
	}
	if len(rules) == 0 {
		return nil
	}

	return &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: namespace,
			Labels:    labels(appName, s.Name),
		},
		Rules: rules,
	}
}

// BuildClusterRole generates a cluster-wide ClusterRole for the slice's cluster-wide permissions.
// Returns nil if no cluster-wide permissions exist.
func BuildClusterRole(appName string, s plan.Slice) *rbacv1.ClusterRole {
	var rules []rbacv1.PolicyRule
	for _, p := range s.Permissions {
		if !p.ClusterWide {
			continue
		}
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{""},
			Verbs:     p.Verbs,
			Resources: p.Resources,
		})
	}
	if len(rules) == 0 {
		return nil
	}

	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   s.Name,
			Labels: labels(appName, s.Name),
		},
		Rules: rules,
	}
}

// BuildRoleBinding generates a RoleBinding linking the slice's ServiceAccount to the Role.
// Returns nil if no namespace-scoped Role exists.
func BuildRoleBinding(appName string, s plan.Slice, namespace string) *rbacv1.RoleBinding {
	if BuildRole(appName, s, namespace) == nil {
		return nil
	}

	saName := s.ServiceAccount
	if saName == "" {
		saName = s.Name
	}

	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: namespace,
			Labels:    labels(appName, s.Name),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     s.Name,
		},
	}
}

// BuildClusterRoleBinding generates a ClusterRoleBinding linking the slice's
// ServiceAccount to the ClusterRole. Returns nil if no ClusterRole exists.
func BuildClusterRoleBinding(appName string, s plan.Slice, namespace string) *rbacv1.ClusterRoleBinding {
	if BuildClusterRole(appName, s) == nil {
		return nil
	}

	saName := s.ServiceAccount
	if saName == "" {
		saName = s.Name
	}

	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   s.Name,
			Labels: labels(appName, s.Name),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     s.Name,
		},
	}
}
