package main

import (
	"flag"
	"fmt"
	"ktool/src/commands"
	"ktool/src/logger"
	"os"
	"path/filepath"
	"runtime"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var Version string
var Commit string
var BuildTime string

func checkRequired(reqFlags []string, flags []string) bool {
	result := false
	for _, reqFlag := range reqFlags {
		for _, flag := range flags {
			if flag == reqFlag {
				result = true
			}
		}
	}
	return result
}

func main() {

	versionCmd := flag.NewFlagSet("version", flag.ExitOnError)
	versionCmd.Usage = func() {
		fmt.Printf("version: %s\n", Version)
		fmt.Printf("commit: %s\n", Commit)
		fmt.Printf("build time: %s\n", BuildTime)
		fmt.Printf("platform: %s\n", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))
		versionCmd.PrintDefaults()
	}
	scriptCmd := flag.NewFlagSet("script", flag.ExitOnError)
	scriptCmd.Usage = func() {
		fmt.Println("script flags:")
		scriptCmd.PrintDefaults()
	}
	flag.Usage = func() {
		fmt.Println("Usage of ktool: ktool [FLAGS] [COMMAND] [COMMAND_FLAGS]")
		fmt.Println("ktool flags:")
		flag.PrintDefaults()
		fmt.Println("Available commands: script, version")
	}

	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	logNoColor := flag.Bool("no-color", false, "(optional) print logs without color")
	logLevel := flag.String("log-level", "info", "(optional) log level")

	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	log := logger.Logger.WithGlobal(logger.LogOptionsParams{
		NoColor: *logNoColor,
		Level:   *logLevel,
	})

	mainLog := log.New("main")

	scriptFile := scriptCmd.String("file", "", "(required) the script file to run")
	scriptBaseImage := scriptCmd.String("image", "", "(required) the image where the script will be runned")
	scriptJobTemplateFile := scriptCmd.String("template", "", "(optional) the kubernetes Job template to use")
	scriptShell := scriptCmd.String("shell", "/bin/sh -c", "(optional) the shell to use")
	scriptArgs := scriptCmd.String("args", "", "(optional) the args to pass to the script file")
	scriptNamespace := scriptCmd.String("namespace", "default", "(optional) the namespace where to run the script")
	scriptAttach := scriptCmd.Bool("attach", false, "(optional) follow logs")
	scriptDryRun := scriptCmd.Bool("dry-run", false, "(optional) print the kubernetes manifests but doesn't run anything")
	scriptInitTimeoutSeconds := scriptCmd.Uint("init-timeout-seconds", 300, "(optional) the time in seconds to wait until the job is running")
	scriptKeepResources := scriptCmd.Bool("keep-resources", false, "(optional) don't delete job and config")
	scriptKeepOnSig := scriptCmd.Bool("keep-on-sig", false, "(optional) don't terminate job or delete config if command receive a SIGINT or SIGTERM")

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		mainLog.Fatal("fatal error getting kubernetes config, %v", err)
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		mainLog.Fatal("fatal error creating kubernetes client, %v", err)
		panic(err.Error())
	}

	command := args[0]

	switch command {
	case "version":
		versionCmd.Usage()
	case "script":
		mainLog.Debug("started command script")
		scriptCmd.Parse(args[1:])
		if pass := checkRequired([]string{"-file", "-image"}, args[1:]); !pass {
			scriptCmd.Usage()
			panic(1)
		}
		cmd := commands.ScriptCommand{}.NewWithFlags(clientset, log, commands.ScriptFlags{
			JobTemplate:        *scriptJobTemplateFile,
			Attach:             *scriptAttach,
			DryRun:             *scriptDryRun,
			Image:              *scriptBaseImage,
			Shell:              *scriptShell,
			Script:             *scriptFile,
			Args:               *scriptArgs,
			Namespace:          *scriptNamespace,
			InitTimeoutSeconds: *scriptInitTimeoutSeconds,
			KeepOnSig:          *scriptKeepOnSig,
			KeepResources:      *scriptKeepResources,
		})

		if err := cmd.Exec(); err != nil {
			mainLog.Fatal("fatal error, %v", err)
			panic(err)
		}

	}

}
