package k8s

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
)

// Standard Kubernetes recommended labels.
const (
	labelApp       = "app.kubernetes.io/name"
	labelComponent = "app.kubernetes.io/component"
	labelPartOf    = "app.kubernetes.io/part-of"
	labelManagedBy = "app.kubernetes.io/managed-by"
)

// labels builds the standard Kubernetes label set for a slice within an application.
// name=appName, component=sliceName, part-of=appName, managed-by=mozza.
func labels(appName, sliceName string) map[string]string {
	return map[string]string{
		labelApp:       appName,
		labelComponent: sliceName,
		labelPartOf:    appName,
		labelManagedBy: "mozza",
	}
}

// marshalObject serializes a K8s runtime.Object to YAML bytes.
func marshalObject(obj runtime.Object) ([]byte, error) {
	serializer := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	buf := &yamlBuffer{}
	buf.WriteString("---\n")
	if err := serializer.Encode(obj, buf); err != nil {
		return nil, fmt.Errorf("marshalObject: %w", err)
	}
	return buf.Bytes(), nil
}

// yamlBuffer is a simple byte buffer that implements io.Writer.
type yamlBuffer struct {
	data []byte
}

// Write implements io.Writer.
func (b *yamlBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

// WriteString appends a string to the buffer.
func (b *yamlBuffer) WriteString(s string) {
	b.data = append(b.data, s...)
}

// Bytes returns the buffer contents.
func (b *yamlBuffer) Bytes() []byte {
	return b.data
}
