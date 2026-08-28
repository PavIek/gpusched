package main

import (
	"fmt"
	"os"

	"github.com/PavIek/gpusched/pkg/scheduler/plugins/gpufit"
	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"
)

func main() {
	command := app.NewSchedulerCommand(app.WithPlugin("GPUFit-plugin", gpufit.NewPluginFactory()))

	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
