package rules

import (
	"fmt"
	"strings"

	"github.com/gshepptech/mozza/internal/doctor"
	"github.com/gshepptech/mozza/internal/plan"
)

// PublicDatabaseWarning checks if any database or cache slice is publicly exposed.
type PublicDatabaseWarning struct{}

// Name returns the rule identifier.
func (PublicDatabaseWarning) Name() string { return "public-database" }

// Evaluate warns if a database or cache slice has Public=true.
func (PublicDatabaseWarning) Evaluate(p *plan.AppPlan, _ *doctor.Signal) []doctor.Finding {
	var findings []doctor.Finding

	for _, s := range p.Slices {
		if !s.Public {
			continue
		}
		if s.Kind == plan.SliceKindDatabase || s.Kind == plan.SliceKindCache {
			findings = append(findings, doctor.Finding{
				Rule:        "public-database",
				Severity:    doctor.SeverityWarning,
				Message:     fmt.Sprintf("slice %q (%s) is publicly exposed — databases and caches should not be internet-facing", s.Name, s.Kind),
				Explanation: "Exposing a database or cache to the public internet is a serious security risk. Attackers can attempt brute-force logins, exploit known vulnerabilities, or exfiltrate data.",
				Fix:         fmt.Sprintf("remove 'open to the public' from the %q section", s.Name),
				RecipeLine:  fmt.Sprintf("  # remove this line from %q:\n  # open to the public", s.Name),
			})
		}
	}

	if len(findings) == 0 {
		return []doctor.Finding{{
			Rule:     "public-database",
			Severity: doctor.SeverityOK,
			Message:  "no databases or caches are publicly exposed",
		}}
	}

	return findings
}

// NoHealthCheck checks if web or API slices have health checks configured.
type NoHealthCheck struct{}

// Name returns the rule identifier.
func (NoHealthCheck) Name() string { return "no-health-check" }

// Evaluate warns if a web, API, or gateway slice has no probes and no health path.
func (NoHealthCheck) Evaluate(p *plan.AppPlan, _ *doctor.Signal) []doctor.Finding {
	var findings []doctor.Finding

	for _, s := range p.Slices {
		if s.Kind != plan.SliceKindWeb && s.Kind != plan.SliceKindAPI && s.Kind != plan.SliceKindGateway {
			continue
		}
		if s.HealthPath != "" || len(s.Probes) > 0 {
			continue
		}
		findings = append(findings, doctor.Finding{
			Rule:        "no-health-check",
			Severity:    doctor.SeverityWarning,
			Message:     fmt.Sprintf("slice %q (%s) has no health check configured", s.Name, s.Kind),
			Explanation: "Without a health check, Kubernetes cannot detect when your app is broken and will keep sending traffic to unhealthy pods. This means users see errors instead of being routed to healthy instances.",
			Fix:         fmt.Sprintf("add 'health check /healthz' to the %q section", s.Name),
			RecipeLine:  fmt.Sprintf("  health check /healthz"),
			Fixable:     true,
		})
	}

	if len(findings) == 0 {
		return []doctor.Finding{{
			Rule:     "no-health-check",
			Severity: doctor.SeverityOK,
			Message:  "all web and API slices have health checks",
		}}
	}

	return findings
}

// SingleReplicaProduction checks for single-replica web/API slices in production namespaces.
type SingleReplicaProduction struct{}

// Name returns the rule identifier.
func (SingleReplicaProduction) Name() string { return "single-replica-production" }

// Evaluate warns if a web or API slice in a production namespace has one or fewer replicas.
func (SingleReplicaProduction) Evaluate(p *plan.AppPlan, _ *doctor.Signal) []doctor.Finding {
	if !strings.Contains(strings.ToLower(p.Namespace), "prod") {
		return nil
	}

	var findings []doctor.Finding

	for _, s := range p.Slices {
		if s.Kind != plan.SliceKindWeb && s.Kind != plan.SliceKindAPI && s.Kind != plan.SliceKindGateway {
			continue
		}
		if s.Replicas > 1 {
			continue
		}
		findings = append(findings, doctor.Finding{
			Rule:        "single-replica-production",
			Severity:    doctor.SeverityWarning,
			Message:     fmt.Sprintf("slice %q has %d replica(s) in production — consider running at least 2 for high availability", s.Name, s.Replicas),
			Explanation: "Running a single replica in production means any pod restart (deploy, crash, node drain) causes downtime. With 2+ replicas, Kubernetes can route traffic to healthy pods during updates.",
			Fix:         fmt.Sprintf("add 'run 2 copies' (or more) to the %q section", s.Name),
			RecipeLine:  fmt.Sprintf("  run 2 copies"),
		})
	}

	if len(findings) == 0 {
		return []doctor.Finding{{
			Rule:     "single-replica-production",
			Severity: doctor.SeverityOK,
			Message:  "all web and API slices have multiple replicas in production",
		}}
	}

	return findings
}

// NoResourceLimits checks if slices have CPU and memory limits configured.
type NoResourceLimits struct{}

// Name returns the rule identifier.
func (NoResourceLimits) Name() string { return "no-resource-limits" }

// Evaluate warns if a slice has no resource limits set.
func (NoResourceLimits) Evaluate(p *plan.AppPlan, _ *doctor.Signal) []doctor.Finding {
	var findings []doctor.Finding

	for _, s := range p.Slices {
		// Skip databases and caches — they have their own resource management.
		if s.Kind == plan.SliceKindDatabase || s.Kind == plan.SliceKindCache {
			continue
		}
		if s.Resources != nil && (s.Resources.CPULimit != "" || s.Resources.MemoryLimit != "") {
			continue
		}
		findings = append(findings, doctor.Finding{
			Rule:        "no-resource-limits",
			Severity:    doctor.SeverityWarning,
			Message:     fmt.Sprintf("slice %q has no CPU or memory limits — this may cause resource contention", s.Name),
			Explanation: "Without resource limits, a single misbehaving container can consume all the CPU and memory on a node, starving other workloads and potentially crashing the entire node.",
			Fix:         fmt.Sprintf("add 'limit cpu 500m memory 256Mi' to the %q section", s.Name),
			RecipeLine:  fmt.Sprintf("  limit cpu 500m memory 256Mi"),
			Fixable:     true,
		})
	}

	if len(findings) == 0 {
		return []doctor.Finding{{
			Rule:     "no-resource-limits",
			Severity: doctor.SeverityOK,
			Message:  "all slices have resource limits configured",
		}}
	}

	return findings
}

// NoAutoScaleWithHighReplicas checks for slices with high replica counts but no autoscaling.
type NoAutoScaleWithHighReplicas struct{}

// Name returns the rule identifier.
func (NoAutoScaleWithHighReplicas) Name() string { return "no-autoscale-high-replicas" }

// Evaluate suggests HPA when a slice has 5 or more replicas but no autoscaling configured.
func (NoAutoScaleWithHighReplicas) Evaluate(p *plan.AppPlan, _ *doctor.Signal) []doctor.Finding {
	var findings []doctor.Finding

	for _, s := range p.Slices {
		if s.Replicas < 5 {
			continue
		}
		if s.AutoScale != nil {
			continue
		}
		findings = append(findings, doctor.Finding{
			Rule:        "no-autoscale-high-replicas",
			Severity:    doctor.SeverityInfo,
			Message:     fmt.Sprintf("slice %q runs %d replicas without autoscaling — consider adding HPA to scale dynamically", s.Name, s.Replicas),
			Explanation: "Running many fixed replicas wastes resources during low traffic and may not be enough during peak traffic. Autoscaling adjusts the number of replicas automatically based on actual load.",
			Fix:         fmt.Sprintf("add 'scale between %d and %d copies based on cpu 75%%' to the %q section", s.Replicas, s.Replicas*2, s.Name),
			RecipeLine:  fmt.Sprintf("  scale between %d and %d copies based on cpu 75%%", s.Replicas, s.Replicas*2),
		})
	}

	if len(findings) == 0 {
		return []doctor.Finding{{
			Rule:     "no-autoscale-high-replicas",
			Severity: doctor.SeverityOK,
			Message:  "no high-replica slices without autoscaling",
		}}
	}

	return findings
}

// RunAsRoot checks if slices are running as root (no security spec or user 0).
type RunAsRoot struct{}

// Name returns the rule identifier.
func (RunAsRoot) Name() string { return "run-as-root" }

// Evaluate warns if a slice has no security context or runs as user 0.
func (RunAsRoot) Evaluate(p *plan.AppPlan, _ *doctor.Signal) []doctor.Finding {
	var findings []doctor.Finding

	for _, s := range p.Slices {
		// Skip databases and caches — they manage their own users.
		if s.Kind == plan.SliceKindDatabase || s.Kind == plan.SliceKindCache {
			continue
		}
		if s.Security != nil && s.Security.RunAsUser != 0 {
			continue
		}
		findings = append(findings, doctor.Finding{
			Rule:        "run-as-root",
			Severity:    doctor.SeverityWarning,
			Message:     fmt.Sprintf("slice %q may run as root — this is a security risk", s.Name),
			Explanation: "Running as root inside a container means if an attacker exploits your app, they have full control over the container and may be able to escape to the host. Running as a non-root user limits the blast radius.",
			Fix:         fmt.Sprintf("add 'run as user 1000' to the %q section", s.Name),
			RecipeLine:  fmt.Sprintf("  run as user 1000"),
		})
	}

	if len(findings) == 0 {
		return []doctor.Finding{{
			Rule:     "run-as-root",
			Severity: doctor.SeverityOK,
			Message:  "no slices running as root",
		}}
	}

	return findings
}

// NoGracefulShutdown checks if multi-replica web/API slices have graceful shutdown configured.
type NoGracefulShutdown struct{}

// Name returns the rule identifier.
func (NoGracefulShutdown) Name() string { return "no-graceful-shutdown" }

// Evaluate warns if a web or API slice with multiple replicas has no graceful shutdown period.
func (NoGracefulShutdown) Evaluate(p *plan.AppPlan, _ *doctor.Signal) []doctor.Finding {
	var findings []doctor.Finding

	for _, s := range p.Slices {
		if s.Kind != plan.SliceKindWeb && s.Kind != plan.SliceKindAPI && s.Kind != plan.SliceKindGateway {
			continue
		}
		if s.Replicas <= 1 {
			continue
		}
		if s.GracefulShutdown > 0 {
			continue
		}
		findings = append(findings, doctor.Finding{
			Rule:        "no-graceful-shutdown",
			Severity:    doctor.SeverityWarning,
			Message:     fmt.Sprintf("slice %q has %d replicas but no graceful shutdown — in-flight requests may be dropped during deploys", s.Name, s.Replicas),
			Explanation: "Without a graceful shutdown period, Kubernetes kills your pods immediately during deployments. Any requests being processed at that moment get dropped, causing errors for your users.",
			Fix:         fmt.Sprintf("add 'graceful shutdown 30s' to the %q section", s.Name),
			RecipeLine:  fmt.Sprintf("  graceful shutdown 30s"),
		})
	}

	if len(findings) == 0 {
		return []doctor.Finding{{
			Rule:     "no-graceful-shutdown",
			Severity: doctor.SeverityOK,
			Message:  "all multi-replica web and API slices have graceful shutdown configured",
		}}
	}

	return findings
}
