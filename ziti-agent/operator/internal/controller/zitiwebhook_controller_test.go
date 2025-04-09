/*
Copyright 2025 NetFoundry.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubernetesv1alpha1 "github.com/netfoundry/ziti-k8s-agent/ziti-agent/operator/api/v1alpha1"
)

var _ = Describe("ZitiWebhook Controller", func() {

	const resourceName = "test-zitiwebhook"
	const resourceNamespace = "default"

	ctx := context.Background()

	typeNamespacedName := types.NamespacedName{
		Name:      resourceName,
		Namespace: resourceNamespace,
	}
	zitiwebhook := &kubernetesv1alpha1.ZitiWebhook{}
	issuer := &certmanagerv1.Issuer{}
	certificate := &certmanagerv1.Certificate{}
	service := &corev1.Service{}
	serviceAccount := &corev1.ServiceAccount{}
	clusterRole := &rbacv1.ClusterRole{}
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	mutatingWebhook := &admissionregistrationv1.MutatingWebhookConfiguration{}
	deployment := &appsv1.Deployment{}
	controllerReconciler := &ZitiWebhookReconciler{}

	BeforeEach(OncePerOrdered, func() {
		By("creating the custom resource for the Kind ZitiWebhook")
		err := k8sClient.Get(ctx, typeNamespacedName, zitiwebhook)
		if err != nil && errors.IsNotFound(err) {
			zitiwebhook = &kubernetesv1alpha1.ZitiWebhook{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
				Spec: kubernetesv1alpha1.ZitiWebhookSpec{
					ZitiControllerName: "ziticontroller-sample",
					Name:               resourceName,
				},
			}
			Expect(k8sClient.Create(ctx, zitiwebhook)).To(Succeed())

			controllerReconciler = &ZitiWebhookReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: fakeRecorder,
			}
		}
		DeferCleanup(func() {
			resource := &kubernetesv1alpha1.ZitiWebhook{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ZitiWebhook")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
	})

	AfterEach(OncePerOrdered, func() {
		resource := &kubernetesv1alpha1.ZitiWebhook{}
		err := k8sClient.Get(ctx, typeNamespacedName, resource)
		Expect(err).NotTo(HaveOccurred())

		By("Cleanup the specific resource instance ZitiWebhook")
		Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
	})

	Describe("ZitiWebhook Controller Creation with defined parameters", Ordered, func() {

		Context("Creating Resources using only defaults", func() {

			It("should successfully reconcile all resources", func() {

				By("Running the reconcile loop")
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())
				Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("Normal")))

				By("Verifying the created resources")
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-ca-issuer"}, issuer)).To(Succeed())
				Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("Created a new Issuer")))
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-admission-cert"}, certificate)).To(Succeed())
				Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("Created a new Certificate")))
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-service"}, service)).To(Succeed())
				Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("Created a new Service")))
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-service-account"}, serviceAccount)).To(Succeed())
				Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("Created a new ServiceAccount")))
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: resourceName + "-cluster-role"}, clusterRole)).To(Succeed())
				Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("Created a new ClusterRole")))
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: resourceName + "-cluster-role-binding"}, clusterRoleBinding)).To(Succeed())
				Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("Created a new ClusterRoleBinding")))
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: resourceName + "-mutating-webhook-configuration"}, mutatingWebhook)).To(Succeed())
				Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("Created a new MutatingWebhookConfiguration")))
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-deployment"}, deployment)).To(Succeed())
				Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("Created a new Deployment")))

				By("Verifying the Certificate specs")
				Expect(certificate.Spec.Duration.Duration).To(Equal(time.Duration(2160) * time.Hour))
				Expect(certificate.Spec.RenewBefore.Duration).To(Equal(time.Duration(360) * time.Hour))
				Expect(certificate.Spec.Subject.Organizations).To(Equal([]string{"NetFoundry"}))

				By("Verifying the Service Account specs")
				Expect(serviceAccount.ImagePullSecrets).To(BeEmpty())
				Expect(serviceAccount.AutomountServiceAccountToken).To(BeNil())
				Expect(serviceAccount.Secrets).To(BeEmpty())

				By("Verifying the ClusterRole specs")
				Expect(clusterRole.Rules).To(HaveLen(1))
				Expect(clusterRole.Rules[0].APIGroups).To(Equal([]string{"*"}))
				Expect(clusterRole.Rules[0].Resources).To(Equal([]string{"services", "namespaces"}))
				Expect(clusterRole.Rules[0].Verbs).To(Equal([]string{"get", "list", "watch"}))
				Expect(clusterRole.Rules[0].ResourceNames).To(BeEmpty())
				Expect(clusterRole.Rules[0].NonResourceURLs).To(BeEmpty())

				By("Verifying the MutatingWebhookConfiguration specs")
				Expect(mutatingWebhook.Webhooks).To(HaveLen(1))
				Expect(mutatingWebhook.Webhooks[0].ObjectSelector).To(Equal(&metav1.LabelSelector{}))
				Expect(mutatingWebhook.Webhooks[0].NamespaceSelector).To(Equal(&metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{Key: "kubernetes.io/metadata.name", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"kube-system"}},
						{Key: "tunnel.openziti.io/enabled", Operator: metav1.LabelSelectorOpIn, Values: []string{"true", "false"}},
					}}))
				Expect(mutatingWebhook.Webhooks[0].SideEffects).To(Equal(&[]admissionregistrationv1.SideEffectClass{admissionregistrationv1.SideEffectClassNone}[0]))
				Expect(mutatingWebhook.Webhooks[0].FailurePolicy).To(Equal(&[]admissionregistrationv1.FailurePolicyType{admissionregistrationv1.Fail}[0]))
				Expect(mutatingWebhook.Webhooks[0].TimeoutSeconds).To(Equal(&[]int32{30}[0]))
				Expect(mutatingWebhook.Webhooks[0].MatchPolicy).To(Equal(&[]admissionregistrationv1.MatchPolicyType{admissionregistrationv1.Equivalent}[0]))
				Expect(mutatingWebhook.Webhooks[0].ReinvocationPolicy).To(Equal(&[]admissionregistrationv1.ReinvocationPolicyType{admissionregistrationv1.NeverReinvocationPolicy}[0]))
				Expect(mutatingWebhook.Webhooks[0].AdmissionReviewVersions).To(Equal([]string{"v1"}))
				Expect(mutatingWebhook.Webhooks[0].Rules).To(HaveLen(1))
				Expect(mutatingWebhook.Webhooks[0].Rules[0].Operations).To(Equal([]admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update, admissionregistrationv1.Delete}))
				Expect(mutatingWebhook.Webhooks[0].Rules[0].Rule).To(Equal(admissionregistrationv1.Rule{APIGroups: []string{"*"}, APIVersions: []string{"v1", "v1beta1"}, Resources: []string{"pods"}, Scope: &[]admissionregistrationv1.ScopeType{admissionregistrationv1.AllScopes}[0]}))
				Expect(mutatingWebhook.Webhooks[0].ClientConfig.Service.Name).To(Equal(resourceName + "-service"))
				Expect(mutatingWebhook.Webhooks[0].ClientConfig.Service.Namespace).To(Equal(resourceNamespace))
				Expect(*mutatingWebhook.Webhooks[0].ClientConfig.Service.Path).To(Equal("/ziti-tunnel"))
				Expect(*mutatingWebhook.Webhooks[0].ClientConfig.Service.Port).To(Equal(int32(443)))
				Expect(mutatingWebhook.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte{}))

				By("Verifying the deployment specs")
				Expect(*deployment.Spec.Replicas).To(Equal(int32(2)))
				Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(Equal(resourceName))
				Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("netfoundry/ziti-k8s-agent:latest"))
				Expect(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
				Expect(deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(int32(9443)))
				Expect(deployment.Spec.Template.Spec.Containers[0].Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
				Expect(deployment.Spec.Template.Spec.Containers[0].Args[0]).To(Equal("webhook"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Args[1]).To(Equal("--v=2"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[0].Name).To(Equal("TLS_CERT"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[0].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[1].Name).To(Equal("TLS_PRIVATE_KEY"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[1].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[2].Name).To(Equal("ZITI_ADMIN_CERT"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[2].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[3].Name).To(Equal("ZITI_ADMIN_KEY"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[3].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[4].Name).To(Equal("ZITI_CTRL_CA_BUNDLE"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[4].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[5].Name).To(Equal("SIDECAR_IMAGE"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[5].Value).To(Equal("openziti/ziti-tunnel"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[6].Name).To(Equal("SIDECAR_IMAGE_VERSION"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[6].Value).To(Equal("latest"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[7].Name).To(Equal("SIDECAR_IMAGE_PULL_POLICY"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[7].Value).To(Equal("IfNotPresent"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[8].Name).To(Equal("SIDECAR_PREFIX"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[8].Value).To(Equal("zt"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[9].Name).To(Equal("SIDECAR_IDENTITY_DIR"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[9].Value).To(Equal("/ziti-tunnel"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[10].Name).To(Equal("ZITI_MGMT_API"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[10].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[11].Name).To(Equal("POD_SECURITY_CONTEXT_OVERRIDE"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[11].Value).To(Equal("false"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[12].Name).To(Equal("CLUSTER_DNS_SERVICE_IP"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[12].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[13].Name).To(Equal("SEARCH_DOMAIN_LIST"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[13].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[14].Name).To(Equal("ZITI_ROLE_KEY"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[14].Value).To(Equal("identity.openziti.io/role-attributes"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String()).To(Equal("100m"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().String()).To(Equal("128Mi"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().String()).To(Equal("500m"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().String()).To(Equal("512Mi"))
				Expect(deployment.Spec.Template.Spec.ServiceAccountName).To(Equal(resourceName + "-service-account"))
				Expect(deployment.Spec.Template.Spec.DeprecatedServiceAccount).To(Equal(resourceName + "-service-account"))
				Expect(deployment.Spec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyAlways))
				Expect(deployment.Spec.Template.Spec.DNSPolicy).To(Equal(corev1.DNSClusterFirst))
				Expect(deployment.Spec.Template.Spec.SecurityContext).To(Equal(&corev1.PodSecurityContext{}))
				Expect(deployment.Spec.Template.Spec.SchedulerName).To(Equal(corev1.DefaultSchedulerName))
				Expect(deployment.Spec.Template.Spec.TerminationGracePeriodSeconds).To(Equal(&[]int64{30}[0]))
				Expect(deployment.Spec.Strategy.Type).To(Equal(appsv1.RollingUpdateDeploymentStrategyType))
				Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.Type).To(Equal(intstr.String))
				Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.StrVal).To(Equal("25%"))
				Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.Type).To(Equal(intstr.String))
				Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.StrVal).To(Equal("25%"))
				Expect(deployment.Spec.ProgressDeadlineSeconds).To(Equal(&[]int32{600}[0]))
				Expect(deployment.Spec.RevisionHistoryLimit).To(Equal(&[]int32{10}[0]))
			})

		})

		Context("Creating Resources using all Spec fields", func() {

			It("should successfully reconcile all resources", func() {

				By("Updating the custom resource for the Kind ZitiWebhook")
				err := k8sClient.Get(ctx, typeNamespacedName, zitiwebhook)
				Expect(err).NotTo(HaveOccurred())
				zitiwebhook = &kubernetesv1alpha1.ZitiWebhook{
					ObjectMeta: metav1.ObjectMeta{
						Name:            resourceName,
						Namespace:       resourceNamespace,
						ResourceVersion: zitiwebhook.ResourceVersion,
					},
					Spec: kubernetesv1alpha1.ZitiWebhookSpec{
						ZitiControllerName: "ziticontroller-sample",
						Name:               resourceName,
						DeploymentSpec: kubernetesv1alpha1.DeploymentSpec{
							Env: kubernetesv1alpha1.DeploymentEnvVars{
								ZitiCtrlMgmtApi:     "https://localhost:443/edge/management/v1",
								SidecarImage:        "openziti/ziti-router",
								SidecarImageVersion: "1.4.2",
							},
						},
					},
				}
				Expect(k8sClient.Update(ctx, zitiwebhook)).To(Succeed())

				By("Running the reconcile loop again")
				_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())
				Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("Normal")))

				By("Verifying the created resources")
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-ca-issuer"}, issuer)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-admission-cert"}, certificate)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-service"}, service)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-service-account"}, serviceAccount)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: resourceName + "-cluster-role"}, clusterRole)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: resourceName + "-cluster-role-binding"}, clusterRoleBinding)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: resourceName + "-mutating-webhook-configuration"}, mutatingWebhook)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-deployment"}, deployment)).To(Succeed())
				Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("Updated Deployment")))

				By("Verifying the Certificate specs")
				Expect(certificate.Spec.Duration.Duration).To(Equal(time.Duration(2160) * time.Hour))
				Expect(certificate.Spec.RenewBefore.Duration).To(Equal(time.Duration(360) * time.Hour))
				Expect(certificate.Spec.Subject.Organizations).To(Equal([]string{"NetFoundry"}))

				By("Verifying the Service Account specs")
				Expect(serviceAccount.ImagePullSecrets).To(BeEmpty())
				Expect(serviceAccount.AutomountServiceAccountToken).To(BeNil())
				Expect(serviceAccount.Secrets).To(BeEmpty())

				By("Verifying the ClusterRole specs")
				Expect(clusterRole.Rules).To(HaveLen(1))
				Expect(clusterRole.Rules[0].APIGroups).To(Equal([]string{"*"}))
				Expect(clusterRole.Rules[0].Resources).To(Equal([]string{"services", "namespaces"}))
				Expect(clusterRole.Rules[0].Verbs).To(Equal([]string{"get", "list", "watch"}))
				Expect(clusterRole.Rules[0].ResourceNames).To(BeEmpty())
				Expect(clusterRole.Rules[0].NonResourceURLs).To(BeEmpty())

				By("Verifying the MutatingWebhookConfiguration specs")
				Expect(mutatingWebhook.Webhooks).To(HaveLen(1))
				Expect(mutatingWebhook.Webhooks[0].ObjectSelector).To(Equal(&metav1.LabelSelector{}))
				Expect(mutatingWebhook.Webhooks[0].NamespaceSelector).To(Equal(&metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{Key: "kubernetes.io/metadata.name", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"kube-system"}},
						{Key: "tunnel.openziti.io/enabled", Operator: metav1.LabelSelectorOpIn, Values: []string{"true", "false"}},
					}}))
				Expect(mutatingWebhook.Webhooks[0].SideEffects).To(Equal(&[]admissionregistrationv1.SideEffectClass{admissionregistrationv1.SideEffectClassNone}[0]))
				Expect(mutatingWebhook.Webhooks[0].FailurePolicy).To(Equal(&[]admissionregistrationv1.FailurePolicyType{admissionregistrationv1.Fail}[0]))
				Expect(mutatingWebhook.Webhooks[0].TimeoutSeconds).To(Equal(&[]int32{30}[0]))
				Expect(mutatingWebhook.Webhooks[0].MatchPolicy).To(Equal(&[]admissionregistrationv1.MatchPolicyType{admissionregistrationv1.Equivalent}[0]))
				Expect(mutatingWebhook.Webhooks[0].ReinvocationPolicy).To(Equal(&[]admissionregistrationv1.ReinvocationPolicyType{admissionregistrationv1.NeverReinvocationPolicy}[0]))
				Expect(mutatingWebhook.Webhooks[0].AdmissionReviewVersions).To(Equal([]string{"v1"}))
				Expect(mutatingWebhook.Webhooks[0].Rules).To(HaveLen(1))
				Expect(mutatingWebhook.Webhooks[0].Rules[0].Operations).To(Equal([]admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update, admissionregistrationv1.Delete}))
				Expect(mutatingWebhook.Webhooks[0].Rules[0].Rule).To(Equal(admissionregistrationv1.Rule{APIGroups: []string{"*"}, APIVersions: []string{"v1", "v1beta1"}, Resources: []string{"pods"}, Scope: &[]admissionregistrationv1.ScopeType{admissionregistrationv1.AllScopes}[0]}))
				Expect(mutatingWebhook.Webhooks[0].ClientConfig.Service.Name).To(Equal(resourceName + "-service"))
				Expect(mutatingWebhook.Webhooks[0].ClientConfig.Service.Namespace).To(Equal(resourceNamespace))
				Expect(*mutatingWebhook.Webhooks[0].ClientConfig.Service.Path).To(Equal("/ziti-tunnel"))
				Expect(*mutatingWebhook.Webhooks[0].ClientConfig.Service.Port).To(Equal(int32(443)))
				Expect(mutatingWebhook.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte{}))

				By("Verifying the deployment specs")
				Expect(*deployment.Spec.Replicas).To(Equal(int32(2)))
				Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(Equal(resourceName))
				Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("netfoundry/ziti-k8s-agent:latest"))
				Expect(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
				Expect(deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(int32(9443)))
				Expect(deployment.Spec.Template.Spec.Containers[0].Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
				Expect(deployment.Spec.Template.Spec.Containers[0].Args[0]).To(Equal("webhook"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Args[1]).To(Equal("--v=2"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[0].Name).To(Equal("TLS_CERT"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[0].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[1].Name).To(Equal("TLS_PRIVATE_KEY"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[1].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[2].Name).To(Equal("ZITI_ADMIN_CERT"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[2].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[3].Name).To(Equal("ZITI_ADMIN_KEY"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[3].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[4].Name).To(Equal("ZITI_CTRL_CA_BUNDLE"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[4].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[5].Name).To(Equal("SIDECAR_IMAGE"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[5].Value).To(Equal("openziti/ziti-router"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[6].Name).To(Equal("SIDECAR_IMAGE_VERSION"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[6].Value).To(Equal("1.4.2"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[7].Name).To(Equal("SIDECAR_IMAGE_PULL_POLICY"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[7].Value).To(Equal("IfNotPresent"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[8].Name).To(Equal("SIDECAR_PREFIX"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[8].Value).To(Equal("zt"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[9].Name).To(Equal("SIDECAR_IDENTITY_DIR"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[9].Value).To(Equal("/ziti-tunnel"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[10].Name).To(Equal("ZITI_MGMT_API"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[10].Value).To(Equal("https://localhost:443/edge/management/v1"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[11].Name).To(Equal("POD_SECURITY_CONTEXT_OVERRIDE"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[11].Value).To(Equal("false"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[12].Name).To(Equal("CLUSTER_DNS_SERVICE_IP"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[12].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[13].Name).To(Equal("SEARCH_DOMAIN_LIST"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[13].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[14].Name).To(Equal("ZITI_ROLE_KEY"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[14].Value).To(Equal("identity.openziti.io/role-attributes"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String()).To(Equal("100m"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().String()).To(Equal("128Mi"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().String()).To(Equal("500m"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().String()).To(Equal("512Mi"))
				Expect(deployment.Spec.Template.Spec.ServiceAccountName).To(Equal(resourceName + "-service-account"))
				Expect(deployment.Spec.Template.Spec.DeprecatedServiceAccount).To(Equal(resourceName + "-service-account"))
				Expect(deployment.Spec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyAlways))
				Expect(deployment.Spec.Template.Spec.DNSPolicy).To(Equal(corev1.DNSClusterFirst))
				Expect(deployment.Spec.Template.Spec.SecurityContext).To(Equal(&corev1.PodSecurityContext{}))
				Expect(deployment.Spec.Template.Spec.SchedulerName).To(Equal(corev1.DefaultSchedulerName))
				Expect(deployment.Spec.Template.Spec.TerminationGracePeriodSeconds).To(Equal(&[]int64{30}[0]))
				Expect(deployment.Spec.Strategy.Type).To(Equal(appsv1.RollingUpdateDeploymentStrategyType))
				Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.Type).To(Equal(intstr.String))
				Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.StrVal).To(Equal("25%"))
				Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.Type).To(Equal(intstr.String))
				Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.StrVal).To(Equal("25%"))
				Expect(deployment.Spec.ProgressDeadlineSeconds).To(Equal(&[]int32{600}[0]))
				Expect(deployment.Spec.RevisionHistoryLimit).To(Equal(&[]int32{10}[0]))
			})

		})

		Context("Deleting Resources", func() {

			AfterEach(OncePerOrdered, func() {
				By("Verifying the Certificate specs")
				Expect(certificate.Spec.Duration.Duration).To(Equal(time.Duration(2160) * time.Hour))
				Expect(certificate.Spec.RenewBefore.Duration).To(Equal(time.Duration(360) * time.Hour))
				Expect(certificate.Spec.Subject.Organizations).To(Equal([]string{"NetFoundry"}))

				By("Verifying the Service Account specs")
				Expect(serviceAccount.ImagePullSecrets).To(BeEmpty())
				Expect(serviceAccount.AutomountServiceAccountToken).To(BeNil())
				Expect(serviceAccount.Secrets).To(BeEmpty())

				By("Verifying the ClusterRole specs")
				Expect(clusterRole.Rules).To(HaveLen(1))
				Expect(clusterRole.Rules[0].APIGroups).To(Equal([]string{"*"}))
				Expect(clusterRole.Rules[0].Resources).To(Equal([]string{"services", "namespaces"}))
				Expect(clusterRole.Rules[0].Verbs).To(Equal([]string{"get", "list", "watch"}))
				Expect(clusterRole.Rules[0].ResourceNames).To(BeEmpty())
				Expect(clusterRole.Rules[0].NonResourceURLs).To(BeEmpty())

				By("Verifying the MutatingWebhookConfiguration specs")
				Expect(mutatingWebhook.Webhooks).To(HaveLen(1))
				Expect(mutatingWebhook.Webhooks[0].ObjectSelector).To(Equal(&metav1.LabelSelector{}))
				Expect(mutatingWebhook.Webhooks[0].NamespaceSelector).To(Equal(&metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{Key: "kubernetes.io/metadata.name", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"kube-system"}},
						{Key: "tunnel.openziti.io/enabled", Operator: metav1.LabelSelectorOpIn, Values: []string{"true", "false"}},
					}}))
				Expect(mutatingWebhook.Webhooks[0].SideEffects).To(Equal(&[]admissionregistrationv1.SideEffectClass{admissionregistrationv1.SideEffectClassNone}[0]))
				Expect(mutatingWebhook.Webhooks[0].FailurePolicy).To(Equal(&[]admissionregistrationv1.FailurePolicyType{admissionregistrationv1.Fail}[0]))
				Expect(mutatingWebhook.Webhooks[0].TimeoutSeconds).To(Equal(&[]int32{30}[0]))
				Expect(mutatingWebhook.Webhooks[0].MatchPolicy).To(Equal(&[]admissionregistrationv1.MatchPolicyType{admissionregistrationv1.Equivalent}[0]))
				Expect(mutatingWebhook.Webhooks[0].ReinvocationPolicy).To(Equal(&[]admissionregistrationv1.ReinvocationPolicyType{admissionregistrationv1.NeverReinvocationPolicy}[0]))
				Expect(mutatingWebhook.Webhooks[0].AdmissionReviewVersions).To(Equal([]string{"v1"}))
				Expect(mutatingWebhook.Webhooks[0].Rules).To(HaveLen(1))
				Expect(mutatingWebhook.Webhooks[0].Rules[0].Operations).To(Equal([]admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update, admissionregistrationv1.Delete}))
				Expect(mutatingWebhook.Webhooks[0].Rules[0].Rule).To(Equal(admissionregistrationv1.Rule{APIGroups: []string{"*"}, APIVersions: []string{"v1", "v1beta1"}, Resources: []string{"pods"}, Scope: &[]admissionregistrationv1.ScopeType{admissionregistrationv1.AllScopes}[0]}))
				Expect(mutatingWebhook.Webhooks[0].ClientConfig.Service.Name).To(Equal(resourceName + "-service"))
				Expect(mutatingWebhook.Webhooks[0].ClientConfig.Service.Namespace).To(Equal(resourceNamespace))
				Expect(*mutatingWebhook.Webhooks[0].ClientConfig.Service.Path).To(Equal("/ziti-tunnel"))
				Expect(*mutatingWebhook.Webhooks[0].ClientConfig.Service.Port).To(Equal(int32(443)))
				Expect(mutatingWebhook.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte{}))

				By("Verifying the deployment specs")
				Expect(*deployment.Spec.Replicas).To(Equal(int32(2)))
				Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(Equal(resourceName))
				Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("netfoundry/ziti-k8s-agent:latest"))
				Expect(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
				Expect(deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(int32(9443)))
				Expect(deployment.Spec.Template.Spec.Containers[0].Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
				Expect(deployment.Spec.Template.Spec.Containers[0].Args[0]).To(Equal("webhook"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Args[1]).To(Equal("--v=2"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[0].Name).To(Equal("TLS_CERT"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[0].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[1].Name).To(Equal("TLS_PRIVATE_KEY"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[1].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[2].Name).To(Equal("ZITI_ADMIN_CERT"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[2].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[3].Name).To(Equal("ZITI_ADMIN_KEY"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[3].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[4].Name).To(Equal("ZITI_CTRL_CA_BUNDLE"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[4].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[5].Name).To(Equal("SIDECAR_IMAGE"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[5].Value).To(Equal("openziti/ziti-router"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[6].Name).To(Equal("SIDECAR_IMAGE_VERSION"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[6].Value).To(Equal("1.4.2"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[7].Name).To(Equal("SIDECAR_IMAGE_PULL_POLICY"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[7].Value).To(Equal("IfNotPresent"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[8].Name).To(Equal("SIDECAR_PREFIX"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[8].Value).To(Equal("zt"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[9].Name).To(Equal("SIDECAR_IDENTITY_DIR"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[9].Value).To(Equal("/ziti-tunnel"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[10].Name).To(Equal("ZITI_MGMT_API"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[10].Value).To(Equal("https://localhost:443/edge/management/v1"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[11].Name).To(Equal("POD_SECURITY_CONTEXT_OVERRIDE"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[11].Value).To(Equal("false"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[12].Name).To(Equal("CLUSTER_DNS_SERVICE_IP"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[12].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[13].Name).To(Equal("SEARCH_DOMAIN_LIST"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[13].Value).To(Equal(""))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[14].Name).To(Equal("ZITI_ROLE_KEY"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env[14].Value).To(Equal("identity.openziti.io/role-attributes"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String()).To(Equal("100m"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().String()).To(Equal("128Mi"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().String()).To(Equal("500m"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().String()).To(Equal("512Mi"))
				Expect(deployment.Spec.Template.Spec.ServiceAccountName).To(Equal(resourceName + "-service-account"))
				Expect(deployment.Spec.Template.Spec.DeprecatedServiceAccount).To(Equal(resourceName + "-service-account"))
				Expect(deployment.Spec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyAlways))
				Expect(deployment.Spec.Template.Spec.DNSPolicy).To(Equal(corev1.DNSClusterFirst))
				Expect(deployment.Spec.Template.Spec.SecurityContext).To(Equal(&corev1.PodSecurityContext{}))
				Expect(deployment.Spec.Template.Spec.SchedulerName).To(Equal(corev1.DefaultSchedulerName))
				Expect(deployment.Spec.Template.Spec.TerminationGracePeriodSeconds).To(Equal(&[]int64{30}[0]))
				Expect(deployment.Spec.Strategy.Type).To(Equal(appsv1.RollingUpdateDeploymentStrategyType))
				Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.Type).To(Equal(intstr.String))
				Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.StrVal).To(Equal("25%"))
				Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.Type).To(Equal(intstr.String))
				Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.StrVal).To(Equal("25%"))
				Expect(deployment.Spec.ProgressDeadlineSeconds).To(Equal(&[]int32{600}[0]))
				Expect(deployment.Spec.RevisionHistoryLimit).To(Equal(&[]int32{10}[0]))
			})

			It("should successfully recreate the deleted resources", func() {

				By("Deleting resources to be reconciled and verifying if they are deleted")
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-ca-issuer"}, issuer)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-admission-cert"}, certificate)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-service"}, service)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-service-account"}, serviceAccount)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: resourceName + "-cluster-role"}, clusterRole)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: resourceName + "-cluster-role-binding"}, clusterRoleBinding)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: resourceName + "-mutating-webhook-configuration"}, mutatingWebhook)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-deployment"}, deployment)).To(Succeed())

				Expect(k8sClient.Delete(ctx, issuer)).To(Succeed())
				Expect(k8sClient.Delete(ctx, certificate)).To(Succeed())
				Expect(k8sClient.Delete(ctx, service)).To(Succeed())
				Expect(k8sClient.Delete(ctx, serviceAccount)).To(Succeed())
				Expect(k8sClient.Delete(ctx, clusterRole)).To(Succeed())
				Expect(k8sClient.Delete(ctx, clusterRoleBinding)).To(Succeed())
				Expect(k8sClient.Delete(ctx, mutatingWebhook)).To(Succeed())
				Expect(k8sClient.Delete(ctx, deployment)).To(Succeed())

				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-ca-issuer"}, issuer)).NotTo(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-admission-cert"}, certificate)).NotTo(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-service"}, service)).NotTo(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-service-account"}, serviceAccount)).NotTo(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: resourceName + "-cluster-role"}, clusterRole)).NotTo(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: resourceName + "-cluster-role-binding"}, clusterRoleBinding)).NotTo(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: resourceName + "-mutating-webhook-configuration"}, mutatingWebhook)).NotTo(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-deployment"}, deployment)).NotTo(Succeed())

				By("Running the reconsile loop again")
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())
				Eventually(fakeRecorder.Events).Should(Receive(ContainSubstring("Normal")))

				By("Checking if resources are reconciled successfully")
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-ca-issuer"}, issuer)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-admission-cert"}, certificate)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-service"}, service)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-service-account"}, serviceAccount)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: resourceName + "-cluster-role"}, clusterRole)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: resourceName + "-cluster-role-binding"}, clusterRoleBinding)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: resourceName + "-mutating-webhook-configuration"}, mutatingWebhook)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: resourceNamespace, Name: resourceName + "-deployment"}, deployment)).To(Succeed())
			})
		})
	})
})
