package cluster

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// defaultResyncPeriod is the interval at which the informer factory
// re-lists all resources from the API server.
const defaultResyncPeriod = 30 * time.Second

// InformerManager watches Kubernetes resources via SharedInformers and
// keeps the ClusterCache up to date.
type InformerManager struct {
	cache    *ClusterCache
	clientFn func() (kubernetes.Interface, error)
	factory  informers.SharedInformerFactory
	stopCh   chan struct{}
	running  bool
	mu       sync.Mutex
}

// NewInformerManager creates an InformerManager that uses clientFn to
// obtain a Kubernetes clientset and populates the given cache.
func NewInformerManager(clientFn func() (kubernetes.Interface, error), c *ClusterCache) *InformerManager {
	return &InformerManager{
		cache:    c,
		clientFn: clientFn,
		stopCh:   make(chan struct{}),
	}
}

// Start initialises the SharedInformerFactory, registers event handlers
// for all watched resource types, and starts the informers. It is safe
// to call from multiple goroutines; only the first call takes effect.
func (m *InformerManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	cs, err := m.clientFn()
	if err != nil {
		return fmt.Errorf("informer manager: %w", err)
	}

	m.factory = informers.NewSharedInformerFactory(cs, defaultResyncPeriod)
	m.registerHandlers(cs)
	m.factory.Start(m.stopCh)

	// Wait up to 5 seconds for initial sync — don't block forever.
	syncCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	syncDone := make(chan struct{})
	go func() {
		m.factory.WaitForCacheSync(m.stopCh)
		close(syncDone)
	}()
	select {
	case <-syncDone:
		slog.Info("informer manager started, cache synced")
	case <-syncCtx.Done():
		slog.Warn("informer manager started, cache sync timed out (will continue in background)")
	}

	m.running = true
	return nil
}

// Stop signals all informers to shut down.
func (m *InformerManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}
	close(m.stopCh)
	m.running = false
	slog.Info("informer manager stopped")
}

// registerHandlers wires up event handlers for each resource type.
func (m *InformerManager) registerHandlers(cs kubernetes.Interface) {
	// Pods
	podInformer := m.factory.Core().V1().Pods().Informer()
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ any) { m.syncPods() },
		UpdateFunc: func(_, _ any) { m.syncPods() },
		DeleteFunc: func(_ any) { m.syncPods() },
	}
	if _, err := podInformer.AddEventHandler(handler); err != nil {
		slog.Error("failed to add pod event handler", "error", err)
	}

	// Nodes
	nodeInformer := m.factory.Core().V1().Nodes().Informer()
	nodeHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ any) { m.syncNodes(); m.syncMetrics() },
		UpdateFunc: func(_, _ any) { m.syncNodes(); m.syncMetrics() },
		DeleteFunc: func(_ any) { m.syncNodes(); m.syncMetrics() },
	}
	if _, err := nodeInformer.AddEventHandler(nodeHandler); err != nil {
		slog.Error("failed to add node event handler", "error", err)
	}

	// Deployments
	depInformer := m.factory.Apps().V1().Deployments().Informer()
	depHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ any) { m.syncDeployments() },
		UpdateFunc: func(_, _ any) { m.syncDeployments() },
		DeleteFunc: func(_ any) { m.syncDeployments() },
	}
	if _, err := depInformer.AddEventHandler(depHandler); err != nil {
		slog.Error("failed to add deployment event handler", "error", err)
	}

	// Namespaces
	nsInformer := m.factory.Core().V1().Namespaces().Informer()
	nsHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ any) { m.syncNamespaces() },
		UpdateFunc: func(_, _ any) { m.syncNamespaces() },
		DeleteFunc: func(_ any) { m.syncNamespaces() },
	}
	if _, err := nsInformer.AddEventHandler(nsHandler); err != nil {
		slog.Error("failed to add namespace event handler", "error", err)
	}

	// Services
	svcInformer := m.factory.Core().V1().Services().Informer()
	svcHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ any) { m.syncServices() },
		UpdateFunc: func(_, _ any) { m.syncServices() },
		DeleteFunc: func(_ any) { m.syncServices() },
	}
	if _, err := svcInformer.AddEventHandler(svcHandler); err != nil {
		slog.Error("failed to add service event handler", "error", err)
	}

	// Events
	evtInformer := m.factory.Core().V1().Events().Informer()
	evtHandler := cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ any) { m.syncEvents() },
		UpdateFunc: func(_, _ any) { m.syncEvents() },
		DeleteFunc: func(_ any) { m.syncEvents() },
	}
	if _, err := evtInformer.AddEventHandler(evtHandler); err != nil {
		slog.Error("failed to add event event handler", "error", err)
	}
}

// --- sync helpers: read from informer store, transform, push to cache ---

func (m *InformerManager) syncPods() {
	items := m.factory.Core().V1().Pods().Informer().GetStore().List()
	pods := make([]PodInfo, 0, len(items))
	for _, obj := range items {
		p, ok := obj.(*corev1.Pod)
		if !ok {
			continue
		}

		var restarts int32
		readyCount := 0
		totalContainers := len(p.Spec.Containers)
		for _, cs := range p.Status.ContainerStatuses {
			restarts += cs.RestartCount
			if cs.Ready {
				readyCount++
			}
		}

		app := p.Labels["app"]
		if app == "" {
			app = p.Labels["app.kubernetes.io/name"]
		}

		pods = append(pods, PodInfo{
			Name:      p.Name,
			Namespace: p.Namespace,
			Status:    string(p.Status.Phase),
			Ready:     fmt.Sprintf("%d/%d", readyCount, totalContainers),
			Restarts:  restarts,
			Age:       formatAge(p.CreationTimestamp.Time),
			Node:      p.Spec.NodeName,
			IP:        p.Status.PodIP,
			App:       app,
		})
	}
	m.cache.SetPods(pods)

	// Recompute metrics whenever pods change (they affect pod counts
	// and CPU/memory requested).
	m.syncMetrics()
}

func (m *InformerManager) syncNodes() {
	items := m.factory.Core().V1().Nodes().Informer().GetStore().List()
	nodes := make([]NodeInfo, 0, len(items))
	for _, obj := range items {
		n, ok := obj.(*corev1.Node)
		if !ok {
			continue
		}

		status := "NotReady"
		for _, c := range n.Status.Conditions {
			if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
				status = "Ready"
				break
			}
		}

		roles := "worker"
		if _, ok := n.Labels["node-role.kubernetes.io/control-plane"]; ok {
			roles = "control-plane"
		} else if _, ok := n.Labels["node-role.kubernetes.io/master"]; ok {
			roles = "control-plane"
		}

		var internalIP string
		for _, addr := range n.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				internalIP = addr.Address
				break
			}
		}

		nodes = append(nodes, NodeInfo{
			Name:       n.Name,
			Status:     status,
			Roles:      roles,
			Age:        formatAge(n.CreationTimestamp.Time),
			Version:    n.Status.NodeInfo.KubeletVersion,
			CPU:        n.Status.Allocatable.Cpu().String(),
			Memory:     n.Status.Allocatable.Memory().String(),
			InternalIP: internalIP,
		})
	}
	m.cache.SetNodes(nodes)
}

func (m *InformerManager) syncDeployments() {
	items := m.factory.Apps().V1().Deployments().Informer().GetStore().List()
	deps := make([]DeploymentInfo, 0, len(items))
	for _, obj := range items {
		d, ok := obj.(*appsv1.Deployment)
		if !ok {
			continue
		}

		img := ""
		if len(d.Spec.Template.Spec.Containers) > 0 {
			img = d.Spec.Template.Spec.Containers[0].Image
		}
		replicas := int32(1)
		if d.Spec.Replicas != nil {
			replicas = *d.Spec.Replicas
		}

		deps = append(deps, DeploymentInfo{
			Name:      d.Name,
			Namespace: d.Namespace,
			Ready:     fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, replicas),
			UpToDate:  d.Status.UpdatedReplicas,
			Available: d.Status.AvailableReplicas,
			Age:       formatAge(d.CreationTimestamp.Time),
			Image:     img,
			Labels:    d.Labels,
		})
	}
	m.cache.SetDeployments(deps)
}

func (m *InformerManager) syncNamespaces() {
	items := m.factory.Core().V1().Namespaces().Informer().GetStore().List()
	nsList := make([]NamespaceInfo, 0, len(items))
	for _, obj := range items {
		ns, ok := obj.(*corev1.Namespace)
		if !ok {
			continue
		}
		nsList = append(nsList, NamespaceInfo{
			Name:   ns.Name,
			Status: string(ns.Status.Phase),
			Age:    formatAge(ns.CreationTimestamp.Time),
		})
	}
	m.cache.SetNamespaces(nsList)
}

func (m *InformerManager) syncServices() {
	items := m.factory.Core().V1().Services().Informer().GetStore().List()
	svcs := make([]ServiceInfo, 0, len(items))
	for _, obj := range items {
		svc, ok := obj.(*corev1.Service)
		if !ok {
			continue
		}

		portParts := make([]string, len(svc.Spec.Ports))
		for i, p := range svc.Spec.Ports {
			portParts[i] = fmt.Sprintf("%d/%s", p.Port, p.Protocol)
		}
		ports := strings.Join(portParts, ",")

		svcs = append(svcs, ServiceInfo{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Type:      string(svc.Spec.Type),
			ClusterIP: svc.Spec.ClusterIP,
			Ports:     ports,
			Age:       formatAge(svc.CreationTimestamp.Time),
		})
	}
	m.cache.SetServices(svcs)
}

func (m *InformerManager) syncEvents() {
	items := m.factory.Core().V1().Events().Informer().GetStore().List()
	events := make([]EventInfo, 0, len(items))
	for _, obj := range items {
		e, ok := obj.(*corev1.Event)
		if !ok {
			continue
		}

		age := formatAge(e.LastTimestamp.Time)
		if e.LastTimestamp.IsZero() {
			age = formatAge(e.CreationTimestamp.Time)
		}

		events = append(events, EventInfo{
			Type:      e.Type,
			Reason:    e.Reason,
			Message:   e.Message,
			Object:    e.InvolvedObject.Kind + "/" + e.InvolvedObject.Name,
			Namespace: e.Namespace,
			Age:       age,
			Count:     e.Count,
		})
	}
	m.cache.SetEvents(events)
}

func (m *InformerManager) syncMetrics() {
	nodeItems := m.factory.Core().V1().Nodes().Informer().GetStore().List()
	podItems := m.factory.Core().V1().Pods().Informer().GetStore().List()

	var totalCPU, totalMem float64
	var oldestNode time.Time
	for _, obj := range nodeItems {
		n, ok := obj.(*corev1.Node)
		if !ok {
			continue
		}
		totalCPU += n.Status.Allocatable.Cpu().AsApproximateFloat64()
		totalMem += float64(n.Status.Allocatable.Memory().Value()) / (1024 * 1024 * 1024)
		if oldestNode.IsZero() || n.CreationTimestamp.Time.Before(oldestNode) {
			oldestNode = n.CreationTimestamp.Time
		}
	}

	var running, pending, failed int
	var cpuRequested, memRequested float64
	for _, obj := range podItems {
		p, ok := obj.(*corev1.Pod)
		if !ok {
			continue
		}
		switch p.Status.Phase { //nolint:exhaustive // only counting running/pending/failed
		case corev1.PodRunning:
			running++
			for _, c := range p.Spec.Containers {
				cpuRequested += c.Resources.Requests.Cpu().AsApproximateFloat64()
				memRequested += float64(c.Resources.Requests.Memory().Value()) / (1024 * 1024 * 1024)
			}
		case corev1.PodPending:
			pending++
		case corev1.PodFailed:
			failed++
		default:
			// PodSucceeded, PodUnknown — not counted
		}
	}

	cpuPct := 0.0
	if totalCPU > 0 {
		cpuPct = (cpuRequested / totalCPU) * 100
	}
	memPct := 0.0
	if totalMem > 0 {
		memPct = (memRequested / totalMem) * 100
	}

	m.cache.SetMetrics(&MetricsInfo{
		Nodes:       len(nodeItems),
		CPUCores:    totalCPU,
		CPUPercent:  cpuPct,
		MemoryGB:    totalMem,
		MemPercent:  memPct,
		TotalPods:   len(podItems),
		RunningPods: running,
		PendingPods: pending,
		FailedPods:  failed,
		Uptime:      formatAge(oldestNode),
	})
}

// formatAge returns a human-readable age string like "3d", "5h", "2m".
func formatAge(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
}
