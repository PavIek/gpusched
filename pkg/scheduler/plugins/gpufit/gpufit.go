package gpufit

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	fwk "k8s.io/kube-scheduler/framework"
)

const (
	PluginName = "GPUFit"
)

type GPUFit struct {
	logger           klog.Logger
	frameworkHandler fwk.Handle
}

// var _ fwk.FilterPlugin = &GPUFit{}
// var _ fwk.ScorePlugin = &GPUFit{}

// var _ fwk.PreFilterPlugin = &GPUFit{}
var _ fwk.EnqueueExtensions = &GPUFit{}

func (pl *GPUFit) Name() string {
	return PluginName
}

// func (pl *GPUFit) Filter(ctx context.Context, state fwk.CycleState, pod *corev1.Pod, nodeInfo fwk.NodeInfo) *fwk.Status {

// 	// requestedGPU := pod.Spec.Containers[0].Resources.Requests["nvidia.com/gpu"]
// 	// if requestedGPU.IsZero() {
// 	// 	return fwk.NewStatus(fwk.Success, "")
// 	// }

// 	// allocatableGPUNum := nodeInfo.GetAllocatable().GetScalarResources()["nvidia.com/gpu"]
// 	// requestedOnNodeNum := nodeInfo.GetRequested().GetScalarResources()["nvidia.com/gpu"]

// 	// if allocatableGPUNum < requestedOnNodeNum {
// 	// 	return fwk.NewStatus(fwk.Unschedulable,
// 	// 		fmt.Sprintf("insufficient GPU: available %d, requested %d", allocatableGPUNum, requestedOnNodeNum))
// 	// }

// 	return fwk.NewStatus(fwk.Success, "")
// }

// func (pl *GPUFit) Score(ctx context.Context, state fwk.CycleState, pod *corev1.Pod, nodeInfo fwk.NodeInfo) (int64, *fwk.Status) {

// 	// nodeInfo, err := pl.handle.SnapshotSharedLister().NodeInfos().Get(nodeInfo.Node().Name)
// 	// if err != nil {
// 	// 	return 0, fwk.NewStatus(fwk.Error, fmt.Sprintf("failed to get node info: %v", err))
// 	// }

// 	// total := nodeInfo.GetAllocatable().GetScalarResources()["nvidia.com/gpu"]
// 	// if total == 0 {
// 	// 	return 0, fwk.NewStatus(fwk.Success, "")
// 	// }
// 	// total = 2

// 	// usedNum := nodeInfo.GetRequested().GetScalarResources()["nvidia.com/gpu"]

// 	// usedNum = 0

// 	// utilization := float64(usedNum) / float64(total)

// 	//score := int64(utilization * 100)
// 	var score int64 = 100

// 	return score, fwk.NewStatus(fwk.Success, "")
// }

// func (pl *GPUFit) ScoreExtensions() fwk.ScoreExtensions {
// 	return nil
// }

func New(ctx context.Context, obj runtime.Object, handle fwk.Handle) (fwk.Plugin, error) {
	lh := klog.FromContext(ctx).WithValues("plugin", PluginName)
	lh.V(5).Info("creating new GPUFit plugin")

	plugin := &GPUFit{
		logger:           lh,
		frameworkHandler: handle,
	}

	return plugin, nil
}

// func (pl *GPUFit) PreFilter(ctx context.Context, state fwk.CycleState, pod *v1.Pod, nodes []fwk.NodeInfo) (*fwk.PreFilterResult, *fwk.Status) {
// 	return nil, fwk.NewStatus(fwk.Success, "")
// }

func (pl *GPUFit) EventsToRegister(_ context.Context) ([]fwk.ClusterEventWithHint, error) {
	// To register a custom event, follow the naming convention at:
	// https://github.com/kubernetes/kubernetes/pull/101394
	// Please follow: eventhandlers.go#L403-L410
	eqGVK := fmt.Sprintf("GPUFit.%s", "scheduler")
	return []fwk.ClusterEventWithHint{
		{Event: fwk.ClusterEvent{Resource: fwk.Pod, ActionType: fwk.Delete}},
		{Event: fwk.ClusterEvent{Resource: fwk.EventResource(eqGVK), ActionType: fwk.All}},
	}, nil
}

// PreFilterExtensions returns a PreFilterExtensions interface if the plugin implements one.
// func (pl *GPUFit) PreFilterExtensions() fwk.PreFilterExtensions {
// 	return nil
// }
