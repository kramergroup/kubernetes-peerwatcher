package main

import (
	"errors"
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

/* Global variable defintions */

// File name of kubeconfig for out-of-cluster usecase
var kubeconfig string

// Namespace and podname of peer
var namespace string
var podname string

// Kubernetes API source
var clientset *kubernetes.Clientset

// Local cache of API objects
var downstream cache.Store

// Controller to act on watched API changes
var controller *cache.Controller

func main() {

	// First determine the hostname, which is used as podname if none is given as parameter
	hostname, err := os.Hostname()
	if err != nil {
		panic(err.Error())
	}

	// Process command-line arguments
	{
		// kubeconfig - filename of out-of-cluster configuration
		flag.StringVar(&kubeconfig, "kubeconfig", "~/.kube/config", "path to kubeconfig file for out-of-cluster operation")

		// namespace - the namespace of the pod to watch
		flag.StringVar(&namespace, "namespace", v1.NamespaceDefault, "namespace for pod")

		// name - the name of the pod to watch
		flag.StringVar(&podname, "name", hostname, "name of pod")

		// parse command-line arguments
		flag.Parse()
	}

	// Configure Kubernetes API source
	{
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

	}

	// Get the pod from the API source
	pod, err := clientset.Pods(namespace).Get(podname)
	if err != nil {
		panic(err.Error())
	}

	// Find the owner of the pod and extract selector to find other
	// pods managed by the same owner
	selector, err := GetSelectorForPodsFromOwnerReference(pod)
	if err != nil {
		panic(err.Error())
	}

	// Create Downstream and Watch
	opts := v1.ListOptions{LabelSelector: selector.String()}
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
				fmt.Printf("add: %s (%d)\n", obj.(*v1.Pod).Name,
					len(downstream.ListKeys()))
			},
			DeleteFunc: func(obj interface{}) {
				fmt.Printf("delete: %s (%d)\n", obj.(*v1.Pod).Name,
					len(downstream.ListKeys()))
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				fmt.Printf("old: %s, new: %s \n",
					oldObj.(*v1.Pod).Name,
					newObj.(*v1.Pod).Name)
			},
		},
	)

	fmt.Printf("Start watching pods with selector: %s \n", selector.String())
	stop := make(chan struct{})
	go controller.Run(stop)

	// Run forever
	for {
		time.Sleep(time.Second)
	}
}

// GetSelectorForPodsFromOwnerReference obtains the pod selector from the
// ownerReference of the pod
func GetSelectorForPodsFromOwnerReference(pod *v1.Pod) (selector fields.Selector, err error) {

	// First handle cases where the pod declares an owner (at the moment that seems to be only supported by ReplicaSet)
	if len(pod.OwnerReferences) == 0 {
		return nil, errors.New("Pod has no owner reference.")
	}

	podOwnerRef := pod.OwnerReferences[0]

	// Handle different owners separately (even if only ReplicaSet seems to declare ownership in pod defintions at the moment)
	switch podOwnerRef.Kind {
	case "ReplicaSet":
		owner, _ := clientset.ReplicaSets(namespace).Get(podOwnerRef.Name)
		labels := fields.Set(owner.Spec.Selector.MatchLabels).AsSelector()
		return labels, nil
	case "DeamonSet": // Untested
		owner, _ := clientset.DaemonSets(namespace).Get(podOwnerRef.Name)
		labels := fields.Set(owner.Spec.Selector.MatchLabels).AsSelector()
		return labels, nil
	case "ReplicationController": // Untested
		owner, _ := clientset.ReplicationControllers(namespace).Get(podOwnerRef.Name)
		labels := fields.Set(owner.Spec.Selector).AsSelector()
		return labels, nil
	default:
		return nil, errors.New("Unknown owner type " + podOwnerRef.Kind)
	}

}
