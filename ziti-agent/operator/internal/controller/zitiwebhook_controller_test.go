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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kubernetesv1alpha1 "github.com/netfoundry/ziti-k8s-agent/ziti-agent/operator/api/v1alpha1"
)

var _ = Describe("ZitiWebhook Controller", func() {

	const zitiwebhookName = "test-zitiwebhook"
	const zitiwebhookNamespace = "default"
	// Define constants for Eventually timings
	const timeout = time.Second * 10
	const interval = time.Millisecond * 250

	ctx := context.Background()

	typeNamespacedName := types.NamespacedName{
		Name:      zitiwebhookName,
		Namespace: zitiwebhookNamespace,
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
	ownerRef := metav1.OwnerReference{}

	BeforeEach(OncePerOrdered, func() {

		// Initialize the reconciler within the BeforeEach where the client is ready
		controllerReconciler = &ZitiWebhookReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Recorder: fakeRecorder,
		}

		ownerRef = metav1.OwnerReference{
			APIVersion:         kubernetesv1alpha1.GroupVersion.String(),
			Kind:               "ZitiWebhook",
			Name:               zitiwebhookName,
			UID:                zitiwebhook.UID, // Ensure zitiwebhook has UID after creation/get
			Controller:         &[]bool{true}[0],
			BlockOwnerDeletion: &[]bool{true}[0],
		}

		By("ensuring resources from previous tests are cleaned up")
		err := k8sClient.Get(ctx, typeNamespacedName, zitiwebhook)
		if err == nil {
			By("Deleting the existing ZitiWebhook resource")
			Expect(k8sClient.Delete(ctx, zitiwebhook)).To(Succeed())

			By("Cleaning up the resources created by the previous test using reconcile loop")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Cleaning up the cluster resources manually")
			_ = k8sClient.Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: zitiwebhookName + "-cluster-role"}})
			_ = k8sClient.Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: zitiwebhookName + "-cluster-role-binding"}})
			_ = k8sClient.Delete(ctx, &admissionregistrationv1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: zitiwebhookName + "-mutating-webhook-configuration"}})
		}

		By("creating the custom resource for the Kind ZitiWebhook")
		zitiwebhook = &kubernetesv1alpha1.ZitiWebhook{
			ObjectMeta: metav1.ObjectMeta{
				Name:      typeNamespacedName.Name,
				Namespace: typeNamespacedName.Namespace,
			},
			Spec: kubernetesv1alpha1.ZitiWebhookSpec{
				ZitiControllerName: "ziticontroller-sample",
				Name:               typeNamespacedName.Name,
			},
		}

		By("Creating the ZitiWebhook resource")
		Expect(k8sClient.Create(ctx, zitiwebhook)).To(Succeed())

		// Add finalizer manually for testing deletion if needed
		// Use Eventually to handle potential conflicts on initial creation/update
		Eventually(func() error {
			// Re-fetch the resource before updating
			err := k8sClient.Get(ctx, typeNamespacedName, zitiwebhook)
			if err != nil {
				return err
			}
			if controllerutil.AddFinalizer(zitiwebhook, zitiWebhookFinalizer) {
				return k8sClient.Update(ctx, zitiwebhook)
			}
			// Finalizer already present, no update needed
			return nil
		}, timeout, interval).Should(Succeed(), "Should succeed in adding the finalizer")

		// Drain events before each test to prevent interference
		Eventually(fakeRecorder.Events).ShouldNot(Receive())
	})

	AfterEach(OncePerOrdered, func() {
		By("Cleanup ZitiWebhook if exists")
		err := k8sClient.Get(ctx, typeNamespacedName, zitiwebhook)
		if err == nil {
			Expect(k8sClient.Delete(ctx, zitiwebhook)).To(Succeed())
			// Wait for deletion to complete, including finalizer processing
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, zitiwebhook)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue()) // Increase timeout for finalizer
		} else if !errors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred())
		}

		By("Drainging events after each test")
		Eventually(fakeRecorder.Events).ShouldNot(Receive())
	})

	Describe("ZitiWebhook Controller Creation with defined parameters", Ordered, func() {

		Context("Creating Resources using only defaults", func() {

			It("should successfully reconcile all resources", func() {

				By("Running the reconcile loop")
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())

				ownerRef.UID = zitiwebhook.UID // Ensure ownerRef has the correct UID after creation

				By("Verifying the created resources and events")
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Created a new Issuer")))
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-ca-issuer"}, issuer)).To(Succeed())
				Expect(issuer.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))

				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Created a new Certificate")))
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-admission-cert"}, certificate)).To(Succeed())
				Expect(certificate.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))

				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Created a new Service")))
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-service"}, service)).To(Succeed())
				Expect(service.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))

				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Created a new ServiceAccount")))
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-service-account"}, serviceAccount)).To(Succeed())
				Expect(serviceAccount.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))

				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Created a new ClusterRole")))
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-cluster-role"}, clusterRole)).To(Succeed())

				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Created a new ClusterRoleBinding")))
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-cluster-role-binding"}, clusterRoleBinding)).To(Succeed())

				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Created a new MutatingWebhookConfiguration")))
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-mutating-webhook-configuration"}, mutatingWebhook)).To(Succeed())

				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Created a new Deployment")))
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-deployment"}, deployment)).To(Succeed())
				Expect(deployment.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))

				By("Verifying the Certificate specs")
				Expect(certificate.Spec.Duration.Duration).To(Equal(time.Duration(2160) * time.Hour))
				Expect(certificate.Spec.RenewBefore.Duration).To(Equal(time.Duration(360) * time.Hour))
				Expect(certificate.Spec.Subject.Organizations).To(Equal([]string{"NetFoundry"}))
				Expect(certificate.ObjectMeta.Labels).To(HaveKeyWithValue("app", zitiwebhook.Spec.Name))
				Expect(certificate.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", zitiwebhook.Spec.Name+"-"+zitiwebhook.Namespace))
				Expect(certificate.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", zitiwebhook.Spec.Name+"-operator"))
				Expect(certificate.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", zitiwebhook.Spec.Name+"-controller"))
				Expect(certificate.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/component", "webhook"))

				By("Verifying the Service Account specs")
				Expect(serviceAccount.ImagePullSecrets).To(BeEmpty())
				Expect(serviceAccount.AutomountServiceAccountToken).To(BeNil())
				Expect(serviceAccount.Secrets).To(BeEmpty())
				Expect(serviceAccount.ObjectMeta.Labels).To(HaveKeyWithValue("app", zitiwebhook.Spec.Name))
				Expect(serviceAccount.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", zitiwebhook.Spec.Name+"-"+zitiwebhook.Namespace))
				Expect(serviceAccount.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", zitiwebhook.Spec.Name+"-operator"))
				Expect(serviceAccount.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", zitiwebhook.Spec.Name+"-controller"))
				Expect(serviceAccount.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/component", "webhook"))

				By("Verifying the ClusterRole specs")
				Expect(clusterRole.Rules).To(HaveLen(2))
				Expect(clusterRole.Rules[0].APIGroups).To(Equal([]string{""}))
				Expect(clusterRole.Rules[0].Resources).To(Equal([]string{"services", "namespaces"}))
				Expect(clusterRole.Rules[0].Verbs).To(Equal([]string{"get", "list", "watch"}))
				Expect(clusterRole.Rules[0].ResourceNames).To(BeEmpty())
				Expect(clusterRole.Rules[0].NonResourceURLs).To(BeEmpty())
				Expect(clusterRole.Rules[1].APIGroups).To(Equal([]string{""}))
				Expect(clusterRole.Rules[1].Resources).To(Equal([]string{"persistentvolumeclaims"}))
				Expect(clusterRole.Rules[1].Verbs).To(Equal([]string{"get", "delete"}))
				Expect(clusterRole.Rules[1].ResourceNames).To(BeEmpty())
				Expect(clusterRole.Rules[1].NonResourceURLs).To(BeEmpty())
				Expect(clusterRole.ObjectMeta.Labels).To(HaveKeyWithValue("app", zitiwebhook.Spec.Name))
				Expect(clusterRole.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", zitiwebhook.Spec.Name+"-"+zitiwebhook.Namespace))
				Expect(clusterRole.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", zitiwebhook.Spec.Name+"-operator"))
				Expect(clusterRole.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", zitiwebhook.Spec.Name+"-controller"))
				Expect(clusterRole.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/component", "webhook"))

				By("Verifying the MutatingWebhookConfiguration specs")
				Expect(mutatingWebhook.Webhooks).To(HaveLen(2))
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
				Expect(mutatingWebhook.Webhooks[0].Rules[0].Rule.APIGroups).To(Equal([]string{""}))
				Expect(mutatingWebhook.Webhooks[0].Rules[0].Rule.APIVersions).To(Equal([]string{"v1", "v1beta1"}))
				Expect(mutatingWebhook.Webhooks[0].Rules[0].Rule.Resources).To(Equal([]string{"pods"}))
				Expect(*mutatingWebhook.Webhooks[0].Rules[0].Rule.Scope).To(Equal(admissionregistrationv1.AllScopes))
				Expect(mutatingWebhook.Webhooks[0].ClientConfig.Service.Name).To(Equal(zitiwebhookName + "-service"))
				Expect(mutatingWebhook.Webhooks[0].ClientConfig.Service.Namespace).To(Equal(zitiwebhookNamespace))
				Expect(*mutatingWebhook.Webhooks[0].ClientConfig.Service.Path).To(Equal("/ziti-tunnel"))
				Expect(*mutatingWebhook.Webhooks[0].ClientConfig.Service.Port).To(Equal(int32(9443)))
				Expect(mutatingWebhook.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte{}))
				Expect(mutatingWebhook.Webhooks[1].ObjectSelector).To(Equal(&metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{Key: "router.openziti.io/enabled", Operator: metav1.LabelSelectorOpIn, Values: []string{"true", "false"}},
					}}))
				Expect(mutatingWebhook.Webhooks[1].NamespaceSelector).To(Equal(&metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{Key: "kubernetes.io/metadata.name", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"kube-system"}},
					}}))
				Expect(mutatingWebhook.Webhooks[1].SideEffects).To(Equal(&[]admissionregistrationv1.SideEffectClass{admissionregistrationv1.SideEffectClassNone}[0]))
				Expect(mutatingWebhook.Webhooks[1].FailurePolicy).To(Equal(&[]admissionregistrationv1.FailurePolicyType{admissionregistrationv1.Fail}[0]))
				Expect(mutatingWebhook.Webhooks[1].TimeoutSeconds).To(Equal(&[]int32{30}[0]))
				Expect(mutatingWebhook.Webhooks[1].MatchPolicy).To(Equal(&[]admissionregistrationv1.MatchPolicyType{admissionregistrationv1.Equivalent}[0]))
				Expect(mutatingWebhook.Webhooks[1].ReinvocationPolicy).To(Equal(&[]admissionregistrationv1.ReinvocationPolicyType{admissionregistrationv1.NeverReinvocationPolicy}[0]))
				Expect(mutatingWebhook.Webhooks[1].AdmissionReviewVersions).To(Equal([]string{"v1"}))
				Expect(mutatingWebhook.Webhooks[1].Rules).To(HaveLen(1))
				Expect(mutatingWebhook.Webhooks[1].Rules[0].Operations).To(Equal([]admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update, admissionregistrationv1.Delete}))
				Expect(mutatingWebhook.Webhooks[1].Rules[0].Rule.APIGroups).To(Equal([]string{""}))
				Expect(mutatingWebhook.Webhooks[1].Rules[0].Rule.APIVersions).To(Equal([]string{"v1", "v1beta1"}))
				Expect(mutatingWebhook.Webhooks[1].Rules[0].Rule.Resources).To(Equal([]string{"pods"}))
				Expect(*mutatingWebhook.Webhooks[1].Rules[0].Rule.Scope).To(Equal(admissionregistrationv1.AllScopes))
				Expect(mutatingWebhook.Webhooks[1].ClientConfig.Service.Name).To(Equal(zitiwebhookName + "-service"))
				Expect(mutatingWebhook.Webhooks[1].ClientConfig.Service.Namespace).To(Equal(zitiwebhookNamespace))
				Expect(*mutatingWebhook.Webhooks[1].ClientConfig.Service.Path).To(Equal("/ziti-router"))
				Expect(*mutatingWebhook.Webhooks[1].ClientConfig.Service.Port).To(Equal(int32(9443)))
				Expect(mutatingWebhook.Webhooks[1].ClientConfig.CABundle).To(Equal([]byte{}))
				Expect(mutatingWebhook.ObjectMeta.Labels).To(HaveKeyWithValue("app", zitiwebhook.Spec.Name))
				Expect(mutatingWebhook.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", zitiwebhook.Spec.Name+"-"+zitiwebhook.Namespace))
				Expect(mutatingWebhook.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", zitiwebhook.Spec.Name+"-operator"))
				Expect(mutatingWebhook.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", zitiwebhook.Spec.Name+"-controller"))
				Expect(mutatingWebhook.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/component", "webhook"))

				By("Verifying the deployment specs")
				Expect(*deployment.Spec.Replicas).To(Equal(int32(2)))
				Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
				container := deployment.Spec.Template.Spec.Containers[0]
				Expect(container.Name).To(Equal(zitiwebhookName))
				Expect(container.Image).To(Equal("netfoundry/ziti-k8s-agent:latest"))
				Expect(container.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
				Expect(container.Ports).To(HaveLen(1))
				Expect(container.Ports[0].ContainerPort).To(Equal(int32(9443)))
				Expect(container.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
				Expect(container.Args).To(Equal([]string{"webhook", "--v=2"}))
				Expect(container.Env).To(HaveLen(15))
				Expect(container.Env[0]).To(Equal(corev1.EnvVar{
					Name:  "TLS_CERT",
					Value: "",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef:         nil,
						ResourceFieldRef: nil,
						ConfigMapKeyRef:  nil,
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-zitiwebhook-server-cert",
							},
							Key:      "tls.crt",
							Optional: nil,
						},
					},
				}))
				Expect(container.Env[1]).To(Equal(corev1.EnvVar{
					Name:  "TLS_PRIVATE_KEY",
					Value: "",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef:         nil,
						ResourceFieldRef: nil,
						ConfigMapKeyRef:  nil,
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-zitiwebhook-server-cert",
							},
							Key:      "tls.key",
							Optional: nil,
						},
					},
				}))
				Expect(container.Env[2]).To(Equal(corev1.EnvVar{
					Name:  "ZITI_ADMIN_CERT",
					Value: "",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef:         nil,
						ResourceFieldRef: nil,
						ConfigMapKeyRef:  nil,
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "ziticontroller-sample-secret",
							},
							Key:      "tls.crt",
							Optional: nil,
						},
					},
				}))
				Expect(container.Env[3]).To(Equal(corev1.EnvVar{
					Name:  "ZITI_ADMIN_KEY",
					Value: "",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef:         nil,
						ResourceFieldRef: nil,
						ConfigMapKeyRef:  nil,
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "ziticontroller-sample-secret",
							},
							Key:      "tls.key",
							Optional: nil,
						},
					},
				}))
				Expect(container.Env[4]).To(Equal(corev1.EnvVar{
					Name:  "ZITI_CTRL_CA_BUNDLE",
					Value: "",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef:         nil,
						ResourceFieldRef: nil,
						ConfigMapKeyRef:  nil,
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "ziticontroller-sample-secret",
							},
							Key:      "tls.ca",
							Optional: nil,
						},
					},
				}))
				Expect(container.Env[5]).To(Equal(corev1.EnvVar{
					Name:      "SIDECAR_IMAGE",
					Value:     "openziti/ziti-tunnel",
					ValueFrom: nil,
				}))
				Expect(container.Env[6]).To(Equal(corev1.EnvVar{
					Name:      "SIDECAR_IMAGE_VERSION",
					Value:     "latest",
					ValueFrom: nil,
				}))
				Expect(container.Env[7]).To(Equal(corev1.EnvVar{
					Name:      "SIDECAR_IMAGE_PULL_POLICY",
					Value:     "IfNotPresent",
					ValueFrom: nil,
				}))
				Expect(container.Env[8]).To(Equal(corev1.EnvVar{
					Name:      "SIDECAR_PREFIX",
					Value:     "zt",
					ValueFrom: nil,
				}))
				Expect(container.Env[9]).To(Equal(corev1.EnvVar{
					Name:      "SIDECAR_IDENTITY_DIR",
					Value:     "/ziti-tunnel",
					ValueFrom: nil,
				}))
				Expect(container.Env[10]).To(Equal(corev1.EnvVar{
					Name:      "ZITI_MGMT_API",
					Value:     "",
					ValueFrom: nil,
				}))
				Expect(container.Env[11]).To(Equal(corev1.EnvVar{
					Name:      "POD_SECURITY_CONTEXT_OVERRIDE",
					Value:     "false",
					ValueFrom: nil,
				}))
				Expect(container.Env[12]).To(Equal(corev1.EnvVar{
					Name:      "CLUSTER_DNS_SERVICE_IP",
					Value:     "",
					ValueFrom: nil,
				}))
				Expect(container.Env[13]).To(Equal(corev1.EnvVar{
					Name:      "SEARCH_DOMAIN_LIST",
					Value:     "",
					ValueFrom: nil,
				}))
				Expect(container.Env[14]).To(Equal(corev1.EnvVar{
					Name:      "ZITI_ROLE_KEY",
					Value:     "identity.openziti.io/role-attributes",
					ValueFrom: nil,
				}))
				Expect(container.Resources.Requests.Cpu().String()).To(Equal("100m"))
				Expect(container.Resources.Requests.Memory().String()).To(Equal("128Mi"))
				Expect(container.Resources.Limits.Cpu().String()).To(Equal("500m"))
				Expect(container.Resources.Limits.Memory().String()).To(Equal("512Mi"))
				Expect(deployment.Spec.Template.Spec.ServiceAccountName).To(Equal(zitiwebhookName + "-service-account"))
				Expect(deployment.Spec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyAlways))
				Expect(deployment.Spec.Template.Spec.DNSPolicy).To(Equal(corev1.DNSClusterFirst))
				Expect(deployment.Spec.Template.Spec.SecurityContext).To(Equal(&corev1.PodSecurityContext{}))
				Expect(deployment.Spec.Template.Spec.SchedulerName).To(Equal(corev1.DefaultSchedulerName))
				Expect(deployment.Spec.Template.Spec.TerminationGracePeriodSeconds).To(Equal(&[]int64{30}[0]))
				Expect(deployment.Spec.Strategy.Type).To(Equal(appsv1.RollingUpdateDeploymentStrategyType))
				Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.Type).To(Equal(intstr.String))
				Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.StrVal).To(Equal("25%"))
				Expect(deployment.Spec.ProgressDeadlineSeconds).To(Equal(&[]int32{600}[0]))
				Expect(deployment.Spec.RevisionHistoryLimit).To(Equal(&[]int32{10}[0]))
				Expect(deployment.ObjectMeta.Labels).To(HaveKeyWithValue("app", zitiwebhook.Spec.Name))
				Expect(deployment.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", zitiwebhook.Spec.Name+"-"+zitiwebhook.Namespace))
				Expect(deployment.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", zitiwebhook.Spec.Name+"-operator"))
				Expect(deployment.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", zitiwebhook.Spec.Name+"-controller"))
				Expect(deployment.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/component", "webhook"))

			})

			It("should be idempotent", func() {
				// First reconcile already happened in the previous test or BeforeEach
				By("Ensuring resources exist after first reconcile")
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-deployment"}, deployment)).To(Succeed())
				// Add checks for other critical resources if needed

				// Drain events from the first reconcile
				Eventually(fakeRecorder.Events).ShouldNot(Receive())

				By("Running the reconcile loop again")
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying no 'Created' or 'Updated' events were generated")
				// Use Consistently to ensure no events appear for a short duration
				Consistently(fakeRecorder.Events, time.Second*2).ShouldNot(Receive(ContainSubstring("Created")))
				Consistently(fakeRecorder.Events, time.Second*2).ShouldNot(Receive(ContainSubstring("Updated"))) // Allow "Merged" event if merge logic runs

				By("Verifying resources remain unchanged")
				currentDeployment := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-deployment"}, currentDeployment)).To(Succeed())
				// Compare relevant fields. Generation/ResourceVersion might change, but spec should be the same.
				Expect(currentDeployment.Spec).To(Equal(deployment.Spec))
				// Add comparisons for other resources if necessary
			})

		})

		Context("Creating Resources using all custom spec fields", func() {

			It("should successfully reconcile all resources with updated specs", func() {

				By("Fetching the latest version of the ZitiWebhook resource")
				Expect(k8sClient.Get(ctx, typeNamespacedName, zitiwebhook)).To(Succeed())

				By("Defining the updated spec")
				sideEffectClass := admissionregistrationv1.SideEffectClassNoneOnDryRun
				failurePolicy := admissionregistrationv1.Ignore
				matchPolicy := admissionregistrationv1.Exact
				reinvocationPolicy := admissionregistrationv1.IfNeededReinvocationPolicy
				timeoutSeconds := int32(20)
				updatedWebhook := zitiwebhook.DeepCopy()
				updatedWebhook.Spec = kubernetesv1alpha1.ZitiWebhookSpec{
					ZitiControllerName: "ziticontroller-sample-updated",
					Name:               zitiwebhookName,
					Cert: kubernetesv1alpha1.CertificateSpecs{
						Duration:      int64((time.Duration(1440) * time.Hour).Hours()),
						RenewBefore:   int64((time.Duration(180) * time.Hour).Hours()),
						Organizations: []string{"DariuszInc"},
					},
					ServiceAccount: kubernetesv1alpha1.ServiceAccountSpec{
						ImagePullSecrets: []corev1.LocalObjectReference{
							{Name: "test-pull-secret"},
						},
						AutomountServiceAccountToken: &[]bool{true}[0],
						Secrets: []corev1.ObjectReference{
							{
								Kind:      "Secret",
								Name:      "test-secret",
								Namespace: zitiwebhookNamespace,
							},
						},
					},
					ClusterRoleSpec: kubernetesv1alpha1.ClusterRoleSpec{
						Rules: []rbacv1.PolicyRule{
							{ // Keep original rule
								APIGroups: []string{""},
								Resources: []string{"services", "namespaces"},
								Verbs:     []string{"get", "list", "watch"},
							},
							{ // Added rule
								APIGroups: []string{"apps"},
								Resources: []string{"configmaps"},
								Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
							},
						},
					},
					MutatingWebhookSpec: []admissionregistrationv1.MutatingWebhook{
						{
							Name: "tunnel.ziti.webhook",
							ObjectSelector: &metav1.LabelSelector{ // Change selector
								MatchLabels: map[string]string{"app": "test"},
							},
							NamespaceSelector: &metav1.LabelSelector{ // Change selector
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{Key: "kubernetes.io/metadata.name", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"kube-system", "cert-manager"}}, // Add cert-manager
								}},
							SideEffects:             &sideEffectClass,
							FailurePolicy:           &failurePolicy,
							TimeoutSeconds:          &timeoutSeconds,
							MatchPolicy:             &matchPolicy,
							ReinvocationPolicy:      &reinvocationPolicy,
							AdmissionReviewVersions: []string{"v1", "v1beta1"}, // Add v1beta1
							Rules: []admissionregistrationv1.RuleWithOperations{ // Change rules
								{
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}, // Removed Delete
									Rule: admissionregistrationv1.Rule{
										APIGroups:   []string{""},
										APIVersions: []string{"v1"}, // Removed v1beta1
										Resources:   []string{"pods"},
										Scope:       &[]admissionregistrationv1.ScopeType{admissionregistrationv1.NamespacedScope}[0], // Changed scope
									},
								},
							},
							ClientConfig: admissionregistrationv1.WebhookClientConfig{
								Service: &admissionregistrationv1.ServiceReference{
									Name:      zitiwebhookName + "-service",
									Namespace: zitiwebhookNamespace,
									Path:      &[]string{"/ziti-tunnel-v2"}[0],
									Port:      &[]int32{9443}[0],
								},
								CABundle: []byte("notReal"),
							},
						},
						{
							Name: "router.ziti.webhook",
							ObjectSelector: &metav1.LabelSelector{ // Change selector
								MatchLabels: map[string]string{"app": "test"},
							},
							NamespaceSelector: &metav1.LabelSelector{ // Change selector
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{Key: "kubernetes.io/metadata.name", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"cert-manager"}}, // Add cert-manager
									{Key: "router.openziti.io/enabled", Operator: metav1.LabelSelectorOpIn, Values: []string{"true", "false"}},    // Add cert-manager
								}},
							SideEffects:             &sideEffectClass,
							FailurePolicy:           &failurePolicy,
							TimeoutSeconds:          &timeoutSeconds,
							MatchPolicy:             &matchPolicy,
							ReinvocationPolicy:      &reinvocationPolicy,
							AdmissionReviewVersions: []string{"v1", "v1beta1"}, // Add v1beta1
							Rules: []admissionregistrationv1.RuleWithOperations{ // Change rules
								{
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}, // Removed Delete
									Rule: admissionregistrationv1.Rule{
										APIGroups:   []string{""},
										APIVersions: []string{"v1"}, // Removed v1beta1
										Resources:   []string{"pods"},
										Scope:       &[]admissionregistrationv1.ScopeType{admissionregistrationv1.NamespacedScope}[0], // Changed scope
									},
								},
							},
							ClientConfig: admissionregistrationv1.WebhookClientConfig{
								Service: &admissionregistrationv1.ServiceReference{
									Name:      zitiwebhookName + "-service",
									Namespace: zitiwebhookNamespace,
									Path:      &[]string{"/ziti-router-v2"}[0],
									Port:      &[]int32{9443}[0],
								},
								CABundle: []byte("notReal"),
							},
						},
					},
					DeploymentSpec: kubernetesv1alpha1.DeploymentSpec{
						Replicas:        1,
						Image:           "openziti/ziti-edge-tunnel",
						ImageVersion:    "1.0.0",
						ImagePullPolicy: corev1.PullAlways,
						Port:            8443,
						Env: kubernetesv1alpha1.DeploymentEnvVars{
							SidecarImage:           "openziti/ziti-edge-tunnel",
							SidecarImageVersion:    "1.2.0",
							SidecarImagePullPolicy: corev1.PullAlways,
							SidecarPrefix:          "zt-updated",
							SidecarIdentityDir:     "/ziti-tunnel-updated",
							ZitiCtrlMgmtApi:        "https://updated-controller:1280/edge/management/v1",
							PodSecurityOverride:    true,
							ClusterDnsServiceIP:    "",
							SearchDomainList:       "",
							ZitiRoleKey:            "identity.openziti.io/role-attributes-updated",
						},
						ResourceRequest: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
						ResourceLimit: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						MaxUnavailable:                "50%",
						MaxSurge:                      "50%",
						TerminationGracePeriodSeconds: 45,
						ProgressDeadlineSeconds:       900,
						RevisionHistoryLimit:          5,
						LogLevel:                      3,
					},
				}
				Expect(k8sClient.Update(ctx, updatedWebhook)).To(Succeed())

				// Drain events from the update itself if any (e.g., finalizer added if missing)
				Eventually(fakeRecorder.Events).ShouldNot(Receive())

				By("Running the reconcile loop again after update")
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())

				ownerRef.UID = zitiwebhook.UID // Ensure ownerRef has the correct UID after creation

				By("Verifying the updated resources and events")
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-ca-issuer"}, issuer)).To(Succeed())
				Expect(issuer.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))

				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-admission-cert"}, certificate)).To(Succeed())
				Expect(certificate.Spec.Duration.Duration).To(Equal(time.Duration(1440) * time.Hour))
				Expect(certificate.Spec.RenewBefore.Duration).To(Equal(time.Duration(180) * time.Hour))
				Expect(certificate.Spec.Subject.Organizations).To(Equal([]string{"DariuszInc"}))
				Expect(certificate.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Patched Certificate")))

				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-service"}, service)).To(Succeed())
				Expect(service.Spec.Ports[0].TargetPort.IntVal).To(Equal(int32(8443)))
				Expect(service.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Patched Service")))

				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-service-account"}, serviceAccount)).To(Succeed())
				Expect(serviceAccount.ImagePullSecrets).To(HaveLen(1))
				Expect(serviceAccount.ImagePullSecrets[0].Name).To(Equal("test-pull-secret"))
				Expect(serviceAccount.AutomountServiceAccountToken).To(Equal(&[]bool{true}[0]))
				Expect(serviceAccount.Secrets).To(HaveLen(1))
				Expect(serviceAccount.Secrets[0].Name).To(Equal("test-secret"))
				Expect(serviceAccount.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Patched ServiceAccount")))

				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-cluster-role"}, clusterRole)).To(Succeed())
				Expect(clusterRole.Rules).To(HaveLen(2))
				Expect(clusterRole.Rules[0].APIGroups).To(Equal([]string{""}))
				Expect(clusterRole.Rules[0].Resources).To(Equal([]string{"services", "namespaces"}))
				Expect(clusterRole.Rules[0].Verbs).To(Equal([]string{"get", "list", "watch"}))
				Expect(clusterRole.Rules[0].ResourceNames).To(BeEmpty())
				Expect(clusterRole.Rules[0].NonResourceURLs).To(BeEmpty())
				Expect(clusterRole.Rules[1].APIGroups).To(Equal([]string{"apps"}))
				Expect(clusterRole.Rules[1].Resources).To(Equal([]string{"configmaps"}))
				Expect(clusterRole.Rules[1].Verbs).To(Equal([]string{"get", "list", "watch", "create", "update", "delete"}))
				Expect(clusterRole.ObjectMeta.Labels).To(HaveKeyWithValue("app", zitiwebhook.Spec.Name))
				Expect(clusterRole.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", zitiwebhook.Spec.Name+"-"+zitiwebhook.Namespace))
				Expect(clusterRole.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", zitiwebhook.Spec.Name+"-operator"))
				Expect(clusterRole.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", zitiwebhook.Spec.Name+"-controller"))
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Patched ClusterRole")))

				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-cluster-role-binding"}, clusterRoleBinding)).To(Succeed())

				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-mutating-webhook-configuration"}, mutatingWebhook)).To(Succeed())
				Expect(mutatingWebhook.Webhooks).To(HaveLen(2))
				Expect(mutatingWebhook.Webhooks[0].Name).To(Equal("tunnel.ziti.webhook"))
				Expect(mutatingWebhook.Webhooks[0].ObjectSelector.MatchLabels).To(HaveKeyWithValue("app", "test"))
				Expect(mutatingWebhook.Webhooks[0].NamespaceSelector.MatchExpressions[0].Values).To(ContainElement("cert-manager"))
				Expect(mutatingWebhook.Webhooks[0].SideEffects).To(Equal(&sideEffectClass))
				Expect(mutatingWebhook.Webhooks[0].FailurePolicy).To(Equal(&failurePolicy))
				Expect(mutatingWebhook.Webhooks[0].TimeoutSeconds).To(Equal(&timeoutSeconds))
				Expect(mutatingWebhook.Webhooks[0].MatchPolicy).To(Equal(&matchPolicy))
				Expect(mutatingWebhook.Webhooks[0].ReinvocationPolicy).To(Equal(&reinvocationPolicy))
				Expect(mutatingWebhook.Webhooks[0].AdmissionReviewVersions).To(Equal([]string{"v1", "v1beta1"}))
				Expect(mutatingWebhook.Webhooks[0].Rules[0].Operations).To(Equal([]admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}))
				Expect(*mutatingWebhook.Webhooks[0].Rules[0].Rule.Scope).To(Equal(admissionregistrationv1.NamespacedScope))
				Expect(*mutatingWebhook.Webhooks[0].ClientConfig.Service.Path).To(Equal("/ziti-tunnel-v2"))
				Expect(mutatingWebhook.Webhooks[0].ClientConfig.Service.Port).To(Equal(&[]int32{9443}[0]))
				Expect(mutatingWebhook.Webhooks[1].Name).To(Equal("router.ziti.webhook"))
				Expect(mutatingWebhook.Webhooks[1].ObjectSelector.MatchLabels).To(HaveKeyWithValue("app", "test"))
				Expect(mutatingWebhook.Webhooks[1].NamespaceSelector.MatchExpressions[0].Values).To(ContainElement("cert-manager"))
				Expect(mutatingWebhook.Webhooks[1].NamespaceSelector.MatchExpressions[1].Values).To(ContainElement("true"))
				Expect(mutatingWebhook.Webhooks[1].NamespaceSelector.MatchExpressions[1].Values).To(ContainElement("false"))
				Expect(mutatingWebhook.Webhooks[1].SideEffects).To(Equal(&sideEffectClass))
				Expect(mutatingWebhook.Webhooks[1].FailurePolicy).To(Equal(&failurePolicy))
				Expect(mutatingWebhook.Webhooks[1].TimeoutSeconds).To(Equal(&timeoutSeconds))
				Expect(mutatingWebhook.Webhooks[1].MatchPolicy).To(Equal(&matchPolicy))
				Expect(mutatingWebhook.Webhooks[1].ReinvocationPolicy).To(Equal(&reinvocationPolicy))
				Expect(mutatingWebhook.Webhooks[1].AdmissionReviewVersions).To(Equal([]string{"v1", "v1beta1"}))
				Expect(mutatingWebhook.Webhooks[1].Rules[0].Operations).To(Equal([]admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}))
				Expect(*mutatingWebhook.Webhooks[1].Rules[0].Rule.Scope).To(Equal(admissionregistrationv1.NamespacedScope))
				Expect(*mutatingWebhook.Webhooks[1].ClientConfig.Service.Path).To(Equal("/ziti-router-v2"))
				Expect(mutatingWebhook.Webhooks[1].ClientConfig.Service.Port).To(Equal(&[]int32{9443}[0]))
				// Expect(mutatingWebhook.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte("notReal")))
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Patched MutatingWebhookConfiguration")))

				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-deployment"}, deployment)).To(Succeed())
				Expect(deployment.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
				Expect(*deployment.Spec.Replicas).To(Equal(int32(1)))
				Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("openziti/ziti-edge-tunnel:1.0.0"))
				Expect(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullAlways))
				Expect(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullAlways))
				Expect(deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(int32(8443)))
				Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(Equal([]string{"webhook", "--v=3"}))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "ZITI_MGMT_API", Value: "https://updated-controller:1280/edge/management/v1"}))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "SIDECAR_IMAGE", Value: "openziti/ziti-edge-tunnel"}))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "SIDECAR_IMAGE_VERSION", Value: "1.2.0"}))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "SIDECAR_IMAGE_PULL_POLICY", Value: "Always"}))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "SIDECAR_PREFIX", Value: "zt-updated"}))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "SIDECAR_IDENTITY_DIR", Value: "/ziti-tunnel-updated"}))
				Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "ZITI_ROLE_KEY", Value: "identity.openziti.io/role-attributes-updated"}))
				Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String()).To(Equal("200m"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().String()).To(Equal("256Mi"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().String()).To(Equal("1"))
				Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().String()).To(Equal("1Gi"))
				Expect(deployment.Spec.Template.Spec.ServiceAccountName).To(Equal(zitiwebhookName + "-service-account"))
				Expect(deployment.Spec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyAlways))
				Expect(deployment.Spec.Template.Spec.DNSPolicy).To(Equal(corev1.DNSClusterFirst))
				Expect(deployment.Spec.Template.Spec.SecurityContext).To(Equal(&corev1.PodSecurityContext{}))
				Expect(deployment.Spec.Template.Spec.SchedulerName).To(Equal(corev1.DefaultSchedulerName))
				Expect(deployment.Spec.Template.Spec.TerminationGracePeriodSeconds).To(Equal(&[]int64{45}[0]))
				Expect(deployment.Spec.Strategy.Type).To(Equal(appsv1.RollingUpdateDeploymentStrategyType))
				Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.Type).To(Equal(intstr.String))
				Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.StrVal).To(Equal("50%"))
				Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.Type).To(Equal(intstr.String))
				Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.StrVal).To(Equal("50%"))
				Expect(deployment.Spec.ProgressDeadlineSeconds).To(Equal(&[]int32{900}[0]))
				Expect(deployment.Spec.RevisionHistoryLimit).To(Equal(&[]int32{5}[0]))
				Expect(deployment.ObjectMeta.Labels).To(HaveKeyWithValue("app", zitiwebhook.Spec.Name))
				Expect(deployment.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", zitiwebhook.Spec.Name+"-"+zitiwebhook.Namespace))
				Expect(deployment.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", zitiwebhook.Spec.Name+"-operator"))
				Expect(deployment.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", zitiwebhook.Spec.Name+"-controller"))
				Expect(deployment.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/component", "webhook"))
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Patched Deployment")))
			})
		})

		Context("Deleting Resources", func() {

			// Note: This test relies on the resources being created/updated by the previous tests in the Ordered context.
			It("should successfully recreate the deleted owned resources", func() {

				By("Ensuring resources exist before deletion")
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-ca-issuer"}, issuer)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-admission-cert"}, certificate)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-service"}, service)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-service-account"}, serviceAccount)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-deployment"}, deployment)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-cluster-role"}, clusterRole)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-cluster-role-binding"}, clusterRoleBinding)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-mutating-webhook-configuration"}, mutatingWebhook))
				}, timeout, interval).Should(BeFalse(), "Resources should exist before deletion")

				// Drain events from previous reconciles
				Eventually(fakeRecorder.Events).ShouldNot(Receive())

				By("Deleting resources manually")
				Expect(k8sClient.Delete(ctx, issuer)).To(Succeed())
				Expect(k8sClient.Delete(ctx, certificate)).To(Succeed())
				Expect(k8sClient.Delete(ctx, service)).To(Succeed())
				Expect(k8sClient.Delete(ctx, serviceAccount)).To(Succeed())
				Expect(k8sClient.Delete(ctx, deployment)).To(Succeed())
				Expect(k8sClient.Delete(ctx, clusterRole)).To(Succeed())
				Expect(k8sClient.Delete(ctx, clusterRoleBinding)).To(Succeed())
				Expect(k8sClient.Delete(ctx, mutatingWebhook)).To(Succeed())

				By("Verifying resources are deleted")
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-ca-issuer"}, issuer)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-admission-cert"}, certificate)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-service"}, service)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-service-account"}, serviceAccount)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-deployment"}, deployment)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-cluster-role"}, clusterRole)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-cluster-role-binding"}, clusterRoleBinding)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-mutating-webhook-configuration"}, mutatingWebhook))
				}, timeout, interval).Should(BeTrue())

				// Note: Cluster-scoped resources might not be recreated automatically by controller-runtime's Owns if deleted manually in testenv
				// We test their creation/update in other contexts.

				By("Running the reconcile loop again")
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())

				ownerRef.UID = zitiwebhook.UID // Ensure ownerRef has the correct UID after creation

				By("Checking if owned namespaced resources are reconciled successfully")
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-ca-issuer"}, issuer)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-admission-cert"}, certificate)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-service"}, service)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-service-account"}, serviceAccount)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-deployment"}, deployment)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-cluster-role"}, clusterRole)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-cluster-role-binding"}, clusterRoleBinding)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-mutating-webhook-configuration"}, mutatingWebhook))
				}, timeout, interval).Should(BeFalse(), "Resources should reconcile successfully")
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Created a new Issuer")))
				Expect(issuer.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Created a new Certificate")))
				Expect(certificate.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Created a new Service")))
				Expect(service.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Created a new ServiceAccount")))
				Expect(serviceAccount.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Created a new Deployment")))
				Expect(deployment.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
			})

			It("should reconcile labels and ownership if manually removed", func() {

				By("Removing labels from resources")
				existingIssuer := issuer.DeepCopy()
				existingCertificate := certificate.DeepCopy()
				existingService := service.DeepCopy()
				existingServiceAccount := serviceAccount.DeepCopy()
				existingDeployment := deployment.DeepCopy()
				existingClusterRole := clusterRole.DeepCopy()
				existingClusterRoleBinding := clusterRoleBinding.DeepCopy()
				existingMutatingWebhook := mutatingWebhook.DeepCopy()

				issuer.ObjectMeta.Labels = map[string]string{}
				issuer.OwnerReferences = nil
				certificate.ObjectMeta.Labels = map[string]string{}
				certificate.OwnerReferences = nil
				service.ObjectMeta.Labels = map[string]string{}
				service.OwnerReferences = nil
				serviceAccount.ObjectMeta.Labels = map[string]string{}
				serviceAccount.OwnerReferences = nil
				deployment.ObjectMeta.Labels = map[string]string{}
				deployment.OwnerReferences = nil
				clusterRole.ObjectMeta.Labels = map[string]string{}
				clusterRoleBinding.ObjectMeta.Labels = map[string]string{}
				mutatingWebhook.ObjectMeta.Labels = map[string]string{}

				Expect(k8sClient.Patch(ctx, issuer, client.MergeFrom(existingIssuer))).To(Succeed())
				Expect(k8sClient.Patch(ctx, certificate, client.MergeFrom(existingCertificate))).To(Succeed())
				Expect(k8sClient.Patch(ctx, service, client.MergeFrom(existingService))).To(Succeed())
				Expect(k8sClient.Patch(ctx, serviceAccount, client.MergeFrom(existingServiceAccount))).To(Succeed())
				Expect(k8sClient.Patch(ctx, deployment, client.MergeFrom(existingDeployment))).To(Succeed())
				Expect(k8sClient.Patch(ctx, clusterRole, client.MergeFrom(existingClusterRole))).To(Succeed())
				Expect(k8sClient.Patch(ctx, clusterRoleBinding, client.MergeFrom(existingClusterRoleBinding))).To(Succeed())
				Expect(k8sClient.Patch(ctx, mutatingWebhook, client.MergeFrom(existingMutatingWebhook))).To(Succeed())

				By("Verifying labels and owner reference are removed")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-ca-issuer"}, issuer)).To(Succeed())
					g.Expect(issuer.ObjectMeta.Labels).To(BeEmpty())
					g.Expect(issuer.OwnerReferences).To(BeEmpty())
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-admission-cert"}, certificate)).To(Succeed())
					g.Expect(certificate.ObjectMeta.Labels).To(BeEmpty())
					g.Expect(certificate.OwnerReferences).To(BeEmpty())
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-service"}, service)).To(Succeed())
					g.Expect(service.ObjectMeta.Labels).To(BeEmpty())
					g.Expect(service.ObjectMeta.OwnerReferences).To(BeEmpty())
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-service-account"}, serviceAccount)).To(Succeed())
					g.Expect(serviceAccount.ObjectMeta.Labels).To(BeEmpty())
					g.Expect(serviceAccount.ObjectMeta.OwnerReferences).To(BeEmpty())
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-deployment"}, deployment)).To(Succeed())
					g.Expect(deployment.ObjectMeta.Labels).To(BeEmpty())
					g.Expect(deployment.ObjectMeta.OwnerReferences).To(BeEmpty())
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-cluster-role"}, clusterRole)).To(Succeed())
					g.Expect(clusterRole.ObjectMeta.Labels).To(BeEmpty())
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-cluster-role-binding"}, clusterRoleBinding)).To(Succeed())
					g.Expect(clusterRoleBinding.ObjectMeta.Labels).To(BeEmpty())
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-mutating-webhook-configuration"}, mutatingWebhook)).To(Succeed())
					g.Expect(mutatingWebhook.ObjectMeta.Labels).To(BeEmpty())
				}, timeout, interval).Should(Succeed())

				By("Running the reconcile loop again")
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying labels are reconciled back onto the resources")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-ca-issuer"}, issuer)).To(Succeed())
					g.Expect(issuer.ObjectMeta.Labels).To(HaveKeyWithValue("app", zitiwebhook.Spec.Name))
					g.Expect(issuer.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", zitiwebhook.Spec.Name+"-"+zitiwebhook.Namespace))
					g.Expect(issuer.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", zitiwebhook.Spec.Name+"-operator"))
					g.Expect(issuer.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", zitiwebhook.Spec.Name+"-controller"))
					g.Expect(issuer.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/component", "webhook"))
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-admission-cert"}, certificate)).To(Succeed())
					g.Expect(certificate.ObjectMeta.Labels).To(HaveKeyWithValue("app", zitiwebhook.Spec.Name))
					g.Expect(certificate.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", zitiwebhook.Spec.Name+"-"+zitiwebhook.Namespace))
					g.Expect(certificate.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", zitiwebhook.Spec.Name+"-operator"))
					g.Expect(certificate.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", zitiwebhook.Spec.Name+"-controller"))
					g.Expect(certificate.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/component", "webhook"))
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-service"}, service)).To(Succeed())
					g.Expect(service.ObjectMeta.Labels).To(HaveKeyWithValue("app", zitiwebhook.Spec.Name))
					g.Expect(service.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", zitiwebhook.Spec.Name+"-"+zitiwebhook.Namespace))
					g.Expect(service.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", zitiwebhook.Spec.Name+"-operator"))
					g.Expect(service.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", zitiwebhook.Spec.Name+"-controller"))
					g.Expect(service.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/component", "webhook"))
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-service-account"}, serviceAccount)).To(Succeed())
					g.Expect(serviceAccount.ObjectMeta.Labels).To(HaveKeyWithValue("app", zitiwebhook.Spec.Name))
					g.Expect(serviceAccount.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", zitiwebhook.Spec.Name+"-"+zitiwebhook.Namespace))
					g.Expect(serviceAccount.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", zitiwebhook.Spec.Name+"-operator"))
					g.Expect(serviceAccount.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", zitiwebhook.Spec.Name+"-controller"))
					g.Expect(serviceAccount.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/component", "webhook"))
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-deployment"}, deployment)).To(Succeed())
					g.Expect(deployment.ObjectMeta.Labels).To(HaveKeyWithValue("app", zitiwebhook.Spec.Name))
					g.Expect(deployment.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", zitiwebhook.Spec.Name+"-"+zitiwebhook.Namespace))
					g.Expect(deployment.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", zitiwebhook.Spec.Name+"-operator"))
					g.Expect(deployment.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", zitiwebhook.Spec.Name+"-controller"))
					g.Expect(deployment.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/component", "webhook"))
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-cluster-role"}, clusterRole)).To(Succeed())
					g.Expect(clusterRole.ObjectMeta.Labels).To(HaveKeyWithValue("app", zitiwebhook.Spec.Name))
					g.Expect(clusterRole.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", zitiwebhook.Spec.Name+"-"+zitiwebhook.Namespace))
					g.Expect(clusterRole.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", zitiwebhook.Spec.Name+"-operator"))
					g.Expect(clusterRole.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", zitiwebhook.Spec.Name+"-controller"))
					g.Expect(clusterRole.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/component", "webhook"))
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-cluster-role-binding"}, clusterRoleBinding)).To(Succeed())
					g.Expect(clusterRoleBinding.ObjectMeta.Labels).To(HaveKeyWithValue("app", zitiwebhook.Spec.Name))
					g.Expect(clusterRoleBinding.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", zitiwebhook.Spec.Name+"-"+zitiwebhook.Namespace))
					g.Expect(clusterRoleBinding.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", zitiwebhook.Spec.Name+"-operator"))
					g.Expect(clusterRoleBinding.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", zitiwebhook.Spec.Name+"-controller"))
					g.Expect(clusterRoleBinding.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/component", "webhook"))
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-mutating-webhook-configuration"}, mutatingWebhook)).To(Succeed())
					g.Expect(mutatingWebhook.ObjectMeta.Labels).To(HaveKeyWithValue("app", zitiwebhook.Spec.Name))
					g.Expect(mutatingWebhook.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", zitiwebhook.Spec.Name+"-"+zitiwebhook.Namespace))
					g.Expect(mutatingWebhook.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", zitiwebhook.Spec.Name+"-operator"))
					g.Expect(mutatingWebhook.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", zitiwebhook.Spec.Name+"-controller"))
					g.Expect(mutatingWebhook.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/component", "webhook"))
				}, timeout, interval).Should(Succeed())

				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Patched Issuer")))
				Expect(issuer.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Patched Certificate")))
				Expect(certificate.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Patched Service")))
				Expect(service.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Patched ServiceAccount")))
				Expect(serviceAccount.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Patched Deployment")))
				Expect(deployment.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
			})

			It("should remove cluster-scoped resources when ZitiWebhook is deleted", func() {

				By("Deleting the ZitiWebhook resource")
				Expect(k8sClient.Get(ctx, typeNamespacedName, zitiwebhook)).To(Succeed())
				Expect(k8sClient.Delete(ctx, zitiwebhook)).To(Succeed())

				By("Running the reconcile loop for deletion")
				// Reconcile should be triggered by the deletion event and process the finalizer
				// We might need multiple reconcile loops for the finalizer logic to complete
				Eventually(func() bool {
					// Trigger reconcile
					_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
					Expect(err).NotTo(HaveOccurred())
					// Check if the resource is gone
					checkErr := k8sClient.Get(ctx, typeNamespacedName, zitiwebhook)
					return errors.IsNotFound(checkErr)
				}, timeout, interval).Should(BeTrue(), "ZitiWebhook should be deleted after finalizer runs") // Increased timeout

				By("Verifying cluster-scoped resources are deleted by the finalizer")
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-cluster-role"}, clusterRole)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-cluster-role-binding"}, clusterRoleBinding)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Name: zitiwebhookName + "-mutating-webhook-configuration"}, mutatingWebhook))
				}, timeout, interval).Should(BeTrue())

				By("by the way of the garbage collection due the owner's reference")
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-deployment"}, deployment)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-ca-issuer"}, issuer)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-admission-cert"}, certificate)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-service"}, service)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-deployment"}, deployment)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: zitiwebhookNamespace, Name: zitiwebhookName + "-service-account"}, serviceAccount))
				}, timeout, interval).Should(BeTrue())
			})
		})
	})
})
