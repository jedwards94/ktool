package commands

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"ktool/src/logger"
	path_utils "ktool/src/utils/path"
	string_utils "ktool/src/utils/strings"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/imdario/mergo"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

const (
	appLabel = "ktool-app"
	extLabel = "script-extension"
	annoType = "app.ktool.io/type"
	annoUuid = "app.ktool.io/short-uuid"

	genName       = "ktool-script-"
	containerName = "ktool-script"
	mountName     = "ktool-scripts"
	mountPath     = "/ktool"
	volName       = "ktool-scripts"

	jobType       = "Job"
	jobApiVersion = "batch/v1"

	configMapType       = "ConfigMap"
	configMapApiVersion = "v1"

	cmdFmt           = "/ktool/script%s"
	cmNameFmt        = "ktool-script-%s"
	cmDataKeyFmt     = "script%s"
	labelSelectorFmt = "job-name=%s"
)

type ScriptFlags struct {
	JobTemplate string
	Attach      bool
	DryRun      bool
	Image       string
	Namespace   string
	Shell       string
	Script      string
	Args        string
}

type ScriptCommand struct {
	flags              ScriptFlags
	kClient            kubernetes.Interface
	uuid               string
	context            context.Context
	job                *batchv1.Job
	configMap          *v1.ConfigMap
	logger             logger.Log
	initTimeoutSeconds int
}

// Creates the command struct ready to execute
func (s ScriptCommand) NewWithFlags(
	kClient kubernetes.Interface,
	log logger.Log,
	flags ScriptFlags,
) ScriptCommand {
	return ScriptCommand{
		flags:              flags,
		kClient:            kClient,
		uuid:               uuid.NewString()[:5],
		context:            context.Background(),
		logger:             log.New("script"),
		initTimeoutSeconds: 300,
	}
}

// Loads the Job template from the flag path and returns it as a *v1.Job
func (s *ScriptCommand) loadTemplate() (*batchv1.Job, error) {
	job := &batchv1.Job{}
	if s.flags.JobTemplate != "" {
		yamlFile, _ := path_utils.ExpandUser(s.flags.JobTemplate)

		s.logger.Debug("loading template job from %s", yamlFile)
		yamlJob, err := os.ReadFile(yamlFile)
		if err != nil {
			return nil, fmt.Errorf("fatal error while loading template, %v", err)
		}

		s.logger.Debug("decoding template to job yaml")
		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlJob), 1024)
		decodeErr := decoder.Decode(job)
		if decodeErr != nil {
			return nil, fmt.Errorf("fatal error while decoding template, %v", err)
		}
	}

	// Make sure to have at least one empty container to overwrite
	if len(job.Spec.Template.Spec.Containers) == 0 {
		s.logger.Debug("no container in template, adding empty one")
		job.Spec.Template.Spec.Containers = []v1.Container{
			{},
		}
	}
	return job, nil
}

// Loads the script to be executed in the Job from the flag path and returns it as a string
func (s *ScriptCommand) loadScript() (*string, error) {
	scriptString := ""
	if s.flags.Script != "" {
		file, _ := path_utils.ExpandUser(s.flags.Script)

		s.logger.Debug("loading script from %s", file)
		script, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("fatal error while reading script file, %v", err)

		}

		scriptString = string(script[:])
	}
	return &scriptString, nil
}

// Creates a *v1.Job manifest, it takes the template (if none was passed a default empty one will be created)
// and replaces the items in the container at index 0, replacing its name, image, pullPolicy
// and merges the volumes, mounts, annotations and labels from the original template
func (s *ScriptCommand) createJobManifest() (*batchv1.Job, error) {
	template, err := s.loadTemplate()
	if err != nil {
		return nil, err
	}

	templateLabels := template.ObjectMeta.Labels
	if templateLabels == nil {
		templateLabels = map[string]string{}
	}
	templateLabels[appLabel] = "true"
	templateLabels[extLabel] = strings.TrimPrefix(path.Ext(s.flags.Script), ".")

	templateAnnotations := template.ObjectMeta.Annotations
	if templateAnnotations == nil {
		templateAnnotations = map[string]string{}
	}
	templateAnnotations[annoType] = "script"
	templateAnnotations[annoUuid] = s.uuid

	defaultMode := int32(0777)
	ttl := int32(10)
	gracePeriod := int64(1)

	generateName := genName
	if template.ObjectMeta.GenerateName != "" {
		generateName = template.ObjectMeta.GenerateName
	}

	volumes := append(
		template.Spec.Template.Spec.Volumes,
		v1.Volume{
			Name: volName,
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					DefaultMode: &defaultMode,
					LocalObjectReference: v1.LocalObjectReference{
						Name: fmt.Sprintf(cmNameFmt, s.uuid),
					},
				},
			},
		},
	)
	volumeMounts := append(
		template.Spec.Template.Spec.Containers[0].VolumeMounts,
		v1.VolumeMount{
			Name:      mountName,
			MountPath: mountPath,
		},
	)

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    s.flags.Namespace,
			GenerateName: generateName,
			Labels:       templateLabels,
			Annotations:  templateAnnotations,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       jobType,
			APIVersion: jobApiVersion,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttl,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:            containerName,
							Image:           s.flags.Image,
							ImagePullPolicy: v1.PullAlways,
							Command:         append(string_utils.ArgsToList((s.flags.Shell)), fmt.Sprintf(cmdFmt, path.Ext(s.flags.Script))),
							Args:            string_utils.ArgsToList(s.flags.Args),
							VolumeMounts:    volumeMounts,
							Env:             template.Spec.Template.Spec.Containers[0].Env,
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
					Volumes:       volumes,
				},
			},
		},
	}

	s.logger.Debug("overwriting %#v with %#v", template.ObjectMeta, &job.ObjectMeta)
	if err := mergo.Merge(&(template.ObjectMeta), job.ObjectMeta, mergo.WithOverride); err != nil {
		return nil, err
	}

	s.logger.Debug("overwriting %#v with %#v", template.TypeMeta, &job.TypeMeta)
	if err := mergo.Merge(&(template.TypeMeta), job.TypeMeta, mergo.WithOverride); err != nil {
		return nil, err
	}
	if template.Spec.TTLSecondsAfterFinished == nil {
		s.logger.Debug("setting ttl to %d", ttl)
		template.Spec.TTLSecondsAfterFinished = &ttl
	}
	if template.Spec.Template.Spec.TerminationGracePeriodSeconds == nil {
		s.logger.Debug("setting termination grace period to %d", gracePeriod)
		template.Spec.Template.Spec.TerminationGracePeriodSeconds = &gracePeriod
	}

	s.logger.Debug("pod volumes %#v", job.Spec.Template.Spec.Volumes)
	template.Spec.Template.Spec.Volumes = job.Spec.Template.Spec.Volumes

	s.logger.Debug("pod restart policy %#v", job.Spec.Template.Spec.RestartPolicy)
	template.Spec.Template.Spec.RestartPolicy = job.Spec.Template.Spec.RestartPolicy

	s.logger.Debug("overwriting template first container with %#v", &job.Spec.Template.Spec.Containers[0])
	if err := mergo.Merge(&(template.Spec.Template.Spec.Containers[0]), job.Spec.Template.Spec.Containers[0], mergo.WithOverride); err != nil {
		return nil, err
	}

	return template, nil
}

// Creates the Job in the cluster
func (s *ScriptCommand) createJob() error {
	job, err := s.createJobManifest()
	if err != nil {
		return err
	}
	createdJob, err := s.kClient.BatchV1().Jobs(s.flags.Namespace).Create(s.context, job, metav1.CreateOptions{})
	if err != nil {

		return err
	}
	s.logger.Debug("created job %s on namespace %s", createdJob.Name, createdJob.Namespace)
	s.job = createdJob
	return nil
}

// Deletes the Job from the cluster
func (s *ScriptCommand) deleteJob() error {
	if s.job == nil {
		return nil
	}
	delPol := metav1.DeletePropagationForeground
	delOptions := metav1.DeleteOptions{
		PropagationPolicy: &delPol,
	}
	if err := s.kClient.BatchV1().Jobs(s.flags.Namespace).Delete(s.context, s.job.Name, delOptions); err != nil {
		return err
	}
	s.logger.Debug("deleted job %s on namespace %s", s.job.Name, s.job.Namespace)
	s.job = nil
	return nil
}

// Loads the script as string and creates a *v1.ConfigMap that contains a item called
// script<.extension> with the data from the script
func (s *ScriptCommand) createConfigMapManifest() (*v1.ConfigMap, error) {
	script, err := s.loadScript()
	if err != nil {
		return nil, err
	}
	cnf := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(cmNameFmt, s.uuid),
			Namespace: s.flags.Namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       configMapType,
			APIVersion: configMapApiVersion,
		},
		Data: map[string]string{
			fmt.Sprintf(cmDataKeyFmt, path.Ext(s.flags.Script)): *script,
		},
	}
	s.logger.Debug("configmap script data %#v", cnf.Data)
	return cnf, nil
}

// Creates the ConfigMap in the cluster
func (s *ScriptCommand) createConfigMap() error {
	confMan, err := s.createConfigMapManifest()
	if err != nil {
		return err
	}

	conf, err := s.kClient.CoreV1().ConfigMaps(s.flags.Namespace).Create(s.context, confMan, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	s.logger.Debug("created configmap %s on namespace %s", conf.Name, conf.Namespace)
	s.configMap = conf
	return nil
}

// Deletes the ConfigMap from the cluster
func (s *ScriptCommand) deleteConfigMap() error {
	if s.configMap == nil {
		return nil
	}
	delPol := metav1.DeletePropagationBackground
	gracePeriod := int64(1)
	if err := s.kClient.CoreV1().ConfigMaps(s.flags.Namespace).Delete(s.context, s.configMap.Name, metav1.DeleteOptions{
		PropagationPolicy:  &delPol,
		GracePeriodSeconds: &gracePeriod,
	}); err != nil {
		return err
	}
	s.logger.Debug("deleted configmap %s on namespace %s", s.configMap.Name, s.configMap.Namespace)
	s.configMap = nil
	return nil
}

// Gets the pod related to the Job created
func (s *ScriptCommand) jobPod() (*v1.Pod, error) {
	labelSelector := fmt.Sprintf(labelSelectorFmt, s.job.Name)
	s.logger.Debug("listing pods with label selector \"%s\" on namespace %s", labelSelector, s.flags.Namespace)
	pods, err := s.kClient.CoreV1().Pods(s.flags.Namespace).List(s.context, metav1.ListOptions{
		LabelSelector: labelSelector,
		Limit:         1,
	})
	if err != nil {
		return nil, err
	}

	for len(pods.Items) == 0 {
		pods, err = s.kClient.CoreV1().Pods(s.flags.Namespace).List(s.context, metav1.ListOptions{
			LabelSelector: labelSelector,
			Limit:         1,
		})
		if err != nil {
			return nil, err
		}
	}
	return &pods.Items[0], nil
}

// Watchs the pod status and phase to notify when it dies or ends successfully
// Intended to be called a a go rotuine
func (s *ScriptCommand) watchPod(done chan error, name string) {
	l := s.logger.New(name)
	s.logger.Debug("monitoring pod \"%s\" started", name)
	fieldSelector := fmt.Sprintf("metadata.name=%s", name)
	watcher, err := s.kClient.CoreV1().Pods(s.flags.Namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		done <- err
	}
	defer watcher.Stop()
	for event := range watcher.ResultChan() {
		switch event.Type {
		case watch.Added:
			continue
		case watch.Modified:
			pod := event.Object.(*v1.Pod)
			if pod.Status.ContainerStatuses[0].State.Terminated != nil {
				l.Info("exit code: %d", pod.Status.ContainerStatuses[0].State.Terminated.ExitCode)
				l.Info("reason: %s", pod.Status.ContainerStatuses[0].State.Terminated.Reason)
				l.Info("message: %s", pod.Status.ContainerStatuses[0].State.Terminated.Message)
				done <- nil
				return
			}
		case watch.Deleted:
			pod := event.Object.(*v1.Pod)
			done <- fmt.Errorf("watcher error. pod %s deleted", pod.Name)
			return
		case watch.Error:
			err := event.Object.(*metav1.Status)
			done <- fmt.Errorf("watcher error. apiStatus=%s message=%s reason=%s", err.Status, err.Message, err.Reason)
			return
		}
	}

}

// Streams the logs of a named pod to stdout and return the number of lines readed and the error
func (s *ScriptCommand) streamPodLogs(log chan struct {
	count int
	err   error
}, name string) {
	l := s.logger.New(name)
	linesOfLog := 0
	req := s.kClient.CoreV1().Pods(s.flags.Namespace).GetLogs(name, &v1.PodLogOptions{
		Follow:    true,
		Container: containerName,
	})
	stream, err := req.Stream(s.context)
	if err != nil {
		l.Error("error while staring log stream, %v", err)
		log <- struct {
			count int
			err   error
		}{linesOfLog, err}
	}
	defer stream.Close()
	buf := make([]byte, 1024)
	l.Info("starting log stream for pod %s on container ktool-script", name)
	for {
		n, err := stream.Read(buf)
		if err == io.EOF {
			l.Info("logs ended!")
			break
		}
		if err != nil {
			l.Error("error reading log stream: %v", err)
			log <- struct {
				count int
				err   error
			}{linesOfLog, err}
		}
		l.Log(string(buf[:n]))
		linesOfLog++
	}
	log <- struct {
		count int
		err   error
	}{linesOfLog, nil}
}

// Waits for a given pod poitner to be in Running Phase
func (s *ScriptCommand) waitForPodToStart(pod *v1.Pod) error {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Duration(s.initTimeoutSeconds)*time.Second))
	defer cancel()
	l := s.logger.New(pod.Name)
	l.Debug("waiting for pod %s to be running", pod.Name)
	labelSelector := fmt.Sprintf(labelSelectorFmt, s.job.Name)
	watcher, err := s.kClient.CoreV1().Pods(s.flags.Namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return err
	}
	defer watcher.Stop()
loop:
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-watcher.ResultChan():
			switch event.Type {
			case watch.Added:
				continue loop
			case watch.Modified:
				pod := event.Object.(*v1.Pod)
				switch pod.Status.Phase {
				case v1.PodPending, v1.PodUnknown:
					continue loop
				case v1.PodRunning, v1.PodSucceeded:
					break loop
				case v1.PodFailed:
					jobContainerStatus := pod.Status.ContainerStatuses[0].State
					err := fmt.Errorf("pod failed. message=\"%s\" reason=\"%s\"", pod.Status.Message, pod.Status.Reason)
					if jobContainerStatus.Terminated != nil {
						err = fmt.Errorf("pod failed. message=\"%s\" reason=\"%s\"", jobContainerStatus.Terminated.Message, jobContainerStatus.Terminated.Reason)
					}
					return err
				}
			case watch.Deleted:
				pod := event.Object.(*v1.Pod)
				return fmt.Errorf("watcher error. pod %s deleted", pod.Name)
			case watch.Error:
				err := event.Object.(*metav1.Status)
				return fmt.Errorf("watcher error. apiStatus=%s message=%s reason=%s", err.Status, err.Message, err.Reason)
			}
		}
	}
	l.Info("pod %s is running!", pod.Name)
	return nil
}

// Go routine that deletes the job and configmap when a SIGINT or SIGTERM are received
func (s *ScriptCommand) interruptClenaup() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	doneChan := make(chan os.Signal, 1)

	go func() {
		sig := <-sigChan
		s.logger.Info("%s received, cleaning up", sig.String())
		s.deleteConfigMap()
		s.deleteJob()
		doneChan <- sig
	}()

	<-doneChan
	s.logger.Info("cleanup ready, goodbye!")
}

// Runs the command. This creates the ConfigMap and the Job in the cluster, wait for the Job to have a
// Running Pod and start streaming it's logs (if attach flag is true) while monitor its status
func (s *ScriptCommand) run() error {
	if err := s.createConfigMap(); err != nil {
		return err
	}
	defer s.deleteConfigMap()

	if err := s.createJob(); err != nil {
		return err
	}
	defer s.deleteJob()

	pod, err := s.jobPod()
	if err != nil {
		return err
	}

	go s.interruptClenaup()

	if err := s.waitForPodToStart(pod); err != nil {
		return err
	}

	watcher := make(chan error)
	defer close(watcher)
	go s.watchPod(watcher, pod.Name)

	if s.flags.Attach {
		logChan := make(chan struct {
			count int
			err   error
		})
		defer close(logChan)
		go s.streamPodLogs(logChan, pod.Name)
		select {
		case err := <-watcher:
			if err != nil {
				return err
			}
		case res := <-logChan:
			if res.err != nil {
				return res.err
			}
		}

	} else {
		if err := <-watcher; err != nil {
			return err
		}
	}

	return nil
}

// Creates both ConfigMap and Job Manifests and prints them to stdout
func (s *ScriptCommand) dryRun() error {
	jobMan, err := s.createJobManifest()
	if err != nil {
		return err
	}
	configMan, err := s.createConfigMapManifest()
	if err != nil {
		return err
	}
	yamlSerializer := json.NewYAMLSerializer(json.DefaultMetaFactory, nil, nil)
	jobOut, _ := runtime.Encode(yamlSerializer, jobMan)
	configOut, _ := runtime.Encode(yamlSerializer, configMan)
	fmt.Println(string(jobOut) + "\n---\n" + string(configOut))
	return nil
}

// Execute the Script Command Routine.
// When dry-run flag is present, this will print both ConfigMap and Job manifests that will be applied.
// if it isn't present, this will create those manifests in the cluster and start monitoring the Pod
// additionally if the attach flag is true, this will also stream the logs to stdout.
func (s *ScriptCommand) Exec() error {
	if s.flags.DryRun {
		s.logger.Debug("dry run started")
		if err := s.dryRun(); err != nil {
			s.logger.Error("error on script dry run:  %v", err)
			return err
		}
	} else {
		if err := s.run(); err != nil {
			s.logger.Error("error on script run: %v", err)
			return err
		}
	}
	return nil

}
