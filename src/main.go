package main

import (
	"flag"
	"fmt"
	"ktool/src/commands"
	"os"
	"path/filepath"

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
		fmt.Printf("vuild time: %s\n", BuildTime)
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
	scriptFile := scriptCmd.String("file", "", "(required) the script file to run")
	scriptBaseImage := scriptCmd.String("image", "", "(required) the image where the script will be runned")
	scriptJobTemplateFile := scriptCmd.String("template", "", "(optional) the kubernetes Job template to use")
	scriptShell := scriptCmd.String("shell", "/bin/sh -c", "(optional) the shell to use")
	scriptArgs := scriptCmd.String("args", "", "(optional) the args to pass to the script file")
	scriptNamespace := scriptCmd.String("namespace", "default", "(optional) the namespace where to run the script")
	scriptAttach := scriptCmd.Bool("attach", false, "(optional) follow logs")
	scriptDryRun := scriptCmd.Bool("dry-run", false, "(optional) print the kubernetes manifests but doesn't run anything")

	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	command := args[0]

	switch command {
	case "version":
		versionCmd.Usage()
	case "script":
		scriptCmd.Parse(args[1:])
		if pass := checkRequired([]string{"-file", "-image"}, args[1:]); !pass {
			scriptCmd.Usage()
			os.Exit(1)
		}
		cmd := commands.ScriptCommand{}.NewWithFlags(*clientset, commands.ScriptFlags{
			JobTemplate: scriptJobTemplateFile,
			Attach:      *scriptAttach,
			DryRun:      *scriptDryRun,
			Image:       scriptBaseImage,
			Shell:       scriptShell,
			Script:      scriptFile,
			Args:        scriptArgs,
			Namespace:   scriptNamespace,
		})

		cmd.Exec()

	}

}
