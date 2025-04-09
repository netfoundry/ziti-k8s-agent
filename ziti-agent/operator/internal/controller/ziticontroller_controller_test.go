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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubernetesv1alpha1 "github.com/netfoundry/ziti-k8s-agent/ziti-agent/operator/api/v1alpha1"
)

var _ = Describe("ZitiController Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-controller"
		const resourceNamespace = "default"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		ziticontroller := &kubernetesv1alpha1.ZitiController{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind ZitiController")
			err := k8sClient.Get(ctx, typeNamespacedName, ziticontroller)
			if err != nil && errors.IsNotFound(err) {
				resource := &kubernetesv1alpha1.ZitiController{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: kubernetesv1alpha1.ZitiControllerSpec{
						Name:     resourceName,
						AdminJwt: "eyJhbGciOiJSUzI1NiIsImtpZCI6ImI0NWFmYWQ1NzFiNDM1NjBkNDU2Y2JjYjI4YzBlZjkwZDgzYzQ2ZTIiLCJ0eXAiOiJKV1QifQ.eyJpc3MiOiJodHRwczovLzhlZDE1YmVhLTU2MDMtNDg1ZS04NTc5LWEyOTcxZDJkYTY4NC5wcm9kdWN0aW9uLm5ldGZvdW5kcnkuaW86NDQzIiwic3ViIjoiWUk3aXIyWkdUbiIsImF1ZCI6WyIiXSwiZXhwIjoxNzQyMjY2MzQxLCJqdGkiOiI0YjNmZjFiZC1hYjJjLTRiOWYtYjQ0Yi0zZjZjOTkwNWZmZWMiLCJlbSI6Im90dCIsImN0cmxzIjpudWxsfQ.BhyAGv_E9P0NYF8tJjDWyXeXHNKAfMmSeBHpa_J0Qs3TSrhWA1-u3BMxtYvCk9zpMgm50Ft3StCxPahgveTT3w40yBgd-uZ8uHRQle5pUkbhXn8g9E5LgAN5ImyFLUVI_vaG-xqJ8uHKG5ScWUU3z7bHVqoURCrlVSwBBF_iw27DA4o3AYTtDvGk7Y5Jbv_4hdyXGHghMoH1rrP0puENYYgLRSk3q9Y4pClk874uA6e_e3SkX_YsxzDVSiFStFAalOce9EpxP_ngN7Cy1Tu7vWAKgCBoYkf5I0AEBRR95yS838cOXqWW25ZK414nj5HYi00_OuzXA4aAQnrYqAsAohVOcDBXzdbDF7jl_oJZWZOeX17YXBXQbdVuA9Ss72vt090oW3_SZhx4zn3k7XRnT6B09kBNF7qNKNdYCAAfL3s0eQgQHp2ZBRaxmjtzbrKNiipREMygBI_B3XPViVhhdDgMtMAkVjvQjeZjsYyGChipFINlCtJjT9BY8Ls3zOWnEXBl35rsu2KIZBMijlZwWTWODoUs46-aoTcCTNyo-_4Clb1M_3-RSYRe41MOZd4TgTJCOTrwUZ5e3l70i2d5qmL6KpRzZ8ONIypmZWS_RIXC_dunyEwriK_G_CMwwWYmWLmvvO4oy0Opzbjl54Fn2lWZLJgHLOTB7Vgr3MHa6Z4",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &kubernetesv1alpha1.ZitiController{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ZitiController")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &ZitiControllerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).To(HaveOccurred())
			Expect(strings.Contains(err.Error(), "token is expired")).To(BeTrue())

		})
	})
})
