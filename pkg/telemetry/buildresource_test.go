package telemetry

import "testing"

func TestBuildResource(t *testing.T) {
	res, err := buildResource(Config{
		ServiceName:    "cdc",
		Namespace:      "krateo-system",
		Version:        "1.3.0",
		CompositionID:  "x.krateo.io/v1/foo",
		DeploymentName: "cdc-abc",
	})
	if err != nil {
		t.Fatalf("buildResource: %v", err)
	}
	got := map[string]string{}
	for _, kv := range res.Attributes() {
		got[string(kv.Key)] = kv.Value.AsString()
	}
	for k, want := range map[string]string{
		"service.name":             "cdc",
		"service.namespace":        "krateo-system",
		"service.version":          "1.3.0",
		"krateo.io/composition-id": "x.krateo.io/v1/foo",
		"k8s.deployment.name":      "cdc-abc",
		"service.instance.id":      "cdc-abc",
	} {
		if got[k] != want {
			t.Errorf("attr %q = %q, want %q", k, got[k], want)
		}
	}
}

func TestBuildResourceOmitsEmpty(t *testing.T) {
	res, err := buildResource(Config{ServiceName: "x"})
	if err != nil {
		t.Fatalf("buildResource: %v", err)
	}
	for _, kv := range res.Attributes() {
		switch string(kv.Key) {
		case "service.namespace", "service.version", "krateo.io/composition-id", "k8s.deployment.name":
			t.Errorf("attr %q should be omitted when its Config field is empty", kv.Key)
		}
	}
}
