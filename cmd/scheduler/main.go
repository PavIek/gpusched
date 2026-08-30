package main

import (
	"os"

	"github.com/PavIek/gpusched/pkg/scheduler/plugins/gpufit"
	"k8s.io/component-base/cli"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"
)

func main() {
	command := app.NewSchedulerCommand(
		app.WithPlugin(gpufit.PluginName, gpufit.New),
	)

	code := cli.Run(command)

	os.Exit(code)
}
