package gpufit

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fwk "k8s.io/kube-scheduler/framework"
)

const (
	PluginName = "GPUFit"
)

type GPUFit struct {
	handle fwk.Handle
}

var _ fwk.FilterPlugin = &GPUFit{}
var _ fwk.ScorePlugin = &GPUFit{}

func (pl *GPUFit) Name() string {
	return PluginName
}

func (pl *GPUFit) Filter(ctx context.Context, state fwk.CycleState, pod *corev1.Pod, nodeInfo fwk.NodeInfo) *fwk.Status {

	requestedGPU := pod.Spec.Containers[0].Resources.Requests["nvidia.com/gpu"]
	if requestedGPU.IsZero() {
		return fwk.NewStatus(fwk.Success, "")
	}

	allocatableGPUNum := nodeInfo.GetAllocatable().GetScalarResources()["nvidia.com/gpu"]
	requestedOnNodeNum := nodeInfo.GetRequested().GetScalarResources()["nvidia.com/gpu"]

	if allocatableGPUNum < requestedOnNodeNum {
		return fwk.NewStatus(fwk.Unschedulable,
			fmt.Sprintf("insufficient GPU: available %d, requested %d", allocatableGPUNum, requestedOnNodeNum))
	}

	return fwk.NewStatus(fwk.Success, "")
}

func (pl *GPUFit) Score(ctx context.Context, state fwk.CycleState, pod *corev1.Pod, nodeInfo fwk.NodeInfo) (int64, *fwk.Status) {

	nodeInfo, err := pl.handle.SnapshotSharedLister().NodeInfos().Get(nodeInfo.Node().Name)
	if err != nil {
		return 0, fwk.NewStatus(fwk.Error, fmt.Sprintf("failed to get node info: %v", err))
	}

	total := nodeInfo.GetAllocatable().GetScalarResources()["nvidia.com/gpu"]
	if total == 0 {
		return 0, fwk.NewStatus(fwk.Success, "")
	}

	usedNum := nodeInfo.GetRequested().GetScalarResources()["nvidia.com/gpu"]

	utilization := float64(usedNum) / float64(total)

	score := int64(utilization * 100)

	return score, fwk.NewStatus(fwk.Success, "")
}

func (pl *GPUFit) ScoreExtensions() fwk.ScoreExtensions {
	return nil
}

func New(ctx context.Context, obj runtime.Object, handle fwk.Handle) (fwk.Plugin, error) {
	return &GPUFit{handle: handle}, nil
}
