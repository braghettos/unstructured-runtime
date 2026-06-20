// Package statusprojection projects values onto an unstructured CR's .status by evaluating
// declarative ${ jq } expressions over a combined "source root", reusing plumbing/jqutil
// (the same engine snowplow uses for widgetDataTemplate).
//
// It is the shared runtime half of Krateo's "status projection" design: a provider
// (composition-dynamic-controller, rest-dynamic-controller) supplies the CR, the resolved
// source documents, and a list of mappings; this package evaluates and writes status. It is
// pure — it performs no client calls; any I/O-bound source (e.g. RESTAction results) is
// resolved by the caller and passed in via `resolved`.
package statusprojection

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/krateoplatformops/plumbing/jqutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// Mapping mirrors snowplow's widgetDataTemplate item (decoupled from any provider CRD):
// write the (typed) result of Expression to .status at ForPath. The json tags are the wire
// format used to ship mappings between components (e.g. core-provider → the CDC ConfigMap).
type Mapping struct {
	// ForPath is the dotted path under .status to write, e.g. "endpoint" or "network.host".
	ForPath string `json:"forPath"`
	// Expression is a ${ jq } program evaluated over the source root. A bare literal (no
	// ${ } wrapper) is used verbatim. A bare path like ".self.spec.host" is the trivial copy.
	Expression string `json:"expression"`
}

// Project evaluates each mapping's Expression over the combined source root and writes the
// typed, normalized result into cr's .status at ForPath.
//
// The root is `resolved` plus the built-ins derived from cr: "self" (the whole object),
// "spec" and "status" (sugar for self.spec / self.status). Callers typically pass
// resolved={"helm": ..., "api": ...}. Numbers/objects/arrays are normalized to
// DeepCopyJSONValue-safe types before writing (jqutil.InferType can return int32, which
// unstructured.SetNestedField panics on).
//
// Errors are per-mapping and aggregated: a failing mapping degrades that one field; the
// others still apply. Returns a joined error (nil if all succeeded).
func Project(ctx context.Context, cr *unstructured.Unstructured, resolved map[string]any, mappings []Mapping) error {
	if cr == nil {
		return fmt.Errorf("nil cr")
	}
	root := buildRoot(cr, resolved)

	var errs []error
	for _, m := range mappings {
		if m.ForPath == "" {
			errs = append(errs, fmt.Errorf("mapping with empty forPath"))
			continue
		}
		// gojq normalizes numbers in its input map IN PLACE, so never hand it the live
		// data: evaluate against a fresh deep copy each time (also isolates mappings from
		// one another and keeps cr / resolved untouched apart from the status writes).
		val, err := evalOne(ctx, m.Expression, runtime.DeepCopyJSONValue(root))
		if err != nil {
			errs = append(errs, fmt.Errorf("forPath %q: %w", m.ForPath, err))
			continue
		}
		if err := setStatusField(cr, m.ForPath, val); err != nil {
			errs = append(errs, fmt.Errorf("forPath %q: %w", m.ForPath, err))
		}
	}
	return errors.Join(errs...)
}

// SetObservedGeneration writes status.observedGeneration = metadata.generation.
func SetObservedGeneration(cr *unstructured.Unstructured) error {
	if cr == nil {
		return fmt.Errorf("nil cr")
	}
	return unstructured.SetNestedField(cr.Object, cr.GetGeneration(), "status", "observedGeneration")
}

// buildRoot assembles the jq evaluation root: the caller's resolved sources, plus the
// built-in self/spec/status views of the CR.
func buildRoot(cr *unstructured.Unstructured, resolved map[string]any) map[string]any {
	root := make(map[string]any, len(resolved)+3)
	for k, v := range resolved {
		root[k] = v
	}
	root["self"] = cr.Object
	if spec, ok := cr.Object["spec"]; ok {
		root["spec"] = spec
	}
	if status, ok := cr.Object["status"]; ok {
		root["status"] = status
	}
	return root
}

// evalOne resolves a single expression. A ${ jq } expression is evaluated over data; a bare
// literal is taken verbatim. The result string is typed via jqutil.InferType and then
// normalized to DeepCopyJSONValue-safe types.
func evalOne(ctx context.Context, expression string, data any) (any, error) {
	s := expression
	if q, ok := jqutil.MaybeQuery(expression); ok {
		out, err := jqutil.Eval(ctx, jqutil.EvalOptions{Query: q, Data: data})
		if err != nil {
			return nil, err
		}
		s = out
	}
	return normalize(jqutil.InferType(s)), nil
}

// setStatusField writes val at status.<dotted forPath>, building intermediate objects.
func setStatusField(cr *unstructured.Unstructured, forPath string, val any) error {
	fields := append([]string{"status"}, strings.Split(forPath, ".")...)
	return unstructured.SetNestedField(cr.Object, val, fields...)
}

// normalize converts a value to types accepted by runtime.DeepCopyJSONValue (the strict
// type switch behind unstructured.SetNestedField): map[string]any, []any, string, int64,
// float64, bool, nil. In particular jqutil.InferType returns int32 for small integers and
// may leave json.Number nested inside maps/slices — both are coerced here.
func normalize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, e := range x {
			m[k] = normalize(e)
		}
		return m
	case []any:
		s := make([]any, len(x))
		for i, e := range x {
			s[i] = normalize(e)
		}
		return s
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case int64, float64, string, bool, nil:
		return x
	case float32:
		return float64(x)
	default:
		// json.Number and any other stringer-ish numeric: prefer int64 then float64.
		if n, ok := v.(interface{ Int64() (int64, error) }); ok {
			if i, err := n.Int64(); err == nil {
				return i
			}
		}
		if n, ok := v.(interface{ Float64() (float64, error) }); ok {
			if f, err := n.Float64(); err == nil {
				return f
			}
		}
		return fmt.Sprintf("%v", x)
	}
}
