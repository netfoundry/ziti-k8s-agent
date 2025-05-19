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
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/golang-jwt/jwt/v5"
	kubernetesv1alpha1 "github.com/netfoundry/ziti-k8s-agent/ziti-agent/operator/api/v1alpha1"
	"github.com/openziti/sdk-golang/ziti"
	"github.com/openziti/sdk-golang/ziti/enroll"
)

// ZitiControllerReconciler reconciles a ZitiController object
type ZitiControllerReconciler struct {
	client.Client
	Scheme                *runtime.Scheme
	WebhookControllerChan chan<- *kubernetesv1alpha1.ZitiController
	RouterControllerChan  chan<- *kubernetesv1alpha1.ZitiController
	channelChecked        bool
}

// +kubebuilder:rbac:groups=kubernetes.openziti.io,resources=ziticontrollers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubernetes.openziti.io,resources=ziticontrollers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubernetes.openziti.io,resources=ziticontrollers/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ZitiController object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *ZitiControllerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(2).Info("ZitiController Reconciliation started")

	ziticontroller := &kubernetesv1alpha1.ZitiController{}
	if err := r.Get(ctx, req.NamespacedName, ziticontroller); err != nil && apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}

	// Deep copy the fetched resource
	existingZitiController := ziticontroller.DeepCopy()

	foundAdminSecret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: ziticontroller.Namespace,
		Name:      ziticontroller.Spec.Name + "-secret",
	}, foundAdminSecret); err != nil && apierrors.IsNotFound(err) {
		if ziticontroller.Spec.AdminJwt == "" {
			return ctrl.Result{}, errors.New("admin jwt is empty")
		}
		if err := r.updateAdminSecret(ctx, ziticontroller, "create"); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		isOttExpired := verifyOtt(ziticontroller.Spec.AdminJwt)
		isCertExpired, _ := checkCertExpiration(foundAdminSecret.Data["tls.crt"])
		if isCertExpired && !isOttExpired {
			if err := r.updateAdminSecret(ctx, ziticontroller, "update"); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// // Determine if the spec has changed
	specChanged := !reflect.DeepEqual(existingZitiController.Spec, ziticontroller.Spec)

	// Check if the channel is empty only on the first access
	if !r.channelChecked {
		select {
		case r.WebhookControllerChan <- ziticontroller:
			log.V(2).Info("ZitiController spec changed, sending update to channel")
		default:
			log.V(2).Info("ZitiController channel is empty on first access and written to")
		}

		select {
		case r.RouterControllerChan <- ziticontroller:
			log.V(2).Info("ZitiController spec changed, sending update to channel")
		default:
			log.V(2).Info("ZitiController channel is empty on first access and written to")
		}
		r.channelChecked = true
	}
	if specChanged {
		r.WebhookControllerChan <- ziticontroller
		log.V(2).Info("ZitiController spec changed, sending update to webhook channel")

		r.RouterControllerChan <- ziticontroller
		log.V(2).Info("ZitiController spec changed, sending update to router channel")
	}

	log.V(2).Info("ZitiController Reconciliation finished")
	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ZitiControllerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubernetesv1alpha1.ZitiController{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

func (r *ZitiControllerReconciler) updateAdminSecret(ctx context.Context, ziticontroller *kubernetesv1alpha1.ZitiController, method string) error {
	zitiCfg, err := enrollIdentityWithJwt(ziticontroller.Spec.AdminJwt)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ziticontroller.Spec.Name + "-secret",
			Namespace: ziticontroller.Namespace,
			Labels: map[string]string{
				"app":                    ziticontroller.Spec.Name,
				"app.kubernetes.io/name": ziticontroller.Spec.Name + "-" + ziticontroller.Namespace,
			},
		},
		Data: map[string][]byte{
			"tls.key": []byte(strings.TrimPrefix(zitiCfg.ID.Key, "pem:")),
			"tls.crt": []byte(strings.TrimPrefix(zitiCfg.ID.Cert, "pem:")),
			"tls.ca":  []byte(strings.TrimPrefix(zitiCfg.ID.CA, "pem:")),
		},
		Type: "kubernetes.io/tls",
	}
	if err := controllerutil.SetControllerReference(ziticontroller, secret, r.Scheme); err != nil {
		return err
	}
	if method == "update" {
		if err := r.Client.Update(ctx, secret); err != nil {
			return err
		}
	}
	if method == "create" {
		if err := r.Client.Create(ctx, secret); err != nil {
			return err
		}
	}
	return nil
}

func verifyOtt(ott string) bool {
	// Parse the token without verifying signature
	token, _, err := new(jwt.Parser).ParseUnverified(ott, jwt.MapClaims{})
	if err != nil {
		log.Log.Error(err, "Error parsing token:")
		return false
	}

	// Check if the token is valid
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		expirationTime := claims["exp"].(float64)
		currentTime := time.Now().Unix()

		if currentTime > int64(expirationTime) {
			return false
		} else {
			return true
		}
	}
	return false
}

func enrollIdentityWithJwt(jwtToken string) (*ziti.Config, error) {
	tkn, _, err := enroll.ParseToken(jwtToken)
	if err != nil {
		return nil, err
	}
	flags := enroll.EnrollmentFlags{
		Token:  tkn,
		KeyAlg: "RSA",
	}
	zitiCfg, err := enroll.Enroll(flags)
	if err != nil {
		return nil, err
	}
	return zitiCfg, nil
}

func checkCertExpiration(certData []byte) (bool, error) {
	cert, err := parseCertificate(certData)
	if err != nil {
		return false, fmt.Errorf("failed to parse certificate: %w", err)
	}

	expirationTime := cert.NotAfter
	currentTime := time.Now().UTC()
	// log.Log.Info("Checking Experation time", "Time: ", expirationTime)
	// log.Log.Info("Checking Current time", "Time: ", currentTime)
	if currentTime.After(expirationTime.UTC()) {
		// log.Log.Info("Certificate is expired")
		return true, nil
	} else {
		// log.Log.Info("Certificate is not expired")
		return false, nil
	}
}

func parseCertificate(certData []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM-encoded certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, nil
}
