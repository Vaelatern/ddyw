package customization

import (
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

var Activities []interface{}
var ActivityOptions []activity.RegisterOptions
var Workflows []interface{}
var WorkflowOptions []workflow.RegisterOptions

func RegWorkflow(in interface{}) {
	RegWorkflowOpt(in, workflow.RegisterOptions{})
}

func RegWorkflowOpt(in interface{}, opts workflow.RegisterOptions) {
	Workflows = append(Workflows, in)
	WorkflowOptions = append(WorkflowOptions, opts)
}

func RegActivity(in interface{}) {
	RegActivityOpt(in, activity.RegisterOptions{})
}

func RegActivityOpt(in interface{}, opts activity.RegisterOptions) {
	Activities = append(Activities, in)
	ActivityOptions = append(ActivityOptions, opts)
}
