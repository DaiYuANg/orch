package kubernetes

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type K8sManager struct {
	clientset *kubernetes.Clientset
	namespace string
}

func NewK8sManager(namespace string) (*K8sManager, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &K8sManager{clientset: cs, namespace: namespace}, nil
}
