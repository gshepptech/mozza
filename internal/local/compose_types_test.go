package local_test

// composeFile mirrors the internal compose struct for test unmarshaling.
type composeFile struct {
	Services map[string]composeService `yaml:"services"`
	Volumes  map[string]struct{}       `yaml:"volumes,omitempty"`
	Networks map[string]composeNetwork `yaml:"networks,omitempty"`
}

// composeService mirrors the internal service struct for test unmarshaling.
type composeService struct {
	Image           string            `yaml:"image"`
	Command         string            `yaml:"command,omitempty"`
	User            string            `yaml:"user,omitempty"`
	ReadOnly        bool              `yaml:"read_only,omitempty"`
	Ports           []string          `yaml:"ports,omitempty"`
	Expose          []string          `yaml:"expose,omitempty"`
	Environment     map[string]string `yaml:"environment,omitempty"`
	DependsOn       interface{}       `yaml:"depends_on,omitempty"`
	Volumes         []string          `yaml:"volumes,omitempty"`
	Networks        []string          `yaml:"networks,omitempty"`
	NetworkMode     string            `yaml:"network_mode,omitempty"`
	Restart         string            `yaml:"restart,omitempty"`
	Deploy          *composeDeploy    `yaml:"deploy,omitempty"`
	Healthcheck     *composeHealth    `yaml:"healthcheck,omitempty"`
	Labels          map[string]string `yaml:"labels,omitempty"`
	CapDrop         []string          `yaml:"cap_drop,omitempty"`
	CapAdd          []string          `yaml:"cap_add,omitempty"`
	StopGracePeriod string            `yaml:"stop_grace_period,omitempty"`
}

// dependsOnList extracts depends_on as a string slice (for simple cases).
func dependsOnList(dep interface{}) []string {
	if dep == nil {
		return nil
	}
	switch v := dep.(type) {
	case []interface{}:
		var result []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return v
	}
	return nil
}

// dependsOnMap extracts depends_on as a map with conditions (for structured cases).
func dependsOnMap(dep interface{}) map[string]string {
	if dep == nil {
		return nil
	}
	m, ok := dep.(map[string]interface{})
	if !ok {
		return nil
	}
	result := make(map[string]string)
	for k, v := range m {
		if inner, ok := v.(map[string]interface{}); ok {
			if cond, ok := inner["condition"].(string); ok {
				result[k] = cond
			}
		}
	}
	return result
}

// composeDeploy mirrors the internal deploy struct for test unmarshaling.
type composeDeploy struct {
	Replicas  int                    `yaml:"replicas,omitempty"`
	Resources *composeDeployResource `yaml:"resources,omitempty"`
}

// composeDeployResource mirrors the internal deploy resources struct for test unmarshaling.
type composeDeployResource struct {
	Limits composeLimits `yaml:"limits"`
}

// composeLimits holds CPU and memory limits for test unmarshaling.
type composeLimits struct {
	CPUs   string `yaml:"cpus,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

// composeHealth mirrors the internal healthcheck struct for test unmarshaling.
type composeHealth struct {
	Test        []string `yaml:"test"`
	Interval    string   `yaml:"interval"`
	Timeout     string   `yaml:"timeout"`
	StartPeriod string   `yaml:"start_period,omitempty"`
	Retries     int      `yaml:"retries"`
}

// composeNetwork mirrors the internal network struct for test unmarshaling.
type composeNetwork struct {
	Driver string `yaml:"driver"`
}
