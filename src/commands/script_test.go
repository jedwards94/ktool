package commands

import (
	"bytes"
	"context"
	"fmt"
	"ktool/src/logger"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/go-test/deep"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/k3s"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	testImage         = "busybox"
	testShell         = "bin/sh"
	successTestScript = "../test_assets/commands/script/success.sh"
	failTestScript    = "../test_assets/commands/script/fail.sh"
	testJobTmpl       = ""
	log               = logger.Logger.WithGlobal(logger.LogOptionsParams{
		Level: "off",
	})
)

func initK3S(t *testing.T, images ...string) kubernetes.Interface {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(3*time.Minute))
	defer cancel()

	k3sContainer, err := k3s.Run(ctx, "rancher/k3s:v1.27.1-k3s1")
	testcontainers.CleanupContainer(t, k3sContainer)
	require.NoError(t, err)

	kubeConfigYaml, err := k3sContainer.GetKubeConfig(ctx)
	require.NoError(t, err)

	restcfg, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigYaml)
	require.NoError(t, err)

	clientset, err := kubernetes.NewForConfig(restcfg)
	require.NoError(t, err)

	provider, err := testcontainers.ProviderDocker.GetProvider()
	require.NoError(t, err)
	for i := range images {
		err = provider.PullImage(ctx, images[i])
		require.NoError(t, err)
	}
	return clientset
}

func loadYamlToJob(path string) *batchv1.Job {
	job := &batchv1.Job{}
	yamlJob, _ := os.ReadFile("../test_assets/commands/script/" + path)
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlJob), 1024)
	decoder.Decode(job)
	return job
}
func loadYamlToCM(path string) *v1.ConfigMap {
	cm := &v1.ConfigMap{}
	yamlCm, _ := os.ReadFile("../test_assets/commands/script/" + path)
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlCm), 1024)
	decoder.Decode(cm)
	return cm
}

func TestScriptComandInstance(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
		Image:       testImage,
		Shell:       testShell,
		Script:      successTestScript,
		Args:        "",
		JobTemplate: "",
		Attach:      false,
		DryRun:      false,
	})

	emptyFlagSet := ScriptFlags{}

	if cmd.flags == emptyFlagSet {
		t.Error("flags should't be empty")
	}

	if cmd.configMap != nil {
		t.Error("configMap should be nil at creation")
	}

	if cmd.job != nil {
		t.Error("job should be nil at creation")
	}

	if cmd.kClient == nil {
		t.Error("kubernetes client shouldn't be nil at creation")
	}

	if len(cmd.uuid) != 5 {
		t.Error("uuid should have length 5")
	}

}

func TestLoadEmptyTemplate(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
		Image:       testImage,
		Shell:       testShell,
		Script:      successTestScript,
		Args:        "",
		JobTemplate: "",
		Attach:      false,
		DryRun:      false,
	})

	template, err := cmd.loadTemplate()
	require.NoError(t, err)
	result := *template
	expected := batchv1.Job{
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{},
					},
				},
			},
		},
	}

	if !reflect.DeepEqual(result, expected) {
		t.Error("templateJob should be empty Job when empty")
	}

}

func TestLoadTemplate(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	t.Run("load valid template", func(t *testing.T) {
		cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
			Image:       testImage,
			Shell:       testShell,
			Script:      successTestScript,
			Args:        "",
			JobTemplate: "../test_assets/commands/script/TestLoadTemplate.yaml",
			Attach:      false,
			DryRun:      false,
		})

		template, err := cmd.loadTemplate()
		require.NoError(t, err)

		result := *template
		expected := *loadYamlToJob("TestLoadTemplate.yaml")

		if diff := deep.Equal(expected, result); diff != nil {
			t.Errorf("Template should not present differences: %v", diff)
		}
	})

	t.Run("with invalid path", func(t *testing.T) {
		cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
			Image:       testImage,
			Shell:       testShell,
			Script:      successTestScript,
			Args:        "",
			JobTemplate: "invalidPath",
			Attach:      false,
			DryRun:      false,
		})

		_, err := cmd.loadTemplate()
		if err == nil {
			t.Error("an invalid path should ruturn an error")
		}
	})

	t.Run("with invalid yaml", func(t *testing.T) {
		cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
			Image:       testImage,
			Shell:       testShell,
			Script:      successTestScript,
			Args:        "",
			JobTemplate: "../test_assets/commands/script/TestLoadTemplateInvalidYAML.yaml",
			Attach:      false,
			DryRun:      false,
		})

		_, err := cmd.loadTemplate()
		if err == nil {
			t.Error("an invalid yaml should ruturn an error")
		}
	})

}

func TestLoadScript(t *testing.T) {

	clientset := fake.NewSimpleClientset()
	t.Run("with valid script", func(t *testing.T) {
		cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
			Image:       testImage,
			Shell:       testShell,
			Script:      successTestScript,
			Args:        "",
			JobTemplate: testJobTmpl,
			Attach:      false,
			DryRun:      false,
		})

		result, err := cmd.loadScript()
		expected := `echo "test 1"
sleep 2
echo "test 2"
sleep 2
echo "test 3"
`

		require.NoError(t, err)

		if *result != expected {
			t.Errorf("script should be %s but got %s", expected, *result)
		}
	})

	t.Run("with invalid path", func(t *testing.T) {
		cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
			Image:       testImage,
			Shell:       testShell,
			Script:      "invalidPath",
			Args:        "",
			JobTemplate: testJobTmpl,
			Attach:      false,
			DryRun:      false,
		})

		_, err := cmd.loadScript()

		if err == nil {
			t.Error("an invalid path should return an error")
		}
	})

}

func TestCreateJobManifest(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	t.Run("with template", func(t *testing.T) {
		cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
			Image:       testImage,
			Shell:       testShell,
			Script:      successTestScript,
			Args:        "",
			JobTemplate: "../test_assets/commands/script/TestCreateJobManifest.yaml",
			Attach:      false,
			DryRun:      false,
			Namespace:   "test",
		})

		cmd.uuid = "1"

		job, err := cmd.createJobManifest()
		result := *job
		expected := *loadYamlToJob("ExpectedTestCreateJobManifest.yaml")
		require.NoError(t, err)

		if diff := deep.Equal(expected, result); diff != nil {
			t.Errorf("Job manifest should not present differences: %v", diff)
		}
	})

	t.Run("without template", func(t *testing.T) {
		cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
			Image:       testImage,
			Shell:       testShell,
			Script:      successTestScript,
			Args:        "",
			JobTemplate: "",
			Attach:      false,
			DryRun:      false,
			Namespace:   "test",
		})

		cmd.uuid = "1"

		job, err := cmd.createJobManifest()
		result := *job
		expected := *loadYamlToJob("ExpectedJobWithoutTemplate.yaml")
		require.NoError(t, err)

		if diff := deep.Equal(expected, result); diff != nil {
			t.Errorf("job manifest should not present differences: %v", diff)
		}

	})

}

func TestCreateJob(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
		Image:       testImage,
		Shell:       testShell,
		Script:      successTestScript,
		Args:        "",
		JobTemplate: "",
		Attach:      false,
		DryRun:      false,
		Namespace:   "test",
	})

	cmd.uuid = "1"

	err := cmd.createJob()
	require.NoError(t, err)

	jobs, _ := clientset.BatchV1().Jobs("test").List(context.TODO(), metav1.ListOptions{})

	if len(jobs.Items) == 0 {
		t.Errorf("should be 1 job, got %d", len(jobs.Items))
	}

	result := jobs.Items[0]
	expected := *loadYamlToJob("ExpectedJobWithoutTemplate.yaml")

	if diff := deep.Equal(expected, result); diff != nil {
		t.Errorf("job manifest should not present differences: %v", diff)
	}

}

func TestDeleteJob(t *testing.T) {
	clientset := fake.NewSimpleClientset(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ktool-script-1",
			Namespace: "test",
		},
	})
	cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
		Image:       testImage,
		Shell:       testShell,
		Script:      successTestScript,
		Args:        "",
		JobTemplate: "",
		Attach:      false,
		DryRun:      false,
		Namespace:   "test",
	})

	cmd.uuid = "1"
	cmd.job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ktool-script-1",
			Namespace: "test",
		},
	}

	err := cmd.deleteJob()
	require.NoError(t, err)

	jobs, _ := clientset.BatchV1().Jobs("test").List(context.TODO(), metav1.ListOptions{})

	if len(jobs.Items) != 0 {
		t.Errorf("should be 0 job, got %d", len(jobs.Items))
	}

}

func TestCreateConfigMapManifest(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
		Image:       testImage,
		Shell:       testShell,
		Script:      successTestScript,
		Args:        "",
		JobTemplate: "",
		Attach:      false,
		DryRun:      false,
		Namespace:   "test",
	})

	cmd.uuid = "1"

	cm, err := cmd.createConfigMapManifest()
	result := *cm
	expected := *loadYamlToCM("ExpectedConfigMap.yaml")
	require.NoError(t, err)

	if diff := deep.Equal(expected, result); diff != nil {
		t.Errorf("config map manifest should not present differences: %v", diff)
	}

}

func TestWatchPod(t *testing.T) {
	clientset := initK3S(t, testImage)
	t.Run("when the pod complete succesfully", func(t *testing.T) {
		cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
			Image:       testImage,
			Shell:       testShell,
			Script:      successTestScript,
			Args:        "",
			JobTemplate: "",
			Attach:      false,
			DryRun:      false,
			Namespace:   "default",
		})

		err := cmd.createConfigMap()
		require.NoError(t, err)
		defer cmd.deleteConfigMap()

		err = cmd.createJob()
		require.NoError(t, err)
		defer cmd.deleteJob()

		pod, err := cmd.jobPod()
		require.NoError(t, err)

		cmd.waitForPodToStart(pod)

		watcher := make(chan error)
		defer close(watcher)
		go cmd.watchPod(watcher, pod.Name)
		result := <-watcher
		require.NoError(t, result)
	})

	t.Run("when the pod dies unexpectedly", func(t *testing.T) {
		cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
			Image:       testImage,
			Shell:       testShell,
			Script:      successTestScript,
			Args:        "",
			JobTemplate: "",
			Attach:      false,
			DryRun:      false,
			Namespace:   "default",
		})

		err := cmd.createConfigMap()
		require.NoError(t, err)
		defer cmd.deleteConfigMap()

		err = cmd.createJob()
		require.NoError(t, err)
		defer cmd.deleteJob()

		pod, err := cmd.jobPod()
		require.NoError(t, err)

		cmd.waitForPodToStart(pod)

		watcher := make(chan error)
		defer close(watcher)
		go cmd.watchPod(watcher, pod.Name)

		gracePeriod := int64(0)
		clientset.CoreV1().Pods("default").Delete(context.Background(), pod.Name, metav1.DeleteOptions{
			GracePeriodSeconds: &gracePeriod,
		})

		result := <-watcher
		expected := fmt.Sprintf("watcher error. pod %s deleted", pod.Name)
		require.EqualError(t, result, expected)
	})

	t.Run("when the pod fails unexpectedly", func(t *testing.T) {
		cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
			Image:       testImage,
			Shell:       testShell,
			Script:      failTestScript,
			Args:        "",
			JobTemplate: "",
			Attach:      false,
			DryRun:      false,
			Namespace:   "default",
		})

		err := cmd.createConfigMap()
		require.NoError(t, err)
		defer cmd.deleteConfigMap()

		err = cmd.createJob()
		require.NoError(t, err)
		defer cmd.deleteJob()

		pod, err := cmd.jobPod()
		require.NoError(t, err)

		cmd.waitForPodToStart(pod)

		watcher := make(chan error)
		defer close(watcher)
		go cmd.watchPod(watcher, pod.Name)

		result := <-watcher
		require.NoError(t, result)
	})

}

func TestWaitForPodToStart(t *testing.T) {

	clientset := initK3S(t, testImage)
	t.Run("with exceeded deadline", func(t *testing.T) {
		cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
			Image:       testImage,
			Shell:       testShell,
			Script:      successTestScript,
			Args:        "",
			JobTemplate: "",
			Attach:      false,
			DryRun:      false,
			Namespace:   "default",
		})

		cmd.initTimeoutSeconds = 5

		err := cmd.createJob()
		require.NoError(t, err)
		defer cmd.deleteJob()

		pod, err := cmd.jobPod()
		require.NoError(t, err)

		err = cmd.waitForPodToStart(pod)
		if err != context.DeadlineExceeded {
			t.Errorf("should return timeout but got %v", err)
		}

	})

	t.Run("with failed status", func(t *testing.T) {
		cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
			Image:       testImage,
			Shell:       "invalidShell",
			Script:      failTestScript,
			Args:        "",
			JobTemplate: "",
			Attach:      false,
			DryRun:      false,
			Namespace:   "default",
		})

		cmd.initTimeoutSeconds = 120

		err := cmd.createConfigMap()
		require.NoError(t, err)
		defer cmd.deleteConfigMap()

		err = cmd.createJob()
		require.NoError(t, err)
		defer cmd.deleteJob()

		pod, err := cmd.jobPod()
		require.NoError(t, err)

		result := cmd.waitForPodToStart(pod)
		require.ErrorContains(t, result, "pod failed")

	})

	t.Run("with normal life cycle", func(t *testing.T) {
		cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
			Image:       testImage,
			Shell:       testShell,
			Script:      successTestScript,
			Args:        "",
			JobTemplate: "",
			Attach:      false,
			DryRun:      false,
			Namespace:   "default",
		})

		err := cmd.createConfigMap()
		require.NoError(t, err)
		defer cmd.deleteConfigMap()

		err = cmd.createJob()
		require.NoError(t, err)
		defer cmd.deleteJob()

		pod, err := cmd.jobPod()
		require.NoError(t, err)

		result := cmd.waitForPodToStart(pod)
		require.NoError(t, result)
	})

}

func TestStreamLog(t *testing.T) {
	clientset := initK3S(t, testImage)
	cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
		Image:       testImage,
		Shell:       testShell,
		Script:      successTestScript,
		Args:        "",
		JobTemplate: "",
		Attach:      false,
		DryRun:      false,
		Namespace:   "default",
	})

	err := cmd.createConfigMap()
	require.NoError(t, err)
	defer cmd.deleteConfigMap()

	err = cmd.createJob()
	require.NoError(t, err)
	defer cmd.deleteJob()

	pod, err := cmd.jobPod()
	require.NoError(t, err)

	podRunning := make(chan bool)
	doneLogs := make(chan int)
	defer close(podRunning)
	defer close(doneLogs)

	cmd.waitForPodToStart(pod)

	result, err := cmd.streamPodLogs(pod.Name)
	require.NoError(t, err)
	expected := 3

	if result != expected {
		t.Errorf("expected %d log read but got: %d", expected, result)
	}

}
