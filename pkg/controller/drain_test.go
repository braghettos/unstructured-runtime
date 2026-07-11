package controller

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/krateoplatformops/plumbing/shortid"
	"github.com/krateoplatformops/unstructured-runtime/pkg/logging"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/util/workqueue"
)

func durPtr(d time.Duration) *time.Duration { return &d }

// drainTestOptions builds controller Options backed by a fake dynamic client seeded with the given
// objects, so the informer lists them on startup and a worker actually reconciles them.
func drainTestOptions(seed []runtime.Object, timeout *time.Duration) Options {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	gvr := schema.GroupVersionResource{Group: "test.example.org", Version: "v1", Resource: "tests"}
	gvk := schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "Test"}
	listGVK := schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "TestList"}
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(listGVK, &unstructured.UnstructuredList{})
	listKinds := map[schema.GroupVersionResource]string{gvr: listGVK.Kind}

	return Options{
		Client:                  dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, seed...),
		GVR:                     gvr,
		Namespace:               "test-namespace",
		ResyncInterval:          30 * time.Second,
		Recorder:                &mockEventRecorder{},
		ThrottledRecorder:       &mockEventRecorder{},
		Logger:                  logging.NewNopLogger(),
		Pluralizer:              &fakePluralizer{},
		GlobalRateLimiter:       workqueue.DefaultTypedControllerRateLimiter[any](),
		GracefulShutdownTimeout: timeout,
	}
}

func newDrainController(t *testing.T, opts Options, ext ExternalClient) *Controller {
	t.Helper()
	sid, err := shortid.New(1, shortid.DefaultABC, 2342)
	require.NoError(t, err)
	c, err := New(sid, opts)
	require.NoError(t, err)
	c.SetExternalClient(ext)
	return c
}

// TestRun_Drain_FinishesInFlightReconcileWithLiveContext is the core proof of the graceful drain:
// when SIGTERM arrives while a reconcile is in flight, Run does NOT return until that reconcile
// finishes, and the reconcile's context is NOT cancelled by the signal — so its API writes complete.
func TestRun_Drain_FinishesInFlightReconcileWithLiveContext(t *testing.T) {
	obj := createTestUnstructured("drain-obj", "test-namespace")

	reconcileStarted := make(chan struct{})
	release := make(chan struct{})
	var ctxLiveDuringDrain atomic.Bool
	var observeCount atomic.Int32

	ext := &mockExternalClient{
		observeFunc: func(ctx context.Context, mg *unstructured.Unstructured) (ExternalObservation, error) {
			if observeCount.Add(1) == 1 {
				close(reconcileStarted) // we are mid-reconcile
				<-release               // block here until the test releases us (after SIGTERM)
				// Captured AFTER SIGTERM: if the reconcile ctx were the signal ctx it would be
				// cancelled here; with the decoupled ctx it is still live.
				ctxLiveDuringDrain.Store(ctx.Err() == nil)
			}
			return ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
		},
	}
	c := newDrainController(t, drainTestOptions([]runtime.Object{obj}, durPtr(10*time.Second)), ext)

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- c.Run(ctx, 1) }()

	select {
	case <-reconcileStarted:
	case <-time.After(15 * time.Second):
		t.Fatal("reconcile never started")
	}

	cancel() // SIGTERM

	// Run must keep the process alive while the in-flight reconcile is unfinished.
	select {
	case <-runDone:
		t.Fatal("Run returned before the in-flight reconcile finished — no drain")
	case <-time.After(300 * time.Millisecond):
	}

	close(release) // let the reconcile complete

	select {
	case err := <-runDone:
		require.NoError(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("Run did not return after the reconcile finished")
	}
	if !ctxLiveDuringDrain.Load() {
		t.Fatal("reconcile ctx was cancelled by SIGTERM — decoupling failed, in-flight writes would be severed")
	}
}

// TestRun_Drain_RequeueStormNoWedge is the controller-level regression for the spin()/shutdown
// wedge the review caught. A reconcile that errors triggers a requeue, so spin() continuously hands
// out items to churning workers; on SIGTERM the queue ShutDown races with an in-flight hand-off.
// "Wait forever" (timeout < 0) is used deliberately: Run only returns if the WaitGroup actually
// completes, so a wedged worker (blocked on a queue mutex in Done()) would hang the test.
func TestRun_Drain_RequeueStormNoWedge(t *testing.T) {
	var seed []runtime.Object
	for i := 0; i < 8; i++ {
		seed = append(seed, createTestUnstructured(fmt.Sprintf("storm-%d", i), "test-namespace"))
	}
	ext := &mockExternalClient{
		observeFunc: func(ctx context.Context, mg *unstructured.Unstructured) (ExternalObservation, error) {
			return ExternalObservation{}, fmt.Errorf("transient error to force a requeue")
		},
	}
	c := newDrainController(t, drainTestOptions(seed, durPtr(-1)), ext) // wait forever

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- c.Run(ctx, 4) }()

	time.Sleep(500 * time.Millisecond) // let the requeue storm churn across 4 workers
	cancel()

	select {
	case err := <-runDone:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after SIGTERM under a requeue storm — spin()/shutdown wedge")
	}
}

// TestRun_Drain_IdleExitsPromptly proves an idle controller (no in-flight work) exits fast on
// SIGTERM instead of waiting out the full grace period — the parked workers are woken by the queue
// shutdown and return immediately.
func TestRun_Drain_IdleExitsPromptly(t *testing.T) {
	c := newDrainController(t, drainTestOptions(nil, durPtr(30*time.Second)), &mockExternalClient{})

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- c.Run(ctx, 2) }()

	// Give it a moment to reach the ready state, then SIGTERM.
	time.Sleep(300 * time.Millisecond)
	cancel()

	select {
	case err := <-runDone:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("idle Run did not exit promptly on SIGTERM (waited out the 30s grace period?)")
	}
}

// TestRun_Drain_TimeoutCancelsStuckReconcile proves the bounded backstop: a reconcile that runs past
// the grace period is cancelled (its ctx fires) and Run returns within roughly the timeout rather
// than hanging forever.
func TestRun_Drain_TimeoutCancelsStuckReconcile(t *testing.T) {
	obj := createTestUnstructured("stuck-obj", "test-namespace")

	reconcileStarted := make(chan struct{})
	reconcileCancelled := make(chan struct{}) // closed when the stuck reconcile observes ctx cancellation
	var once atomic.Bool

	ext := &mockExternalClient{
		observeFunc: func(ctx context.Context, mg *unstructured.Unstructured) (ExternalObservation, error) {
			if once.CompareAndSwap(false, true) {
				close(reconcileStarted)
				<-ctx.Done() // honor ctx: unblock only when the drain cancels us past the grace period
				close(reconcileCancelled)
			}
			return ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
		},
	}
	c := newDrainController(t, drainTestOptions([]runtime.Object{obj}, durPtr(200*time.Millisecond)), ext)

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- c.Run(ctx, 1) }()

	select {
	case <-reconcileStarted:
	case <-time.After(15 * time.Second):
		t.Fatal("reconcile never started")
	}

	start := time.Now()
	cancel() // SIGTERM; reconcile is stuck, grace period is 200ms

	select {
	case err := <-runDone:
		require.NoError(t, err)
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Fatalf("Run took %s to return; grace period backstop did not fire", elapsed)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("Run hung past the grace period — bounded backstop failed")
	}
	// The backstop must have cancelled the stuck reconcile's context (it may observe the
	// cancellation just after Run returns, so wait bounded rather than reading immediately).
	select {
	case <-reconcileCancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("stuck reconcile ctx was never cancelled by the grace-period backstop")
	}
}
