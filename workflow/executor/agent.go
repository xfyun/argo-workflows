package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	workflow "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned/typed/workflow/v1alpha1"
	executorplugins "github.com/argoproj/argo-workflows/v3/pkg/plugins/executor"
	"github.com/argoproj/argo-workflows/v3/util"
	"github.com/argoproj/argo-workflows/v3/util/env"
	"github.com/argoproj/argo-workflows/v3/util/errors"
	"github.com/argoproj/argo-workflows/v3/workflow/common"
	argohttp "github.com/argoproj/argo-workflows/v3/workflow/executor/http"
)

type AgentExecutor struct {
	WorkflowName      string
	ClientSet         kubernetes.Interface
	WorkflowInterface workflow.Interface
	RESTClient        rest.Interface
	Namespace         string
	consideredTasks   map[string]bool
	plugins           []executorplugins.TemplateExecutor
}

type templateExecutor = func(ctx context.Context, tmpl wfv1.Template, reply *wfv1.NodeResult) (time.Duration, error)

func NewAgentExecutor(clientSet kubernetes.Interface, restClient rest.Interface, config *rest.Config, namespace, workflowName string, plugins []executorplugins.TemplateExecutor) *AgentExecutor {
	return &AgentExecutor{
		ClientSet:         clientSet,
		RESTClient:        restClient,
		Namespace:         namespace,
		WorkflowName:      workflowName,
		WorkflowInterface: workflow.NewForConfigOrDie(config),
		consideredTasks:   make(map[string]bool),
		plugins:           plugins,
	}
}

type task struct {
	NodeId   string
	Template wfv1.Template
}

type response struct {
	NodeId string
	Result *wfv1.NodeResult
}

const EnvAgentTaskWorkers = "ARGO_AGENT_TASK_WORKERS"

func (ae *AgentExecutor) Agent(ctx context.Context) error {
	defer runtimeutil.HandleCrash(runtimeutil.PanicHandlers...)
	defer log.Info("stopped agent")

	log := log.WithField("workflow", ae.WorkflowName)

	taskWorkers := env.LookupEnvIntOr(EnvAgentTaskWorkers, 16)
	log.WithField("task_workers", taskWorkers).Info("Starting Agent s15")

	taskQueue := make(chan task)
	responseQueue := make(chan response)
	taskSetInterface := ae.WorkflowInterface.ArgoprojV1alpha1().WorkflowTaskSets(ae.Namespace)

	go ae.patchWorker(ctx, taskSetInterface, responseQueue)
	for i := 0; i < taskWorkers; i++ {
		go ae.taskWorker(ctx, taskQueue, responseQueue)
	}

	for {
		wfWatch, err := taskSetInterface.Watch(ctx, metav1.ListOptions{FieldSelector: "metadata.name=" + ae.WorkflowName})
		if err != nil {
			return err
		}

		for event := range wfWatch.ResultChan() {
			log.WithField("event_type", event.Type).Info("TaskSet Event")

			if event.Type == watch.Deleted {
				// We're done if the task set is deleted
				return nil
			}

			taskSet, ok := event.Object.(*wfv1.WorkflowTaskSet)
			if !ok {
				return apierr.FromObject(event.Object)
			}
			if IsWorkflowCompleted(taskSet) {
				return nil
			}

			for nodeID, tmpl := range taskSet.Spec.Tasks {
				taskQueue <- task{NodeId: nodeID, Template: tmpl}
			}
		}
	}
}

func (ae *AgentExecutor) taskWorker(ctx context.Context, taskQueue chan task, responseQueue chan response) {
	for task := range taskQueue {
		nodeID, tmpl := task.NodeId, task.Template
		log := log.WithFields(log.Fields{"nodeID": nodeID})
		log.Info("Attempting task")

		// Do not work on tasks that have already been considered once, to prevent calling an endpoint more
		// than once unintentionally.
		if _, ok := ae.consideredTasks[nodeID]; ok {
			log.Info("Task is already considered")
			continue
		}

		ae.consideredTasks[nodeID] = true

		log.Info("Processing task")
		result, requeue, err := ae.processTask(ctx, tmpl)
		if err != nil {
			log.WithError(err).Error("Error in agent task")
			return
		}

		log.
			WithField("phase", result.Phase).
			WithField("requeue", requeue).
			Info("Sending result")

		if result.Phase != "" {
			responseQueue <- response{NodeId: nodeID, Result: result}
		}
		if requeue > 0 {
			time.AfterFunc(requeue, func() {
				taskQueue <- task
			})
		}
	}
}

func (ae *AgentExecutor) patchWorker(ctx context.Context, taskSetInterface v1alpha1.WorkflowTaskSetInterface, responseQueue chan response) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	nodeResults := map[string]wfv1.NodeResult{}
	for {
		select {
		case res := <-responseQueue:
			nodeResults[res.NodeId] = *res.Result
		case <-ticker.C:
			if len(nodeResults) == 0 {
				continue
			}

			patch, err := json.Marshal(map[string]interface{}{"status": wfv1.WorkflowTaskSetStatus{Nodes: nodeResults}})
			if err != nil {
				log.WithError(err).Error("Generating Patch Failed")
				continue
			}

			log.WithFields(log.Fields{"workflow": ae.WorkflowName}).Info("Processing Patch")

			obj, err := taskSetInterface.Patch(ctx, ae.WorkflowName, types.MergePatchType, patch, metav1.PatchOptions{})
			if err != nil {
				isTransientErr := errors.IsTransientErr(err)
				log.WithError(err).WithFields(log.Fields{"taskset": obj, "is_transient_error": isTransientErr}).Errorf("TaskSet Patch Failed")

				// If this is not a transient error, then it's likely that the contents of the patch have caused the error.
				// To avoid a deadlock with the workflow overall, or an infinite loop, fail and propagate the error messages
				// to the nodes.
				// If this is a transient error, then simply do nothing and another patch will be retried in the next tick.
				if !isTransientErr {
					for node := range nodeResults {
						nodeResults[node] = wfv1.NodeResult{
							Phase:   wfv1.NodeError,
							Message: fmt.Sprintf("HTTP request completed successfully but an error occurred when patching its result: %s", err),
						}
					}
				}
				continue
			}

			// Patch was successful, clear nodeResults for next iteration
			nodeResults = map[string]wfv1.NodeResult{}

			log.Info("Patched TaskSet")
		}
	}
}

func (ae *AgentExecutor) processTask(ctx context.Context, tmpl wfv1.Template) (*wfv1.NodeResult, time.Duration, error) {
	var executeTemplate templateExecutor
	switch {
	case tmpl.HTTP != nil:
		executeTemplate = ae.executeHTTPTemplate
	case tmpl.Plugin != nil:
		executeTemplate = ae.executePluginTemplate
	default:
		return nil, 0, fmt.Errorf("plugins cannot execute: unknown task type: %v", tmpl.GetType())
	}
	result := &wfv1.NodeResult{}
	requeue, err := executeTemplate(ctx, tmpl, result)
	if err != nil {
		result.Phase = wfv1.NodeFailed
		result.Message = err.Error()
	}

	return result, requeue, nil
}

func (ae *AgentExecutor) executeHTTPTemplate(ctx context.Context, tmpl wfv1.Template, reply *wfv1.NodeResult) (time.Duration, error) {
	httpTemplate := tmpl.HTTP
	request, err := http.NewRequest(httpTemplate.Method, httpTemplate.URL, bytes.NewBufferString(httpTemplate.Body))
	if err != nil {
		return 0, err
	}
	request = request.WithContext(ctx)

	for _, header := range httpTemplate.Headers {
		value := header.Value
		if header.ValueFrom != nil && header.ValueFrom.SecretKeyRef != nil {
			secret, err := util.GetSecrets(ctx, ae.ClientSet, ae.Namespace, header.ValueFrom.SecretKeyRef.Name, header.ValueFrom.SecretKeyRef.Key)
			if err != nil {
				return 0, err
			}
			value = string(secret)
		}
		request.Header.Add(header.Name, value)
	}
	response, err := argohttp.SendHttpRequest(request, httpTemplate.TimeoutSeconds)
	if err != nil {
		return 0, err
	}
	reply.Phase = wfv1.NodeSucceeded
	reply.Outputs = &wfv1.Outputs{
		Parameters: []wfv1.Parameter{{Name: "result", Value: wfv1.AnyStringPtr(response)}},
	}
	return 0, nil
}

func (ae *AgentExecutor) executePluginTemplate(_ context.Context, tmpl wfv1.Template, result *wfv1.NodeResult) (time.Duration, error) {
	args := executorplugins.ExecuteTemplateArgs{
		Workflow: &executorplugins.Workflow{
			ObjectMeta: executorplugins.ObjectMeta{Name: ae.WorkflowName},
		},
		Template: &tmpl,
	}
	reply := &executorplugins.ExecuteTemplateReply{}
	for _, plug := range ae.plugins {
		if err := plug.ExecuteTemplate(args, reply); err != nil {
			return 0, err
		} else if reply.Node != nil {
			*result = *reply.Node
			return reply.GetRequeue(), nil
		}
	}
	return 0, fmt.Errorf("not plugin executed the template")
}

func IsWorkflowCompleted(wts *wfv1.WorkflowTaskSet) bool {
	return wts.Labels[common.LabelKeyCompleted] == "true"
}
