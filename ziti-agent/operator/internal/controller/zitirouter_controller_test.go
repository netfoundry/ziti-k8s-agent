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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubernetesv1alpha1 "github.com/netfoundry/ziti-k8s-agent/ziti-agent/operator/api/v1alpha1"
)

var _ = Describe("ZitiRouter Controller", func() {

	const routerName = "test-router"
	const routerNamespace = "default"
	// Define constants for Eventually timings
	const timeout = time.Second * 10
	const interval = time.Millisecond * 250

	ctx := context.Background()

	typeNamespacedName := types.NamespacedName{
		Name:      routerName,
		Namespace: routerNamespace,
	}
	zitirouter := &kubernetesv1alpha1.ZitiRouter{}
	controllerReconciler := &ZitiRouterReconciler{}
	configMap := &corev1.ConfigMap{}
	service := &corev1.Service{}
	statefulset := &appsv1.StatefulSet{}
	ownerRef := metav1.OwnerReference{}

	BeforeEach(OncePerOrdered, func() {

		// Initialize the reconciler within the BeforeEach where the client is ready
		controllerReconciler = &ZitiRouterReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Recorder: fakeRecorder,
		}

		ownerRef = metav1.OwnerReference{
			APIVersion:         kubernetesv1alpha1.GroupVersion.String(),
			Kind:               "ZitiRouter",
			Name:               routerName,
			UID:                zitirouter.UID, // Ensure zitirouter has UID after creation/get
			Controller:         &[]bool{true}[0],
			BlockOwnerDeletion: &[]bool{true}[0],
		}

		By("ensuring resources from previous tests are cleaned up")
		err := k8sClient.Get(ctx, typeNamespacedName, zitirouter)
		if err == nil {
			By("Deleting the existing ZitiRouter resource")
			Expect(k8sClient.Delete(ctx, zitirouter)).To(Succeed())

			By("Cleaning up the resources created by the previous test using reconcile loop")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
		}

		By("creating the custom resource for the Kind ZitiRouter")
		zitirouter = &kubernetesv1alpha1.ZitiRouter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      typeNamespacedName.Name,
				Namespace: typeNamespacedName.Namespace,
			},
			Spec: kubernetesv1alpha1.ZitiRouterSpec{
				ZitiControllerName: "ziticontroller-sample",
				Name:               typeNamespacedName.Name,
				Deployment: kubernetesv1alpha1.RouterDeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":                    typeNamespacedName.Name,
							"app.kubernetes.io/name": typeNamespacedName.Name + "-" + typeNamespacedName.Namespace,
						},
					},
				},
			},
		}

		By("Creating the ZitiRouter resource")
		Expect(k8sClient.Create(ctx, zitirouter)).To(Succeed())

		// Drain events before each test to prevent interference
		Eventually(fakeRecorder.Events).ShouldNot(Receive())
	})

	AfterEach(OncePerOrdered, func() {
		By("Cleanup ZitiRouter if exists")
		err := k8sClient.Get(ctx, typeNamespacedName, zitirouter)
		if err == nil {
			Expect(k8sClient.Delete(ctx, zitirouter)).To(Succeed())
			// Wait for deletion to complete, including finalizer processing
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, zitirouter)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue()) // Increase timeout for finalizer
		} else if !errors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred())
		}

		By("Drainging events after each test")
		Eventually(fakeRecorder.Events).ShouldNot(Receive())
	})

	Describe("ZitiRouter Controller Creation with default parameters", Ordered, func() {

		Context("Creating Resources using only defaults", func() {

			It("should successfully reconcile all resources", func() {

				By("Running the reconcile loop")
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())

				ownerRef.UID = zitirouter.UID // Ensure ownerRef has the correct UID after creation

				By("Verifying the ZitiRouter resource has the expected owner reference")
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Created a new ConfigMap")))
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-config"}, configMap)).To(Succeed())
				Expect(configMap.OwnerReferences).To(ContainElement(ownerRef))

				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Created a new Service")))
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-service"}, service)).To(Succeed())
				Expect(service.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))

				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Created a new StatefulSet")))
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-statefulset"}, statefulset)).To(Succeed())
				Expect(statefulset.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))

				By("Checking if the ZitiRouter resource has been created with the correct spec")
				Expect(zitirouter.Spec.ZitiControllerName).To(Equal("ziticontroller-sample"))
				Expect(zitirouter.Spec.Name).To(Equal(routerName))

				By("Verifying the ConfigMap specs")
				Expect(configMap.Data).To(HaveKey("ziti-router.yaml"))
				Expect(configMap.Data).To(HaveLen(1))
				Expect(configMap.Data["ziti-router.yaml"]).To(Equal("v: 3\nidentity:\n  cert: /etc/ziti/config/test-router.cert\n  server_cert: /etc/ziti/config/test-router.server.chain.cert\n  key: /etc/ziti/config/test-router.key\n  ca: /etc/ziti/config/test-router.cas\n\nctrl:\n  endpoint: \n\nlink:\n  dialers:\n    - binding: transport\n\nlisteners:\n  - binding: edge\n    address: tls:0.0.0.0:9443\n    options:\n      advertise: test-router-service.default.svc.cluster.local:443\n      connectTimeoutMs: 5000\n      getSessionTimeout: 60s\n\nedge:\n  csr:\n    country: US\n    province: NC\n    locality: Charlotte\n    organization: NetFoundry\n    organizationalUnit: Ziti\n    sans:\n      dns:\n        - localhost\n        - test-router-service.default.svc.cluster.local\n      ip:\n        - 127.0.0.1\n      email:\n      uri:\n\nweb:\n  - name: health-check\n    bindPoints:\n      - interface: 0.0.0.0:8081\n        address: 0.0.0.0:8081 \t\n    apis:\n      - binding: health-checks\n"))

				By("Verifying the Service specs")
				Expect(service.Spec.Selector).To(HaveKeyWithValue("app", routerName))
				Expect(service.Spec.Selector).To(HaveKeyWithValue("app.kubernetes.io/name", routerName+"-"+routerNamespace))
				Expect(service.Spec.Ports).To(HaveLen(1))
				Expect(service.Spec.Ports[0].Name).To(Equal("edge"))
				Expect(service.Spec.Ports[0].Port).To(Equal(int32(443)))
				Expect(service.Spec.Ports[0].TargetPort.IntVal).To(Equal(int32(9443)))
				Expect(service.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
				Expect(service.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
			})
		})

		Context("Creating Resources using all custom spec fields", func() {

			It("should successfully reconcile all resources with updated specs", func() {

				By("Fetching the latest version of the ZitiWebhook resource")
				Expect(k8sClient.Get(ctx, typeNamespacedName, zitirouter)).To(Succeed())

				By("Defining the updated spec")
				updatedRouter := zitirouter.DeepCopy()
				updatedRouter.Spec = kubernetesv1alpha1.ZitiRouterSpec{
					ZitiControllerName: "ziticontroller-sample-updated",
					Name:               routerName,
					ZitiCtrlMgmtApi:    "https://updated-controller:1280/edge/management/v1",
					Config: kubernetesv1alpha1.Config{
						Ctrl: kubernetesv1alpha1.Ctrl{
							Endpoint: "tls:updated-controller:80",
						},
					},
					Deployment: kubernetesv1alpha1.RouterDeploymentSpec{
						Replicas: &[]int32{1}[0],
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app":                    typeNamespacedName.Name,
								"app.kubernetes.io/name": typeNamespacedName.Name + "-" + typeNamespacedName.Namespace,
							},
						},
						Annotations: map[string]string{"example.com/annotation": "value"},
						Container: corev1.Container{
							Image:           "openziti/ziti-router:1.5.4",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            []string{"run", "/etc/ziti/config/ziti-router1.yaml"},
							Command: []string{
								"/entrypoint1.bash",
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "edge1",
									ContainerPort: 8443,
									Protocol:      corev1.ProtocolUDP,
								},
							},
							Env: []corev1.EnvVar{
								{Name: "ZITI_BOOTSTRAP", Value: "false"},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("400m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
						},
						UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
							Type:          appsv1.RollingUpdateStatefulSetStrategyType,
							RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{Partition: &[]int32{1}[0]},
						},
						LogLevel: 3,
					},
				}
				Expect(k8sClient.Update(ctx, updatedRouter)).To(Succeed())

				// Drain events from the update itself if any (e.g., finalizer added if missing)
				Eventually(fakeRecorder.Events).ShouldNot(Receive())

				By("Running the reconcile loop again after update")
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())

				ownerRef.UID = zitirouter.UID // Ensure ownerRef has the correct UID after creation

				By("Verifying the updated resources and events")
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-config"}, configMap)).To(Succeed())
				Expect(configMap.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))

				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-service"}, service)).To(Succeed())
				Expect(service.Spec.Ports[0].TargetPort.IntVal).To(Equal(int32(8443)))
				Expect(service.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Patched Service")))

				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-statefulset"}, statefulset)).To(Succeed())
					g.Expect(statefulset.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
					g.Expect(*statefulset.Spec.Replicas).To(Equal(int32(1)))
					g.Expect(statefulset.Spec.Template.Spec.Containers[0].Image).To(Equal("openziti/ziti-router:1.5.4"))
					g.Expect(statefulset.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
					g.Expect(statefulset.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(int32(8443)))
					g.Expect(statefulset.Spec.Template.Spec.Containers[0].Ports[0].Protocol).To(Equal(corev1.ProtocolUDP))
					g.Expect(statefulset.Spec.Template.Spec.Containers[0].Args).To(ContainElement("run"))
					g.Expect(statefulset.Spec.Template.Spec.Containers[0].Args).To(ContainElement("/etc/ziti/config/ziti-router1.yaml"))
					g.Expect(statefulset.Spec.Template.Spec.Containers[0].Command).To(ContainElement("/entrypoint1.bash"))
					g.Expect(statefulset.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "ZITI_BOOTSTRAP", Value: "false"}))
					g.Expect(statefulset.Spec.Template.Spec.Containers[0].Resources.Requests).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("400m")))
					g.Expect(statefulset.Spec.Template.Spec.Containers[0].Resources.Requests).To(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("512Mi")))
					g.Expect(statefulset.Spec.Template.Spec.Containers[0].Resources.Limits).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("1")))
					g.Expect(statefulset.Spec.Template.Spec.Containers[0].Resources.Limits).To(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("1Gi")))
					g.Expect(statefulset.Spec.UpdateStrategy.Type).To(Equal(appsv1.RollingUpdateStatefulSetStrategyType))
					g.Expect(statefulset.Spec.UpdateStrategy.RollingUpdate).NotTo(BeNil(), "RollingUpdate strategy should be configured")
					g.Expect(statefulset.Spec.UpdateStrategy.RollingUpdate.Partition).To(Equal(&[]int32{1}[0]))
				}, timeout, interval).Should(Succeed())
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Patched StatefulSet")))
			})
		})

		Context("Deleting Resources", func() {

			// Note: This test relies on the resources being created/updated by the previous tests in the Ordered context.
			It("should successfully recreate the deleted owned resources", func() {

				By("Ensuring resources exist before deletion")
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-config"}, configMap)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-service"}, service)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-statefulset"}, statefulset))
				}, timeout, interval).Should(BeFalse(), "Resources should exist before deletion")

				// Drain events from previous reconciles
				Eventually(fakeRecorder.Events).ShouldNot(Receive())

				By("Deleting resources manually")
				Expect(k8sClient.Delete(ctx, configMap)).To(Succeed())
				Expect(k8sClient.Delete(ctx, service)).To(Succeed())
				Expect(k8sClient.Delete(ctx, statefulset)).To(Succeed())

				By("Verifying resources are deleted")
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-config"}, configMap)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-service"}, service)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-statefulset"}, statefulset))
				}, timeout, interval).Should(BeTrue(), "Resources should be deleted")

				By("Running the reconcile loop again")
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())

				ownerRef.UID = zitirouter.UID // Ensure ownerRef has the correct UID after creation

				By("Checking if owned namespaced resources are reconciled successfully")
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-config"}, configMap)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-service"}, service)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-statefulset"}, statefulset))
				}, timeout, interval).Should(BeFalse(), "Resources should reconcile successfully")
				Expect(configMap.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
				Expect(service.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
				Expect(statefulset.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
			})

			It("should reconcile labels and ownership if manually removed", func() {

				By("Removing labels from resources")
				existingConfigMap := configMap.DeepCopy()
				existingService := service.DeepCopy()
				existingStatefulSet := statefulset.DeepCopy()

				configMap.ObjectMeta.Labels = map[string]string{}
				configMap.OwnerReferences = nil
				service.ObjectMeta.Labels = map[string]string{}
				service.OwnerReferences = nil
				statefulset.ObjectMeta.Labels = map[string]string{}
				statefulset.OwnerReferences = nil
				Expect(k8sClient.Patch(ctx, configMap, client.MergeFrom(existingConfigMap))).To(Succeed())
				Expect(k8sClient.Patch(ctx, service, client.MergeFrom(existingService))).To(Succeed())
				Expect(k8sClient.Patch(ctx, statefulset, client.MergeFrom(existingStatefulSet))).To(Succeed())

				By("Verifying labels and owner reference are removed")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-config"}, configMap)).To(Succeed())
					g.Expect(configMap.ObjectMeta.Labels).To(BeEmpty())
					g.Expect(configMap.OwnerReferences).To(BeEmpty())
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-service"}, service)).To(Succeed())
					g.Expect(service.ObjectMeta.Labels).To(BeEmpty())
					g.Expect(service.ObjectMeta.OwnerReferences).To(BeEmpty())
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-statefulset"}, statefulset)).To(Succeed())
					g.Expect(statefulset.ObjectMeta.Labels).To(BeEmpty())
					g.Expect(statefulset.ObjectMeta.OwnerReferences).To(BeEmpty())
				}, timeout, interval).Should(Succeed())

				By("Running the reconcile loop again")
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())

				By("Verifying labels are reconciled back onto the resources")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-config"}, configMap)).To(Succeed())
					g.Expect(configMap.ObjectMeta.Labels).To(HaveKeyWithValue("app", zitirouter.Spec.Name))
					g.Expect(configMap.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", zitirouter.Spec.Name+"-"+zitirouter.Namespace))
					g.Expect(configMap.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", zitirouter.Spec.Name+"-operator"))
					g.Expect(configMap.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", zitirouter.Spec.Name+"-controller"))
					g.Expect(configMap.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/component", "router"))
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-service"}, service)).To(Succeed())
					g.Expect(service.ObjectMeta.Labels).To(HaveKeyWithValue("app", zitirouter.Spec.Name))
					g.Expect(service.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", zitirouter.Spec.Name+"-"+zitirouter.Namespace))
					g.Expect(service.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", zitirouter.Spec.Name+"-operator"))
					g.Expect(service.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", zitirouter.Spec.Name+"-controller"))
					g.Expect(service.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/component", "router"))
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-statefulset"}, statefulset)).To(Succeed())
					g.Expect(statefulset.ObjectMeta.Labels).To(HaveKeyWithValue("app", zitirouter.Spec.Name))
					g.Expect(statefulset.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", zitirouter.Spec.Name+"-"+zitirouter.Namespace))
					g.Expect(statefulset.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/part-of", zitirouter.Spec.Name+"-operator"))
					g.Expect(statefulset.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", zitirouter.Spec.Name+"-controller"))
					g.Expect(statefulset.ObjectMeta.Labels).To(HaveKeyWithValue("app.kubernetes.io/component", "router"))
				}, timeout, interval).Should(Succeed())

				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Patched ConfigMap")))
				Expect(configMap.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Patched Service")))
				Expect(service.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
				Eventually(fakeRecorder.Events, timeout).Should(Receive(ContainSubstring("Patched StatefulSet")))
				Expect(statefulset.ObjectMeta.OwnerReferences).To(ContainElement(ownerRef))
			})

			It("should remove cluster-scoped resources when ZitiRouter is deleted", func() {

				By("Deleting the ZitiRouter resource")
				Expect(k8sClient.Get(ctx, typeNamespacedName, zitirouter)).To(Succeed())
				Expect(k8sClient.Delete(ctx, zitirouter)).To(Succeed())

				By("Running the reconcile loop for deletion")
				// Reconcile should be triggered by the deletion event and process the finalizer if exists
				// We might need multiple reconcile loops for the finalizer logic to complete
				Eventually(func() bool {
					// Trigger reconcile
					_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
					Expect(err).NotTo(HaveOccurred())
					// Check if the resource is gone
					checkErr := k8sClient.Get(ctx, typeNamespacedName, zitirouter)
					return errors.IsNotFound(checkErr)
				}, timeout, interval).Should(BeTrue(), "zitirouter should be deleted after finalizer runs")

				By("by the way of the garbage collection due the owner's reference")
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-config"}, configMap)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-service"}, service)) &&
						errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKey{Namespace: routerNamespace, Name: routerName + "-statefulset"}, statefulset))
				}, timeout, interval).Should(BeTrue(), "Cluster-scoped resources should be deleted after ZitiRouter deletion")
			})
		})
	})
})
