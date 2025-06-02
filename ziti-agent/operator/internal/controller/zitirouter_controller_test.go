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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubernetesv1alpha1 "github.com/netfoundry/ziti-k8s-agent/ziti-agent/operator/api/v1alpha1"
)

var _ = Describe("ZitiRouter Controller", func() {

	const resourceName = "test-resource"
	const resourceNamespace = "default"
	// Define constants for Eventually timings
	const timeout = time.Second * 10
	const interval = time.Millisecond * 250
	ctx := context.Background()

	typeNamespacedName := types.NamespacedName{
		Name:      resourceName,
		Namespace: resourceNamespace,
	}
	zitirouter := &kubernetesv1alpha1.ZitiRouter{}
	controllerReconciler := &ZitiRouterReconciler{}
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
			Name:               resourceName,
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

	Describe("ZitiRouter Controller Creation with defined parameters", Ordered, func() {

		Context("Creating Resources using only defaults", func() {

			It("should successfully reconcile all resources", func() {

				By("Running the reconcile loop")
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
				Expect(err).NotTo(HaveOccurred())

				ownerRef.UID = zitirouter.UID // Ensure ownerRef has the correct UID after creation

				// By("Verifying the ZitiRouter resource has the expected owner reference")
				// Expect(zitirouter.OwnerReferences).To(ContainElement(ownerRef))

				By("Checking if the ZitiRouter resource has been created with the correct spec")
				Expect(zitirouter.Spec.ZitiControllerName).To(Equal("ziticontroller-sample"))
				Expect(zitirouter.Spec.Name).To(Equal(resourceName))
			})
		})
	})

	Describe("ZitiRouter Controller Reconciliation", Ordered, func() {
		It("should reconcile the ZitiRouter resource", func() {
			By("Reconcile the ZitiRouter resource")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if the ZitiRouter resource is still present after reconciliation")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, zitirouter)
				return err == nil
			}, timeout, interval).Should(BeTrue())
		})
	})

	Describe("ZitiRouter Controller Deletion", Ordered, func() {
		It("should delete the ZitiRouter resource", func() {
			By("Deleting the ZitiRouter resource")
			Expect(k8sClient.Delete(ctx, zitirouter)).To(Succeed())

			By("Checking if the ZitiRouter resource is deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, typeNamespacedName, zitirouter)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
