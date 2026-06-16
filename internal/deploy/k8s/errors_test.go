package k8s

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestHumanError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		context  string
		contains string
	}{
		{
			name:     "nil error",
			err:      nil,
			context:  "api",
			contains: "",
		},
		{
			name:     "not found",
			err:      apierrors.NewNotFound(schema.GroupResource{Resource: "deployments"}, "api"),
			context:  "Deployment api",
			contains: "not found",
		},
		{
			name:     "forbidden",
			err:      apierrors.NewForbidden(schema.GroupResource{Resource: "deployments"}, "api", fmt.Errorf("forbidden")),
			context:  "create Deployments",
			contains: "permission",
		},
		{
			name:     "image pull backoff",
			err:      fmt.Errorf("ImagePullBackOff for container api"),
			context:  "api",
			contains: "container image could not be found",
		},
		{
			name:     "crash loop",
			err:      fmt.Errorf("CrashLoopBackOff for pod api-xxx"),
			context:  "api",
			contains: "keeps crashing",
		},
		{
			name:     "oom killed",
			err:      fmt.Errorf("OOMKilled"),
			context:  "api",
			contains: "ran out of memory",
		},
		{
			name:     "unknown error passes through",
			err:      fmt.Errorf("something unexpected"),
			context:  "api",
			contains: "something unexpected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := HumanError(tt.err, tt.context)
			if tt.contains == "" {
				assert.Empty(t, result)
			} else {
				assert.Contains(t, result, tt.contains)
			}
		})
	}
}
