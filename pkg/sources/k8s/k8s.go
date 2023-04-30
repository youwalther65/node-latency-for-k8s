/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package k8s is a latency timing source for K8s API events
package k8s

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/awslabs/node-latency-for-k8s/pkg/sources"

	"k8s.io/client-go/kubernetes"
)

var (
	Name = "K8s"
)

// Source is the K8s API http source
type Source struct {
	clientset    *kubernetes.Clientset
	nodeName     string
	podNamespace string
}

// New instantiates a new instance of the K8s API source
func New(clientset *kubernetes.Clientset, nodeName string, podNamespace string) *Source {
	return &Source{
		clientset:    clientset,
		nodeName:     nodeName,
		podNamespace: podNamespace,
	}
}

// ClearCache is a noop for the K8s API Source since it is an http source, not a log file
func (s Source) ClearCache() {}

// String is a human readable string of the source
func (s Source) String() string {
	return Name
}

// Name is the name of the source
func (s Source) Name() string {
	return Name
}

// FindPodCreationTime retrieves the Pod creation time
func (s *Source) FindPodCreationTime() sources.FindFunc {
	return func(_ sources.Source, _ []byte) ([]string, error) {
		pod, err := s.FindPod()
		if err != nil {
			return nil, err
		}
		return []string{fmt.Sprint(pod.CreationTimestamp.Unix())}, nil
	}
}

// FindPodReadyTime retrieves the Pod Ready time
func (s *Source) FindPodReadyTime() sources.FindFunc {
	return func(_ sources.Source, _ []byte) ([]string, error) {
		pod, err := s.FindPod()
		if err != nil {
			return nil, err
		}
		podReady, ok := lo.Find(pod.Status.Conditions, func(condition corev1.PodCondition) bool {
			return condition.Type == corev1.PodReady
		})
		if !ok {
			return nil, fmt.Errorf("unable to find pod ready condition")
		}
		return []string{fmt.Sprint(podReady.LastTransitionTime.Unix())}, nil
	}
}

// FindPodScheduledTime retrieves the Pod Scheduled time
func (s *Source) FindPodScheduledTime() sources.FindFunc {
	return func(_ sources.Source, _ []byte) ([]string, error) {
		pod, err := s.FindPod()
		if err != nil {
			return nil, err
		}
		podScheduled, ok := lo.Find(pod.Status.Conditions, func(condition corev1.PodCondition) bool {
			return condition.Type == corev1.PodScheduled
		})
		if !ok {
			return nil, fmt.Errorf("unable to find pod scheduled condition")
		}
		return []string{fmt.Sprint(podScheduled.LastTransitionTime.Unix())}, nil
	}
}

// FindPod is a helper that retrieves a pod based on the source nodeName
func (s *Source) FindPod() (*corev1.Pod, error) {
	ctx := context.Background()
	pods, err := s.clientset.CoreV1().Pods(s.podNamespace).List(ctx, v1.ListOptions{FieldSelector: fmt.Sprintf("spec.nodeName=%s", s.nodeName)})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("unable to find pods")
	}
	return &pods.Items[0], nil
}

// FindNodeReadyTime retrieves the Node Ready time
func (s *Source) FindNodeReadyTime() sources.FindFunc {
	return func(_ sources.Source, _ []byte) ([]string, error) {
		node, err := s.FindNode()
		if err != nil {
			return nil, err
		}
		nodeMatches := lo.FilterMap(node.Status.Conditions, func(condition corev1.NodeCondition, _ int) (string, bool) {
			return fmt.Sprint(condition.LastTransitionTime.Unix()), condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue
		})
		if len(nodeMatches) == 0 {
			return nil, fmt.Errorf("unable to find nodes with Ready condition")
		}
		return nodeMatches, nil
	}
}

// FindNodeRegisterTime retrieves the Node Creation time
func (s *Source) FindNodeRegisterTime() sources.FindFunc {
	return func(_ sources.Source, _ []byte) ([]string, error) {
		node, err := s.FindNode()
		if err != nil {
			return nil, err
		}
		return []string{fmt.Sprint(node.CreationTimestamp.Unix())}, nil
	}
}

// FindNode is a helper that retrieves a node based on the source nodeName and then
func (s *Source) FindNode() (*corev1.Node, error) {
	ctx := context.Background()
	node, err := s.clientset.CoreV1().Nodes().Get(ctx, s.nodeName, v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return node, nil
}

// ParseTimeFor parses a Unix timestamp (time in seconds since the Epoch)
func (s *Source) ParseTimeFor(unixSec []byte) (time.Time, error) {
	unixTS, err := strconv.ParseInt(string(unixSec), 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("unable to parse time for K8s event")
	}
	return time.Unix(unixTS, 0), nil
}

// Find will use the Event's FindFunc and CommentFunc to search the source and return the result
func (s *Source) Find(event *sources.Event) ([]sources.FindResult, error) {
	k8sEvents, err := event.FindFn(s, nil)
	if err != nil {
		return nil, err
	}
	var results []sources.FindResult
	for _, k8sEvent := range k8sEvents {
		comment := ""
		if event.CommentFn != nil {
			comment = event.CommentFn(k8sEvent)
		}
		eventTime, err := s.ParseTimeFor([]byte(k8sEvent))
		results = append(results, sources.FindResult{
			Line:      k8sEvent,
			Timestamp: eventTime,
			Comment:   comment,
			Err:       err,
		})
	}
	return sources.SelectMatches(results, event.MatchSelector), nil
}
