package statusprojection

import (
	"context"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func newCR() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "composition.krateo.io/v1-2-0",
		"kind":       "FireworksApp",
		"metadata":   map[string]any{"name": "demo", "namespace": "apps", "generation": int64(3)},
		"spec": map[string]any{
			"service":  map[string]any{"host": "demo.example.com", "port": int64(8080)},
			"replicas": int64(3),
		},
	}}
}

func statusOf(t *testing.T, cr *unstructured.Unstructured, path ...string) any {
	t.Helper()
	v, found, err := unstructured.NestedFieldNoCopy(cr.Object, append([]string{"status"}, path...)...)
	if err != nil || !found {
		t.Fatalf("status.%v not found (found=%v err=%v)", path, found, err)
	}
	return v
}

func TestProject_BuiltinsAndShapes(t *testing.T) {
	cr := newCR()
	resolved := map[string]any{
		"helm": map[string]any{"version": "1.2.0", "status": "deployed"},
		"api": map[string]any{
			"svc": map[string]any{"items": []any{
				map[string]any{
					"metadata": map[string]any{"name": "demo-web"},
					"status":   map[string]any{"loadBalancer": map[string]any{"ingress": []any{map[string]any{"ip": "34.120.55.10"}}}},
				},
			}},
			"deploys": map[string]any{"items": []any{
				map[string]any{"spec": map[string]any{"replicas": int64(3)}, "status": map[string]any{"readyReplicas": int64(3)}},
				map[string]any{"spec": map[string]any{"replicas": int64(2)}, "status": map[string]any{"readyReplicas": int64(1)}},
			}},
		},
	}

	mappings := []Mapping{
		{ForPath: "url", Expression: `${ "https://\(.self.spec.service.host):\(.self.spec.service.port)" }`},
		{ForPath: "chartVersion", Expression: `${ .helm.version }`},
		{ForPath: "specReplicas", Expression: `${ .spec.replicas }`}, // spec sugar
		{ForPath: "endpoint", Expression: `${ .api.svc.items[0].status.loadBalancer.ingress[0].ip }`},
		{ForPath: "readyReplicas", Expression: `${ [ .api.deploys.items[].status.readyReplicas // 0 ] | add }`},
		{ForPath: "ready", Expression: `${ (.helm.status == "deployed") and ([ .api.deploys.items[] | .status.readyReplicas == .spec.replicas ] | all) }`},
		{ForPath: "network.host", Expression: `${ .self.spec.service.host }`}, // nested forPath
		{ForPath: "endpoints", Expression: `${ [ .api.svc.items[] | { name: .metadata.name, ip: .status.loadBalancer.ingress[0].ip } ] }`},
	}

	if err := Project(context.Background(), cr, resolved, mappings); err != nil {
		t.Fatalf("Project: %v", err)
	}

	if got := statusOf(t, cr, "url"); got != "https://demo.example.com:8080" {
		t.Errorf("url = %v", got)
	}
	if got := statusOf(t, cr, "chartVersion"); got != "1.2.0" {
		t.Errorf("chartVersion = %v", got)
	}
	// integer must be int64 (InferType yields int32; normalize must widen) or SetNestedField panics.
	if got := statusOf(t, cr, "specReplicas"); got != int64(3) {
		t.Errorf("specReplicas = %v (%T), want int64(3)", got, got)
	}
	if got := statusOf(t, cr, "endpoint"); got != "34.120.55.10" {
		t.Errorf("endpoint = %v", got)
	}
	if got := statusOf(t, cr, "readyReplicas"); got != int64(4) {
		t.Errorf("readyReplicas = %v (%T), want int64(4)", got, got)
	}
	if got := statusOf(t, cr, "ready"); got != false { // deploy #2 not ready
		t.Errorf("ready = %v, want false", got)
	}
	if got := statusOf(t, cr, "network", "host"); got != "demo.example.com" {
		t.Errorf("network.host = %v", got)
	}
	endpoints := statusOf(t, cr, "endpoints")
	want := []any{map[string]any{"name": "demo-web", "ip": "34.120.55.10"}}
	if !reflect.DeepEqual(endpoints, want) {
		t.Errorf("endpoints = %#v, want %#v", endpoints, want)
	}
}

func TestProject_LiteralExpression(t *testing.T) {
	cr := newCR()
	if err := Project(context.Background(), cr, nil, []Mapping{{ForPath: "region", Expression: "eu-west"}}); err != nil {
		t.Fatalf("Project: %v", err)
	}
	if got := statusOf(t, cr, "region"); got != "eu-west" {
		t.Errorf("region = %v", got)
	}
}

func TestProject_PerMappingErrorIsolation(t *testing.T) {
	cr := newCR()
	mappings := []Mapping{
		{ForPath: "good", Expression: `${ .self.spec.service.host }`},
		{ForPath: "bad", Expression: `${ .foo | (( }`},
	}
	err := Project(context.Background(), cr, nil, mappings)
	if err == nil {
		t.Fatal("expected an aggregated error for the bad mapping")
	}
	// the good mapping must still have been applied
	if got := statusOf(t, cr, "good"); got != "demo.example.com" {
		t.Errorf("good = %v; a bad mapping must not block the others", got)
	}
}

// A caller-supplied `resolved` source containing a non-JSON-safe value (here a plain Go int,
// which runtime.DeepCopyJSONValue panics on) must degrade to an aggregated error rather than
// crashing the worker goroutine. The whole root is deep-copied per mapping, so a globally bad
// source fails every mapping; the core guarantee under test is "returns an error, does not
// panic". A second JSON-safe mapping is included to confirm Project still runs to completion.
func TestProject_NonJSONSafeSourceDoesNotPanic(t *testing.T) {
	cr := newCR()
	resolved := map[string]any{
		"helm": map[string]any{"bad": 5}, // plain int, not int64 -> DeepCopyJSONValue panics
	}
	mappings := []Mapping{
		{ForPath: "host", Expression: `${ .self.spec.service.host }`}, // JSON-safe mapping
		{ForPath: "version", Expression: `${ .helm.bad }`},
	}
	err := Project(context.Background(), cr, resolved, mappings)
	if err == nil {
		t.Fatal("expected an aggregated error for the non-JSON-safe source")
	}
}

func TestSetObservedGeneration(t *testing.T) {
	cr := newCR()
	if err := SetObservedGeneration(cr); err != nil {
		t.Fatalf("SetObservedGeneration: %v", err)
	}
	if got := statusOf(t, cr, "observedGeneration"); got != int64(3) {
		t.Errorf("observedGeneration = %v (%T), want int64(3)", got, got)
	}
}

func TestNormalize_DeepCopySafe(t *testing.T) {
	// int32 (what InferType emits for small ints) must become int64; nested too.
	in := map[string]any{"a": int32(5), "b": []any{int(7), float32(1.5)}, "c": "x", "d": true}
	out := normalize(in).(map[string]any)
	if out["a"] != int64(5) {
		t.Errorf("a = %v (%T)", out["a"], out["a"])
	}
	arr := out["b"].([]any)
	if arr[0] != int64(7) || arr[1] != float64(1.5) {
		t.Errorf("b = %#v", arr)
	}
}

// A jq program that yields no output (empty-array iteration) must NOT write the field —
// writing "" would violate a non-string schema type and fail the whole status update.
func TestProject_EmptyStreamSkipsWrite(t *testing.T) {
	cr := newCR()
	cr.Object["spec"].(map[string]any)["empty"] = []any{}
	mappings := []Mapping{
		{ForPath: "skipped", Expression: `${ .self.spec.empty[] }`},   // empty stream -> skip
		{ForPath: "emptystr", Expression: `${ "" }`},                  // legit empty string -> write
		{ForPath: "kept", Expression: `${ .self.spec.service.host }`}, // sanity
	}
	if err := Project(context.Background(), cr, nil, mappings); err != nil {
		t.Fatalf("Project: %v", err)
	}
	if _, found, _ := unstructured.NestedFieldNoCopy(cr.Object, "status", "skipped"); found {
		t.Error("empty-stream mapping must not write the field")
	}
	if v, found, _ := unstructured.NestedFieldNoCopy(cr.Object, "status", "emptystr"); !found || v != "" {
		t.Errorf("empty-string result should be written as \"\": found=%v v=%v", found, v)
	}
	if v, _, _ := unstructured.NestedString(cr.Object, "status", "kept"); v != "demo.example.com" {
		t.Errorf("kept = %q", v)
	}
}
