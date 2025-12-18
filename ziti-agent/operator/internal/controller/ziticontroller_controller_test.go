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
						Name:            resourceName,
						AdminJwt:        "eyJhbGciOiJSUzI1NiIsImtpZCI6Ijc4MTQzY2YxZTliZDg1ZWE5ODNhNDJlYmNmYzMyOTQ3OTU3NzBkNzIiLCJ0eXAiOiJKV1QifQ.eyJpc3MiOiJodHRwczovL2Q2ZTVmZTBhLWI1NzMtNGQxNC04ODliLWJlNjE5NzFhNTI3Yy5wcm9kdWN0aW9uLm5ldGZvdW5kcnkuaW86NDQzIiwic3ViIjoidGljTXMwbUxJaCIsImF1ZCI6WyIiXSwiZXhwIjoxNzQ4NTI0MDg4LCJqdGkiOiJmZGEwMWIxZi0xMjI2LTQyMWEtODI5Ni01NzkxNDE3YWIyMjgiLCJlbSI6Im90dCIsImN0cmxzIjpudWxsfQ.KexPTSX689s0e36SyUyWHocex95lZ8IthDir7jqhPEMwYXOMEz-Js3Pb1_xM4xLl9Re2dLHB6fR7T7DTRBCwHAVa2yznnX50n9w8co_2PEiCcto7MjfkyIrlz2PSIvmPoDQ0QX-XOVMk_o3A01zG4WG3jrJ5eieqqSbLYXf6TxHrgZyjNjMg9EtuUmY4FbGWKSJGCCe7_xhZCfVcJ5l553LBVpCc6WhJbkzWjnc_q6s_o_V44r03XqqNlJTgIQfsnpV98jdu0Zp4zX5-E51SiTvLHL7uR82Nm7A6eypU0ETeEMRc8sLkMbi9uHgM0TM8Sgbs8D8qBn1_gsrAm-dl0GySH-YFT8BQHEAlaE76o_q8p-_ylUUZlYMIYcTno5GN37TG3FN7dDG3eb6S93cU-iuEY3lVmeRA_KuJvh1koUncEpP_Sh648TM2952Bg9kfsQ1-OfYiHEKa7aDZno1_ApsWOoqMYwZl1-8fhMG_B1qMYVFGZw72wzbLq4JHLev0N9tE99T-gF8Q6VKXvcXCToN-KTkQ03cucO6p6-XqnVpq1wznSveoh4TpCsq2NKqx5SpLpTPJ8xGEaH4QrIt7700sXEdhZLU4ZjfK6VP83V2xWNtokPHBGro7rvxsPtHe_wSWvZN2CyntmtkY6fHBhS1qAk3sqDj-otC2vXOx3Es",
						ZitiCtrlMgmtApi: "https://d6e5fe0a-b573-4d14-889b-be61971a527c.production.netfoundry.io:443/edge/management/v1",
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
