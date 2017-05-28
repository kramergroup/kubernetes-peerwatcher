package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/fields"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

var kubeconfig string

var namespace string
var podname string

var clientset *kubernetes.Clientset

func main() {

	hostname, err := os.Hostname()
	if err != nil {
		panic(err.Error())
	}

	// Process command-line arguments
	flag.StringVar(&kubeconfig, "kubeconfig", "~/.kube/config", "path to kubeconfig file for out-of-cluster operation")
	flag.StringVar(&namespace, "namespace", v1.NamespaceDefault, "namespace for pod")
	flag.StringVar(&podname, "name", hostname, "name of pod")
	flag.Parse()

	// Configure

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		// uses the current context in kubeconfig if running outside of cluster
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(err.Error())
		}
	}

	// creates the clientset
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// Create Downstream and Watch
	pod, err := clientset.Pods(namespace).Get(podname)
	opts := fields.Set{"pod-template-hash": pod.Labels["pod-template-hash"]}.AsSelector()
	watchlist := cache.NewListWatchFromClient(clientset.Core().RESTClient(), "pods", namespace, opts)
	_, controller := cache.NewInformer(
		watchlist,
		&v1.Pod{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				fmt.Printf("add: %s \n", obj)
			},
			DeleteFunc: func(obj interface{}) {
				fmt.Printf("delete: %s \n", obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				fmt.Printf("old: %s, new: %s \n", oldObj, newObj)
			},
		},
	)
	stop := make(chan struct{})
	go controller.Run(stop)

}
