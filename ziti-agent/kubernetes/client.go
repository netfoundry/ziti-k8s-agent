package kubernetes

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

func Client() *kubernetes.Clientset {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatal(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err.Error())
	}
	return clientset
}
