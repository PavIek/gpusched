GpuSched MVP 详细实施计划
项目名称：GpuSched — Kubernetes GPU 智能调度器
项目定位：一个支持 GPU 资源感知调度（Filter + Score）的 Kubernetes 自定义调度器，通过 CRD + Controller + Scheduler Plugin 三组件协同工作
适用场景：AI 训练作业的 GPU 资源管理与调度
预计工期：6-8 周

一、项目概述
1.1 整体架构
GpuSched 由三个核心组件构成，严格遵循 Kubernetes 控制器模式：

text
┌─────────────────────────────────────────────────────────────────┐
│                         Control Plane                          │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐    ┌──────────────────┐    ┌──────────────┐ │
│  │   GPUJob CRD  │    │  Controller      │    │  Scheduler   │ │
│  │  (API 定义)   │◄──►│  (Reconcile)     │◄──►│  (Plugins)   │ │
│  └──────────────┘    └──────────────────┘    └──────────────┘ │
│                              │                        │        │
│                              ▼                        ▼        │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │              Kubernetes API Server + etcd               │ │
│  └──────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                          Data Plane                            │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                     │
│  │  Node 1  │  │  Node 2  │  │  Node N  │                     │
│  │  GPU 0-7 │  │  GPU 0-7 │  │  GPU 0-7 │                     │
│  └──────────┘  └──────────┘  └──────────┘                     │
└─────────────────────────────────────────────────────────────────┘
1.2 组件职责
组件	技术栈	职责
GPUJob CRD	Kubebuilder	用户提交 GPU 作业的声明式 API
Controller	controller-runtime	监听 GPUJob，创建对应的 Pod 并管理生命周期
Scheduler Plugin	Scheduler Framework	在调度周期中注入 GPU 感知逻辑（Filter + Score）
1.3 为什么采用 Scheduler Framework 而非 Extender？
维度	Scheduler Extender	Scheduler Framework（本方案）
实现方式	独立 HTTP 服务	原生 Go 插件，编译进调度器
性能	HTTP 调用有网络延迟	内存调用，无额外开销
调度器缓存	无法访问	可直接读取调度器缓存
官方推荐	已不推荐	官方标准方案
二、环境准备
2.1 本地开发环境
bash
# 1. Go 环境（1.21+）
go version
# 输出: go version go1.21.x linux/amd64

# 2. 安装 kubebuilder
curl -L -o kubebuilder "https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)"
chmod +x kubebuilder && sudo mv kubebuilder /usr/local/bin/

# 3. 安装 kind（本地 Kubernetes 集群）
# macOS:
brew install kind
# Linux:
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
chmod +x ./kind && sudo mv ./kind /usr/local/bin/kind

# 4. 安装 kubectl
# macOS:
brew install kubectl
# Linux:
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl && sudo mv kubectl /usr/local/bin/

# 5. 安装 helm（可选，用于部署）
# macOS:
brew install helm
# Linux:
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# 6. 安装 kube-burner（压测工具）
# 参考: https://github.com/cloud-bulldozer/kube-burner
2.2 创建 Kind 集群
bash
# 创建集群配置文件
cat <<EOF > kind-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
- role: worker
EOF

# 创建集群
kind create cluster --config kind-config.yaml --name gpu-cluster

# 验证集群状态
kubectl cluster-info --context kind-gpu-cluster
kubectl get nodes
2.3 安装 NVIDIA Device Plugin
即使在没有真实 GPU 的本地开发环境，也可以部署 Device Plugin 进行模拟测试：

bash
# 部署 NVIDIA Device Plugin（DaemonSet）
kubectl create -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.17.1/deployments/static/nvidia-device-plugin.yml

# 验证 Pod 运行状态
kubectl get pods -n kube-system | grep nvidia-device-plugin
Device Plugin 通过 gRPC 与 kubelet 通信，上报节点的 GPU 信息。它会查询 NVML 获取 GPU 数量并注册到 kubelet。

三、第一阶段：项目初始化与 CRD 定义（Week 1-2）
3.1 初始化项目
bash
# 1. 创建项目目录
mkdir gpusched && cd gpusched
go mod init github.com/yourname/gpusched

# 2. 使用 kubebuilder 初始化
kubebuilder init --domain gpusched.io

# 3. 创建 API（CRD + Controller）
kubebuilder create api --group scheduling --version v1 --kind GPUJob \
    --resource --controller
命令解释：

kubebuilder init --domain gpusched.io：初始化项目骨架，--domain 指定 API 组的域名后缀

kubebuilder create api：生成 CRD 类型定义和 Controller 脚手架

3.2 项目目录结构
text
gpusched/
├── api/
│   └── v1/
│       ├── gpujob_types.go          # GPUJob CRD 定义
│       └── groupversion_info.go     # API 组注册
├── internal/
│   └── controller/
│       └── gpujob_controller.go     # Reconcile 逻辑[reference:10]
├── cmd/
│   └── main.go                      # Controller 入口
├── config/
│   ├── crd/                         # CRD 安装清单
│   ├── rbac/                        # RBAC 权限
│   └── manager/                     # Controller Manager 配置
├── hack/                            # 辅助脚本
├── Makefile                         # 构建脚本
└── go.mod
3.3 定义 GPUJob CRD
编辑 api/v1/gpujob_types.go：

go
package v1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GPUJobSpec 定义用户期望的 GPU 作业状态
type GPUJobSpec struct {
    // 副本数（分布式训练的 Worker 数量）
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:default=1
    Replicas int32 `json:"replicas"`
    
    // 每个 Pod 需要的 GPU 数量
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:default=1
    GPUsPerPod int32 `json:"gpusPerPod"`
    
    // 容器镜像（训练脚本）
    Image string `json:"image"`
    
    // 启动命令
    // +kubebuilder:validation:Optional
    Command []string `json:"command,omitempty"`
}

// GPUJobStatus 定义 GPU 作业的当前状态
type GPUJobStatus struct {
    // 当前已调度的 Pod 数量
    Scheduled int32 `json:"scheduled,omitempty"`
    
    // 当前运行的 Pod 数量
    Running int32 `json:"running,omitempty"`
    
    // 已成功完成的 Pod 数量
    Succeeded int32 `json:"succeeded,omitempty"`
    
    // 已失败的 Pod 数量
    Failed int32 `json:"failed,omitempty"`
    
    // 作业状态: Pending, Scheduling, Running, Succeeded, Failed
    Phase string `json:"phase,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.spec.replicas`
// +kubebuilder:printcolumn:name="GPUs/Pod",type=integer,JSONPath=`.spec.gpusPerPod`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// GPUJob 是 GPU 调度作业的 API 定义
type GPUJob struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    
    Spec   GPUJobSpec   `json:"spec,omitempty"`
    Status GPUJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GPUJobList 包含多个 GPUJob
type GPUJobList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []GPUJob `json:"items"`
}

func init() {
    SchemeBuilder.Register(&GPUJob{}, &GPUJobList{})
}
3.4 生成 CRD 清单
bash
# 生成 CRD YAML
make manifests

# 安装 CRD 到集群
make install

# 验证 CRD 已安装
kubectl get crd | grep gpujobs
# 输出: gpujobs.scheduling.gpusched.io
3.5 验证：提交第一个 GPUJob
创建示例文件 config/samples/scheduling_v1_gpujob.yaml：

yaml
apiVersion: scheduling.gpusched.io/v1
kind: GPUJob
metadata:
  name: test-gpujob
spec:
  replicas: 2
  gpusPerPod: 1
  image: nvidia/cuda:11.0-base
  command: ["nvidia-smi"]
bash
# 应用示例
kubectl apply -f config/samples/scheduling_v1_gpujob.yaml

# 查看 GPUJob
kubectl get gpujobs

# 查看详情
kubectl describe gpujob test-gpujob
第一阶段验收标准：

□ kubectl get gpujobs 能正常返回结果
□ CRD 已成功注册到集群
□ 项目目录结构完整，make 命令可正常执行
四、第二阶段：Controller 实现（Week 3-4）
4.1 Controller 的 Reconcile 逻辑
Controller 的核心职责是：把 GPUJob 翻译成一组 Pod。

编辑 internal/controller/gpujob_controller.go：

go
package controller

import (
    "context"
    "fmt"
    
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/types"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
    "sigs.k8s.io/controller-runtime/pkg/log"
    
    schedulingv1 "github.com/yourname/gpusched/api/v1"
)

// GPUJobReconciler 负责调谐 GPUJob 资源
type GPUJobReconciler struct {
    client.Client
    Scheme *runtime.Scheme
}

// Reconcile 是调谐循环的核心方法[reference:12]
// 它必须满足幂等性要求——任何事件都可能反复触发它[reference:13]
func (r *GPUJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    logger := log.FromContext(ctx)
    logger.Info("Reconciling GPUJob", "namespacedName", req.NamespacedName)
    
    // 1. 获取 GPUJob 实例
    var job schedulingv1.GPUJob
    if err := r.Get(ctx, req.NamespacedName, &job); err != nil {
        if errors.IsNotFound(err) {
            // 对象已被删除，无需处理
            return ctrl.Result{}, nil
        }
        logger.Error(err, "Failed to get GPUJob")
        return ctrl.Result{}, err
    }
    
    // 2. 列出该 Job 拥有的所有 Pod
    var pods corev1.PodList
    if err := r.List(ctx, &pods, 
        client.InNamespace(req.Namespace),
        client.MatchingLabels{"gpujob-name": job.Name}); err != nil {
        logger.Error(err, "Failed to list pods")
        return ctrl.Result{}, err
    }
    
    // 3. 统计 Pod 状态
    var running, succeeded, failed int32
    for _, pod := range pods.Items {
        switch pod.Status.Phase {
        case corev1.PodRunning:
            running++
        case corev1.PodSucceeded:
            succeeded++
        case corev1.PodFailed:
            failed++
        }
    }
    
    // 4. 根据期望副本数创建/删除 Pod
    currentPods := int32(len(pods.Items))
    desiredPods := job.Spec.Replicas
    
    if currentPods < desiredPods {
        // 创建新的 Pod
        for i := currentPods; i < desiredPods; i++ {
            pod := r.newPodForJob(&job, i)
            if err := r.Create(ctx, pod); err != nil {
                logger.Error(err, "Failed to create pod")
                return ctrl.Result{}, err
            }
            logger.Info("Created pod", "pod", pod.Name)
        }
    } else if currentPods > desiredPods {
        // 删除多余的 Pod
        for i := desiredPods; i < currentPods; i++ {
            pod := &pods.Items[i]
            if err := r.Delete(ctx, pod); err != nil {
                logger.Error(err, "Failed to delete pod")
                return ctrl.Result{}, err
            }
            logger.Info("Deleted pod", "pod", pod.Name)
        }
    }
    
    // 5. 更新 Status
    job.Status.Scheduled = currentPods
    job.Status.Running = running
    job.Status.Succeeded = succeeded
    job.Status.Failed = failed
    
    // 6. 更新 Phase
    if succeeded == desiredPods {
        job.Status.Phase = "Succeeded"
    } else if failed > 0 {
        job.Status.Phase = "Failed"
    } else if running > 0 {
        job.Status.Phase = "Running"
    } else if currentPods > 0 {
        job.Status.Phase = "Scheduling"
    } else {
        job.Status.Phase = "Pending"
    }
    
    if err := r.Status().Update(ctx, &job); err != nil {
        logger.Error(err, "Failed to update status")
        return ctrl.Result{}, err
    }
    
    return ctrl.Result{}, nil
}

// newPodForJob 为 GPUJob 创建 Pod
func (r *GPUJobReconciler) newPodForJob(job *schedulingv1.GPUJob, index int32) *corev1.Pod {
    labels := map[string]string{
        "gpujob-name": job.Name,
        "gpujob-uid":  string(job.UID),
    }
    
    pod := &corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name:      fmt.Sprintf("%s-worker-%d", job.Name, index),
            Namespace: job.Namespace,
            Labels:    labels,
            OwnerReferences: []metav1.OwnerReference{
                *metav1.NewControllerRef(job, schedulingv1.GroupVersion.WithKind("GPUJob")),
            },
        },
        Spec: corev1.PodSpec{
            // 关键：指定使用自定义调度器[reference:14]
            SchedulerName: "gpusched-scheduler",
            Containers: []corev1.Container{
                {
                    Name:    "gpu-worker",
                    Image:   job.Spec.Image,
                    Command: job.Spec.Command,
                    Resources: corev1.ResourceRequirements{
                        Requests: corev1.ResourceList{
                            // 标准 GPU 资源名称，由 Device Plugin 提供[reference:15]
                            "nvidia.com/gpu": *resource.NewQuantity(
                                int64(job.Spec.GPUsPerPod), resource.DecimalSI),
                        },
                        Limits: corev1.ResourceList{
                            "nvidia.com/gpu": *resource.NewQuantity(
                                int64(job.Spec.GPUsPerPod), resource.DecimalSI),
                        },
                    },
                },
            },
            RestartPolicy: corev1.RestartPolicyNever,
        },
    }
    
    return pod
}

// SetupWithManager 设置 Controller 与 Manager
func (r *GPUJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&schedulingv1.GPUJob{}).
        Owns(&corev1.Pod{}).
        Complete(r)
}
4.2 关键设计要点
设计点	说明
幂等性	Reconcile 函数可能因任何事件反复触发，必须保证多次执行结果一致
SchedulerName	每个 Pod 必须设置 schedulerName: "gpusched-scheduler"，才会被自定义调度器处理
OwnerReference	Pod 通过 OwnerReference 指向父 GPUJob，便于垃圾回收和关联查询
GPU 资源请求	使用 nvidia.com/gpu 标准资源名，由 NVIDIA Device Plugin 提供
4.3 RBAC 权限配置
在 config/rbac/role.yaml 中需要包含以下权限：

yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: manager-role
rules:
- apiGroups: ["scheduling.gpusched.io"]
  resources: ["gpujobs"]
  verbs: ["create", "delete", "get", "list", "patch", "update", "watch"]
- apiGroups: ["scheduling.gpusched.io"]
  resources: ["gpujobs/status"]
  verbs: ["get", "patch", "update"]
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["create", "delete", "get", "list", "patch", "update", "watch"]
4.4 部署 Controller
bash
# 生成并应用 RBAC 配置
make manifests
kubectl apply -f config/rbac/role.yaml

# 构建并推送镜像（或使用默认的 controller:latest）
make docker-build docker-push IMG=your-registry/gpusched-controller:latest

# 部署到集群
make deploy IMG=your-registry/gpusched-controller:latest

# 查看 Controller 运行状态
kubectl get pods -n gpusched-system
kubectl logs -f deployment/gpusched-controller-manager -n gpusched-system
第二阶段验收标准：

□ Controller 成功部署并运行
□ 提交 GPUJob 后，Controller 能自动创建对应数量的 Pod
□ 创建的 Pod 的 schedulerName 为 gpusched-scheduler
□ Pod 正确设置了 OwnerReference
□ GPUJob 的 Status 能正确反映 Pod 状态
五、第三阶段：Scheduler Plugin 实现（Week 5-6）
5.1 项目结构调整
需要为调度器创建独立的入口和插件目录：

bash
# 创建调度器入口
mkdir -p cmd/scheduler
touch cmd/scheduler/main.go

# 创建插件目录
mkdir -p pkg/scheduler/plugins/gpufit
touch pkg/scheduler/plugins/gpufit/gpufit.go
最终的目录结构：

text
gpusched/
├── api/v1/                    # CRD 定义
├── internal/controller/        # Controller 实现
├── cmd/
│   ├── main.go                # Controller 入口
│   └── scheduler/
│       └── main.go            # Scheduler 入口（新增）
├── pkg/
│   └── scheduler/
│       └── plugins/
│           └── gpufit/
│               └── gpufit.go  # Filter + Score 插件（新增）
└── config/
    ├── crd/
    ├── rbac/
    └── scheduler/
        └── scheduler-config.yaml  # 调度器配置（新增）
5.2 Scheduler 入口（cmd/scheduler/main.go）
go
package main

import (
    "os"
    
    "k8s.io/component-base/logs"
    _ "k8s.io/component-base/logs/json/register"
    "sigs.k8s.io/scheduler-plugins/pkg/coscheduling"
    
    "github.com/yourname/gpusched/pkg/scheduler/plugins/gpufit"
)

func main() {
    // 初始化命令行参数
    command := coscheduling.NewSchedulerCommand()
    
    // 注册自定义插件
    command.PluginRegistry = append(command.PluginRegistry, 
        gpufit.NewPluginFactory(),
    )
    
    logs.InitLogs()
    defer logs.FlushLogs()
    
    if err := command.Execute(); err != nil {
        os.Exit(1)
    }
}
5.3 Filter 插件实现
编辑 pkg/scheduler/plugins/gpufit/gpufit.go：

go
package gpufit

import (
    "context"
    "fmt"
    
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/kubernetes/pkg/scheduler/framework"
)

const (
    // PluginName 是插件的名称
    PluginName = "GPUFit"
)

// GPUFit 实现了 Filter 和 Score 插件
type GPUFit struct {
    handle framework.Handle
}

var _ framework.FilterPlugin = &GPUFit{}
var _ framework.ScorePlugin = &GPUFit{}

// Name 返回插件名称
func (pl *GPUFit) Name() string {
    return PluginName
}

// Filter 检查节点是否有足够的 GPU 资源[reference:19]
func (pl *GPUFit) Filter(ctx context.Context, state *framework.CycleState, 
    pod *corev1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
    
    // 1. 从 Pod 中读取请求的 GPU 数量
    requestedGPU := pod.Spec.Containers[0].Resources.Requests["nvidia.com/gpu"]
    if requestedGPU.IsZero() {
        // Pod 不请求 GPU，放行
        return framework.NewStatus(framework.Success, "")
    }
    
    // 2. 获取节点可用 GPU 数量
    // 从 nodeInfo.Allocatable 获取总量，减去已请求量
    allocatableGPU := nodeInfo.Allocatable["nvidia.com/gpu"]
    requestedOnNode := nodeInfo.Requested["nvidia.com/gpu"]
    available := allocatableGPU.DeepCopy()
    available.Sub(requestedOnNode)
    
    // 3. 如果不够，拒绝该节点
    if available.Cmp(requestedGPU) < 0 {
        return framework.NewStatus(framework.Unschedulable, 
            fmt.Sprintf("insufficient GPU: available %d, requested %d", 
                available.Value(), requestedGPU.Value()))
    }
    
    return framework.NewStatus(framework.Success, "")
}

// Score 对通过 Filter 的节点打分（Binpack 策略）[reference:20]
func (pl *GPUFit) Score(ctx context.Context, state *framework.CycleState, 
    pod *corev1.Pod, nodeName string) (int64, *framework.Status) {
    
    // 获取节点信息
    nodeInfo, err := pl.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
    if err != nil {
        return 0, framework.NewStatus(framework.Error, 
            fmt.Sprintf("failed to get node info: %v", err))
    }
    
    // 计算 GPU 利用率：已用 / 总量
    total := nodeInfo.Allocatable["nvidia.com/gpu"]
    if total.IsZero() {
        // 没有 GPU 的节点得最低分
        return 0, framework.NewStatus(framework.Success, "")
    }
    
    used := nodeInfo.Requested["nvidia.com/gpu"]
    utilization := float64(used.Value()) / float64(total.Value())
    
    // Binpack: 利用率越高，分数越高
    // 把 Pod 塞到已经忙的节点上，减少碎片
    score := int64(utilization * 100)
    
    return score, framework.NewStatus(framework.Success, "")
}

// ScoreExtensions 返回 Score 扩展（本插件不需要 Normalize）
func (pl *GPUFit) ScoreExtensions() framework.ScoreExtensions {
    return nil
}

// New 创建插件实例
func New(obj runtime.Object, handle framework.Handle) (framework.Plugin, error) {
    return &GPUFit{handle: handle}, nil
}

// NewPluginFactory 创建插件工厂
func NewPluginFactory() framework.PluginFactory {
    return framework.PluginFactory{
        Name: PluginName,
        Factory: func(configuration runtime.Object, f framework.Handle) (framework.Plugin, error) {
            return New(configuration, f)
        },
    }
}
5.4 调度器配置文件
创建 config/scheduler/scheduler-config.yaml：

yaml
apiVersion: kubescheduler.config.k8s.io/v1
kind: KubeSchedulerConfiguration
clientConnection:
  kubeconfig: /etc/kubernetes/scheduler.conf
profiles:
- schedulerName: gpusched-scheduler
  plugins:
    filter:
      enabled:
      - name: GPUFit
    score:
      enabled:
      - name: GPUFit
        weight: 10
5.5 部署 Scheduler
创建 deploy/scheduler.yaml：

yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gpusched-scheduler
  namespace: gpusched-system
  labels:
    component: scheduler
spec:
  replicas: 1
  selector:
    matchLabels:
      component: scheduler
  template:
    metadata:
      labels:
        component: scheduler
    spec:
      serviceAccountName: gpusched-scheduler
      containers:
      - name: scheduler
        image: your-registry/gpusched-scheduler:latest
        imagePullPolicy: IfNotPresent
        command:
        - /usr/local/bin/kube-scheduler
        - --config=/etc/kubernetes/scheduler-config.yaml
        - --v=3
        volumeMounts:
        - name: config
          mountPath: /etc/kubernetes
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 200m
            memory: 256Mi
      volumes:
      - name: config
        configMap:
          name: gpusched-scheduler-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: gpusched-scheduler-config
  namespace: gpusched-system
data:
  scheduler-config.yaml: |
    apiVersion: kubescheduler.config.k8s.io/v1
    kind: KubeSchedulerConfiguration
    profiles:
    - schedulerName: gpusched-scheduler
      plugins:
        filter:
          enabled:
          - name: GPUFit
        score:
          enabled:
          - name: GPUFit
            weight: 10
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: gpusched-scheduler
  namespace: gpusched-system
第三阶段验收标准：

□ Scheduler 成功部署并运行
□ 提交 GPUJob 后，Pod 的 schedulerName 为 gpusched-scheduler
□ Pod 能被调度到有足够 GPU 的节点上
□ 当 GPU 不足时，Pod 保持 Pending 状态
□ Filter 插件能正确过滤 GPU 不足的节点
□ Score 插件能按 Binpack 策略正确打分
六、第四阶段：集成测试与部署（Week 7-8）
6.1 本地集成测试
bash
# 1. 确保 Kind 集群运行
kind get clusters

# 2. 部署 Controller
make deploy IMG=your-registry/gpusched-controller:latest

# 3. 部署 Scheduler
kubectl apply -f deploy/scheduler.yaml

# 4. 验证两个组件都正常运行
kubectl get pods -n gpusched-system

# 5. 提交测试 GPUJob
kubectl apply -f config/samples/scheduling_v1_gpujob.yaml

# 6. 观察调度过程
kubectl get gpujobs -w
kubectl get pods -o wide
kubectl logs -f deployment/gpusched-scheduler -n gpusched-system
6.2 端到端测试场景
测试场景	操作	预期结果
正常调度	提交 replicas=2, gpusPerPod=1 的 GPUJob	2 个 Pod 均 Running
资源不足	提交超过集群 GPU 总量的作业	所有 Pod Pending，等待资源
跨节点调度	一个节点 GPU 用满后提交新作业	新 Pod 自动调度到另一个有 GPU 的节点
作业完成	等待 GPUJob 所有 Pod 完成	Status.Phase = Succeeded
缩容	修改 spec.replicas 从 2 改为 1	多余的 Pod 被自动删除
6.3 构建统一部署脚本
创建 deploy/deploy.sh：

bash
#!/bin/bash
set -e

REGISTRY=${1:-"localhost:5000"}
TAG=${2:-"latest"}

echo "Building and pushing images..."
make docker-build docker-push IMG=${REGISTRY}/gpusched-controller:${TAG}

echo "Deploying Controller..."
make deploy IMG=${REGISTRY}/gpusched-controller:${TAG}

echo "Deploying Scheduler..."
kubectl apply -f deploy/scheduler.yaml

echo "Waiting for deployments to be ready..."
kubectl wait --for=condition=available --timeout=60s \
    deployment/gpusched-controller-manager -n gpusched-system
kubectl wait --for=condition=available --timeout=60s \
    deployment/gpusched-scheduler -n gpusched-system

echo "GpuSched deployed successfully!"
kubectl get pods -n gpusched-system
6.4 清理命令
bash
# 删除所有 GPUJob
kubectl delete gpujobs --all

# 卸载 Controller
make undeploy

# 删除 Scheduler
kubectl delete -f deploy/scheduler.yaml

# 删除 CRD
make uninstall

# 删除 Kind 集群
kind delete cluster --name gpu-cluster
七、关键技术决策汇总
决策点	选择	理由
API 构建	Kubebuilder + CRD	SIG 官方框架，最佳实践
Controller	controller-runtime Reconciler	标准控制器模式，调谐循环必须幂等
调度扩展	Scheduler Framework Plugin	官方推荐，无网络开销
调度策略	Filter + Score (Binpack)	MVP 最小可行，后续可扩展
GPU 资源标识	nvidia.com/gpu	NVIDIA Device Plugin 标准
本地集群	Kind	轻量级，秒级创建，适合开发
调度器名称	gpusched-scheduler	与默认调度器共存
八、学习路径与里程碑
8.1 周计划总览
阶段	周次	核心任务	产出
环境准备	Week 0	安装工具、创建 Kind 集群	可用的本地 K8s 环境
第一阶段	Week 1-2	项目初始化 + CRD 定义	kubectl get gpujobs 可用
第二阶段	Week 3-4	Controller 实现	GPUJob → Pod 自动创建
第三阶段	Week 5-6	Scheduler Plugin 实现	GPU 感知调度生效
第四阶段	Week 7-8	集成测试 + 部署	完整端到端流程验证
8.2 各阶段里程碑检查清单
第一阶段（Week 1-2）验收：

□ kubebuilder init --domain gpusched.io 成功执行
□ kubebuilder create api 成功生成 CRD 骨架
□ make manifests 生成 CRD YAML
□ make install 成功安装 CRD 到集群
□ kubectl get crd | grep gpujobs 能看到 CRD
第二阶段（Week 3-4）验收：

□ Controller 的 Reconcile 逻辑编写完成
□ RBAC 权限配置正确
□ make deploy 成功部署 Controller
□ 提交 GPUJob 后 Pod 自动创建
□ Pod 的 schedulerName 为 gpusched-scheduler
□ GPUJob Status 正确更新
第三阶段（Week 5-6）验收：

□ Filter 插件实现完成
□ Score 插件实现完成（Binpack 策略）
□ Scheduler 配置文件正确
□ Scheduler 成功部署并运行
□ Pod 能被调度到有 GPU 的节点
□ GPU 不足时 Pod 保持 Pending
第四阶段（Week 7-8）验收：

□ 所有测试场景通过
□ 部署脚本可用
□ 文档完整（README + 架构图）
□ 项目代码已提交 GitHub
8.3 学习资料速查
主题	推荐资源
Kubebuilder	The Kubebuilder Book
Scheduler Framework	Kubernetes Scheduling Framework 文档
Device Plugin	NVIDIA Device Plugin 文档
GPU 拓扑	nvidia-smi topo 文档
Controller Runtime	controller-runtime 文档
九、GpuSched 项目的长期演进路线图
9.1 当前版本（MVP，本实施计划）
✅ GPUJob CRD 定义与生命周期管理
✅ Controller 调谐循环实现 Pod 创建与状态同步
✅ Scheduler Filter 插件实现 GPU 资源过滤
✅ Scheduler Score 插件实现 Binpack 打分

9.2 第二阶段（Gang Scheduling）
□ Gang Scheduling：一个作业的所有 Pod 要么全部调度成功，要么全部失败
□ 资源预留（Reserve）与回滚机制
□ 超时控制与失败重试
□ 学习：Raft 共识算法、etcd 并发控制
9.3 第三阶段（拓扑感知调度）
□ 集成 GPU Feature Discovery 采集拓扑信息
□ 解析 nvidia-smi topo -m 输出
□ 拓扑感知的 Score 插件实现
□ 学习：NVLink 物理拓扑、PCIe 带宽
9.4 第四阶段（eBPF 可观测性）
□ 使用 bpftrace 追踪 GPU 调度事件
□ 开发 eBPF 程序追踪 Pod 创建延迟
□ 暴露 eBPF 指标到 Prometheus
□ 学习：eBPF 内核追踪、Linux 内核调度
9.5 第五阶段（生产级优化）
□ 大规模压测（kube-burner）
□ pprof 性能剖析与 Go Runtime 调优
□ Informer 缓存优化
□ 镜像拉取加速（Dragonfly/Nydus）
9.6 长期愿景
□ 成为 Kubernetes SIG-Scheduling 贡献者
□ 开源 GpuSched 项目
□ 撰写系列技术博客（设计、踩坑、优化）
□ 探索 AI 辅助调度（故障预测、碎片预测）
附录 A：常用命令速查
bash
# ---- 开发 ----
make manifests              # 生成 CRD 清单
make install                # 安装 CRD 到集群
make uninstall              # 卸载 CRD
make run                    # 本地运行 Controller
make docker-build           # 构建 Docker 镜像
make deploy                 # 部署到集群
make undeploy               # 卸载

# ---- 集群 ----
kind create cluster --name gpu-cluster
kind delete cluster --name gpu-cluster
kubectl cluster-info --context kind-gpu-cluster
kubectl get nodes
kubectl get pods -A

# ---- GPUJob ----
kubectl get gpujobs
kubectl get gpujobs -o yaml
kubectl describe gpujob <name>
kubectl delete gpujob <name>
kubectl apply -f config/samples/scheduling_v1_gpujob.yaml

# ---- 调试 ----
kubectl logs -f deployment/gpusched-controller-manager -n gpusched-system
kubectl logs -f deployment/gpusched-scheduler -n gpusched-system
kubectl get pods -o wide
kubectl describe pod <pod-name>
附录 B：常见问题排查
B.1 CRD 安装失败
bash
# 检查 CRD 是否已存在
kubectl get crd | grep gpujobs

# 如果存在冲突，先删除再安装
kubectl delete crd gpujobs.scheduling.gpusched.io
make install
B.2 Controller 无法创建 Pod
bash
# 检查 RBAC 权限
kubectl auth can-i create pods --as=system:serviceaccount:gpusched-system:default

# 查看 Controller 日志
kubectl logs -f deployment/gpusched-controller-manager -n gpusched-system
B.3 Pod 一直 Pending
bash
# 查看 Pod 事件
kubectl describe pod <pod-name>

# 检查 Scheduler 是否运行
kubectl get pods -n gpusched-system | grep scheduler

# 检查 Scheduler 日志
kubectl logs -f deployment/gpusched-scheduler -n gpusched-system

# 确认 Pod 的 schedulerName 正确
kubectl get pod <pod-name> -o jsonpath='{.spec.schedulerName}'
B.4 GPU 资源无法识别
bash
# 检查 Device Plugin 是否运行
kubectl get pods -n kube-system | grep nvidia

# 检查节点 GPU 容量
kubectl describe node | grep nvidia.com/gpu

# 检查 Device Plugin 日志
kubectl logs -f daemonset/nvidia-device-plugin -n kube-system
项目 Git 仓库初始化：

bash
git init
git add .
git commit -m "feat: initialize GpuSched project with GPUJob CRD"
git remote add origin https://github.com/yourname/gpusched
git push -u origin main
祝你 GpuSched 开发顺利！每一行代码都是通往系统专家之路的坚实脚印。