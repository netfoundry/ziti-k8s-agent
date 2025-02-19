package kubernetes

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	dnsServiceName      = "kube-dns"
	dnsServiceNamespace = "kube-system"
	cacheTTL            = 5 * time.Minute
)

var (
	lastFetch     time.Time
	cachedDNSIP   string
)

func GetClusterDNSIP(client kubernetes.Interface) (string, error) {
	if time.Since(lastFetch) < cacheTTL && cachedDNSIP != "" {
		klog.V(4).Info("Using cached cluster DNS IP")
		return cachedDNSIP, nil
	}

	svc, err := client.CoreV1().Services(dnsServiceNamespace).Get(
		context.Background(),
		dnsServiceName,
		metav1.GetOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("failed to get %s service: %v", dnsServiceName, err)
	}

	if len(svc.Spec.ClusterIP) == 0 {
		return "", fmt.Errorf("%s service has no ClusterIP", dnsServiceName)
	}

	cachedDNSIP = svc.Spec.ClusterIP
	lastFetch = time.Now()
	klog.V(3).Infof("Updated cluster DNS IP to %s", cachedDNSIP)
	return cachedDNSIP, nil
}