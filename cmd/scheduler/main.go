package main

import (
	"os"

	"k8s.io/component-base/logs"
	"sigs.k8s.io/scheduler-plugins/pkg/coscheduling"
)

func main() {
	command := coscheduling.NewSchedulerCommand()

	command.PluginRegistry = append(command.PluginRegistry, gpufit.NewPluginFactory())

	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
