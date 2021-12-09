/*
Copyright 2019 The Kubernetes Authors.

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

package execservice

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog"

	"k8s.io/perf-tests/clusterloader2/pkg/framework"
	"k8s.io/perf-tests/clusterloader2/pkg/framework/client"
	measurementutil "k8s.io/perf-tests/clusterloader2/pkg/measurement/util"
)

const (
	execDeploymentNamespace = "cluster-loader"
	execDeploymentName      = "exec-pod"
	execDeploymentManifest  = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.Name}}
  namespace: {{.Namespace}}
spec:
  replicas: {{.Replicas}}
  selector:
    matchLabels:
       feature: exec
  template:
    metadata:
       labels:
         feature: exec
    spec:
      containers:
      - name: agnhost
        image: k8s.gcr.io/e2e-test-images/agnhost:2.32
`
	execPodReplicas = 3
	execPodSelector = "feature = exec"

	execPodCheckInterval = 10 * time.Second
	execPodCheckTimeout  = 2 * time.Minute

	execServiceName = "Exec service"
)

var (
	lock     sync.Mutex
	podStore *measurementutil.PodStore
)

// SetUpExecService creates exec pod.
func SetUpExecService(f *framework.Framework) error {
	var err error
	lock.Lock()
	defer lock.Unlock()
	if podStore != nil {
		klog.V(3).Infof("%s: service already running!", execServiceName)
	}
	klog.V(2).Infof("%v: setting up service!", execServiceName)
	mapping := make(map[string]interface{})
	mapping["Name"] = execDeploymentName
	mapping["Namespace"] = execDeploymentNamespace
	mapping["Replicas"] = execPodReplicas
	if err = client.CreateNamespace(f.GetClientSets().GetClient(), execDeploymentNamespace); err != nil {
		return fmt.Errorf("namespace %s creation error: %v", execDeploymentNamespace, err)
	}
	tmpFile, err := ioutil.TempFile(os.TempDir(), "clusterloader2-exec-deployment-")
	if err != nil {
		return fmt.Errorf("failed to create tempfile for exec deployment manifest: %v", err)
	}

	if _, err = tmpFile.Write([]byte(execDeploymentManifest)); err != nil {
		return fmt.Errorf("failed to write to tempfile for exec deployment manifest: %v", err)
	}

	defer os.Remove(tmpFile.Name())

	if err = f.ApplyTemplatedManifests(
		tmpFile.Name(),
		mapping,
		client.Retry(apierrs.IsNotFound)); err != nil {
		return fmt.Errorf("pod %s creation error: %v", execDeploymentName, err)
	}

	stopCh := make(chan struct{})
	time.AfterFunc(execPodCheckTimeout, func() {
		close(stopCh)
	})
	selector := &measurementutil.ObjectSelector{
		Namespace:     execDeploymentNamespace,
		LabelSelector: execPodSelector,
		FieldSelector: "",
	}
	options := &measurementutil.WaitForPodOptions{
		Selector:            selector,
		DesiredPodCount:     func() int { return execPodReplicas },
		CallerName:          execServiceName,
		WaitForPodsInterval: execPodCheckInterval,
	}
	if err = measurementutil.WaitForPods(f.GetClientSets().GetClient(), stopCh, options); err != nil {
		return err
	}
	podStore, err = measurementutil.NewPodStore(f.GetClientSets().GetClient(), selector)
	if err != nil {
		return fmt.Errorf("pod store creation error: %v", err)
	}
	klog.V(2).Infof("%v: service set up successfully!", execServiceName)
	return nil
}

// TearDownExecService deletes exec pod.
func TearDownExecService(f *framework.Framework) error {
	lock.Lock()
	defer lock.Unlock()
	klog.V(2).Infof("%v: tearing down service", execServiceName)
	if podStore != nil {
		podStore.Stop()
		podStore = nil
	}
	if err := client.DeleteNamespace(f.GetClientSets().GetClient(), execDeploymentNamespace); err != nil {
		return fmt.Errorf("deleting %s namespace error: %v", execDeploymentNamespace, err)
	}
	if err := client.WaitForDeleteNamespace(f.GetClientSets().GetClient(), execDeploymentNamespace); err != nil {
		return err
	}
	return nil
}

// RunCommand executes given command on a pod in cluster.
func RunCommand(pod *corev1.Pod, cmd string) (string, error) {
	var stdout, stderr bytes.Buffer
	c := exec.Command("kubectl", "exec", fmt.Sprintf("--namespace=%v", pod.Namespace), pod.Name, "--", "/bin/sh", "-x", "-c", cmd)
	c.Stdout, c.Stderr = &stdout, &stderr
	if err := c.Run(); err != nil {
		return stderr.String(), err
	}
	return stdout.String(), nil
}

// GetPod get a exec service pod in a cluster.
func GetPod() (*corev1.Pod, error) {
	lock.Lock()
	defer lock.Unlock()
	if podStore == nil {
		return nil, fmt.Errorf("exec service not started")
	}
	pods := podStore.List()
	if len(pods) == 0 {
		return nil, fmt.Errorf("no exec pods found")
	}
	return pods[rand.Intn(len(pods))], nil
}
