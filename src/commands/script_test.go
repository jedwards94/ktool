package commands

import (
	"bytes"
	"context"
	"ktool/src/logger"
	"os"
	"reflect"
	"testing"

	"github.com/go-test/deep"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	testImage   = "python"
	testShell   = "python"
	testScript  = "./fake.py"
	testJobTmpl = ""
	log         = logger.Logger.WithGlobal(logger.LogOptionsParams{
		Level: "off",
	})
)

func loadYamlToJob(path string) *batchv1.Job {
	job := &batchv1.Job{}
	yamlJob, _ := os.ReadFile("../test_assets/" + path)
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlJob), 1024)
	decoder.Decode(job)
	return job
}

func TestScriptComandInstance(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
		Image:       testImage,
		Shell:       testShell,
		Script:      testScript,
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
		Script:      testScript,
		Args:        "",
		JobTemplate: "",
		Attach:      false,
		DryRun:      false,
	})

	template, err := cmd.loadTemplate()

	if err != nil {
		t.Errorf("template should not return error %v", err)
	}
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
	cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
		Image:       testImage,
		Shell:       testShell,
		Script:      testScript,
		Args:        "",
		JobTemplate: "../test_assets/TestLoadTemplate.yaml",
		Attach:      false,
		DryRun:      false,
	})

	template, err := cmd.loadTemplate()
	if err != nil {
		t.Errorf("template should not return error %v", err)
	}

	result := *template
	expected := *loadYamlToJob("TestLoadTemplate.yaml")

	if diff := deep.Equal(expected, result); diff != nil {
		t.Errorf("Template should not present differences: %v", diff)
	}

}

func TestLoadTemplateInvalidPath(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
		Image:       testImage,
		Shell:       testShell,
		Script:      testScript,
		Args:        "",
		JobTemplate: "invalidPath",
		Attach:      false,
		DryRun:      false,
	})

	_, err := cmd.loadTemplate()
	if err == nil {
		t.Error("an invalid path should ruturn an error")
	}
}
func TestLoadTemplateInvalidYAML(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
		Image:       testImage,
		Shell:       testShell,
		Script:      testScript,
		Args:        "",
		JobTemplate: "../test_assets/TestLoadTemplateInvalidYAML.yaml",
		Attach:      false,
		DryRun:      false,
	})

	_, err := cmd.loadTemplate()
	if err == nil {
		t.Error("an invalid yaml should ruturn an error")
	}
}

func TestLoadScript(t *testing.T) {

	clientset := fake.NewSimpleClientset()
	cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
		Image:       testImage,
		Shell:       testShell,
		Script:      "../test_assets/TestLoadScript.py",
		Args:        "",
		JobTemplate: testJobTmpl,
		Attach:      false,
		DryRun:      false,
	})

	result, err := cmd.loadScript()
	expected := "print(\"test\")"

	if err != nil {
		t.Errorf("script should not return error %v", err)
	}

	if *result != expected {
		t.Errorf("script should be %s but got %s", expected, *result)
	}

}
func TestLoadScriptInvalidPath(t *testing.T) {

	clientset := fake.NewSimpleClientset()
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

}

func TestCreateJobManifest(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
		Image:       testImage,
		Shell:       testShell,
		Script:      testScript,
		Args:        "",
		JobTemplate: "../test_assets/TestCreateJobManifest.yaml",
		Attach:      false,
		DryRun:      false,
		Namespace:   "test",
	})

	cmd.uuid = "1"

	job, err := cmd.createJobManifest()
	result := *job
	expected := *loadYamlToJob("ExpectedTestCreateJobManifest.yaml")
	if err != nil {
		t.Error("should not return an error")
	}

	if diff := deep.Equal(expected, result); diff != nil {
		t.Errorf("Job manifest should not present differences: %v", diff)
	}

}

func TestCreateJobManifestWithoutTemplate(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
		Image:       testImage,
		Shell:       testShell,
		Script:      testScript,
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
	if err != nil {
		t.Error("should not return an error")
	}

	if diff := deep.Equal(expected, result); diff != nil {
		t.Errorf("job manifest should not present differences: %v", diff)
	}
}

func TestCreateJob(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	cmd := ScriptCommand{}.NewWithFlags(clientset, log, ScriptFlags{
		Image:       testImage,
		Shell:       testShell,
		Script:      testScript,
		Args:        "",
		JobTemplate: "",
		Attach:      false,
		DryRun:      false,
		Namespace:   "test",
	})

	cmd.uuid = "1"

	err := cmd.createJob()
	if err != nil {
		t.Error("should not return error")
	}

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
