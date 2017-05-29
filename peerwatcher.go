package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/fields"
	"k8s.io/client-go/pkg/runtime"
	"k8s.io/client-go/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

var kubeconfig string

var namespace string
var podname string

var clientset *kubernetes.Clientset

var downstream cache.Store
var controller *cache.Controller

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

	// Find the owner of the pod and extract selector to find other
	// pods managed by the same owner
	pod, err := clientset.Pods(namespace).Get(podname)
	podOwnerRef := pod.OwnerReferences[0]
	owner, _ := clientset.ReplicaSets(namespace).Get(podOwnerRef.Name)
	labels := fields.Set(owner.Spec.Selector.MatchLabels)
	opts := v1.ListOptions{LabelSelector: labels.AsSelector().String()}

	// Create Downstream and Watch
	watchlist := cache.ListWatch{
		ListFunc: func(options api.ListOptions) (runtime.Object, error) {
			return clientset.Core().Pods(namespace).List(opts)
		},
		WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
			return clientset.Core().Pods(namespace).Watch(opts)
		},
	}

	downstream, controller = cache.NewInformer(
		&watchlist,
		&v1.Pod{},
		time.Minute,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				fmt.Printf("add: %s (%d)\n", obj.(*v1.Pod).Name, len(downstream.ListKeys()))
			},
			DeleteFunc: func(obj interface{}) {
				fmt.Printf("delete: %s (%d)\n", obj.(*v1.Pod).Name, len(downstream.ListKeys()))
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				fmt.Printf("old: %s, new: %s \n", oldObj.(*v1.Pod).Name, newObj.(*v1.Pod).Name)
			},
		},
	)

	fmt.Printf("Start watching pods with selector: %s \n", labels.AsSelector().String())
	stop := make(chan struct{})
	go controller.Run(stop)

	// Run forever
	for {
		time.Sleep(time.Second)
	}
}
