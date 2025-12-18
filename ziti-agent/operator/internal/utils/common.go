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

package utils

import (
	"context"
	"fmt"
	"net/url"
	"reflect"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/golang-jwt/jwt/v5"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// DeepEqualExcludingFields compares two structs for deep equality, excluding specified fields.
// It returns true if the structs are equal except for the excluded fields.
func DeepEqualExcludingFields(a, b interface{}, excludeFields ...string) bool {
	va := reflect.ValueOf(a)
	vb := reflect.ValueOf(b)

	if va.Type() != vb.Type() {
		return false
	}

	// Create copies
	copyA := reflect.New(va.Type()).Elem()
	copyB := reflect.New(vb.Type()).Elem()
	copyA.Set(va)
	copyB.Set(vb)

	// Clear excluded fields
	for _, field := range excludeFields {
		if fieldA := copyA.FieldByName(field); fieldA.IsValid() && fieldA.CanSet() {
			fieldA.Set(reflect.Zero(fieldA.Type()))
		}
		if fieldB := copyB.FieldByName(field); fieldB.IsValid() && fieldB.CanSet() {
			fieldB.Set(reflect.Zero(fieldB.Type()))
		}
	}

	return reflect.DeepEqual(copyA.Interface(), copyB.Interface())
}

func MergeSpecs(ctx context.Context, current, desired any) (error, bool) {
	log := log.FromContext(ctx)
	ok := false
	log.V(4).Info("Merging Structs", "Current", current, "Desired", desired)

	currentVal := reflect.ValueOf(current)
	desiredVal := reflect.ValueOf(desired)

	if currentVal.Kind() != reflect.Ptr {
		log.V(5).Info("Current is not a pointer, creating a pointer to it")
		currentPtr := reflect.New(currentVal.Type())
		currentPtr.Elem().Set(currentVal)
		currentVal = currentPtr
	}
	if desiredVal.Kind() != reflect.Ptr {
		log.V(5).Info("Desired is not a pointer, creating a pointer to it")
		desiredPtr := reflect.New(desiredVal.Type())
		desiredPtr.Elem().Set(desiredVal)
		desiredVal = desiredPtr
	}

	for i := range currentVal.Elem().NumField() {
		log.V(5).Info("Setting fields", "Field", currentVal.Elem().Type().Field(i).Name, "Value", currentVal.Elem().Field(i))

		if IsManagedField(ctx, currentVal.Elem().Type().Field(i).Name) {
			log.V(5).Info("IsMangedField", "Field", currentVal.Elem().Type().Field(i).Name, "Value", currentVal.Elem().Field(i).Interface())

			if IsZeroValue(ctx, currentVal.Elem().Field(i)) && !IsZeroValue(ctx, desiredVal.Elem().Field(i)) {

				log.V(5).Info("Current fields", "Field", currentVal.Elem().Type().Field(i).Name, "Value", currentVal.Elem().Field(i).Interface())
				log.V(5).Info("Desired fields", "Field", desiredVal.Elem().Type().Field(i).Name, "Value", desiredVal.Elem().Field(i).Interface())
				if currentVal.Elem().Field(i).CanSet() {
					currentVal.Elem().Field(i).Set(desiredVal.Elem().Field(i))
					ok = true
				} else {
					return fmt.Errorf("cannot set field %s", currentVal.Elem().Type().Field(i).Name), ok
				}
			}

			if !IsZeroValue(ctx, currentVal.Elem().Field(i)) && !IsZeroValue(ctx, desiredVal.Elem().Field(i)) {

				currentSubVal := currentVal.Elem().Field(i).Addr()
				desiredSubVal := desiredVal.Elem().Field(i).Addr()

				if currentSubVal.Elem().Kind() == reflect.Struct {
					for j := range currentSubVal.Elem().NumField() {
						log.V(5).Info("Setting subFields", "SubField", currentSubVal.Elem().Type().Field(j).Name, "Value", currentSubVal.Elem().Field(j).Interface())
						log.V(5).Info("Setting subFields", "SubField", desiredSubVal.Elem().Type().Field(j).Name, "Value", desiredSubVal.Elem().Field(j).Interface())
						if IsZeroValue(ctx, currentSubVal.Elem().Field(j)) && !IsZeroValue(ctx, desiredSubVal.Elem().Field(j)) {
							if currentSubVal.Elem().Field(j).CanSet() {
								currentSubVal.Elem().Field(j).Set(desiredSubVal.Elem().Field(j))
								ok = true
							}
						}

						currentSub2Val := currentSubVal.Elem().Field(j).Addr()
						desiredSub2Val := desiredSubVal.Elem().Field(j).Addr()

						if currentSub2Val.Elem().Kind() == reflect.Struct {
							for k := range currentSub2Val.Elem().NumField() {
								log.V(5).Info("Setting sub2Fields", "Sub2Field", currentSub2Val.Elem().Type().Field(k).Name, "Value", currentSub2Val.Elem().Field(k).Interface())
								log.V(5).Info("Setting sub2Fields", "Sub2Field", desiredSub2Val.Elem().Type().Field(k).Name, "Value", desiredSub2Val.Elem().Field(k).Interface())
								if IsZeroValue(ctx, currentSub2Val.Elem().Field(k)) && !IsZeroValue(ctx, desiredSub2Val.Elem().Field(k)) {
									if currentSub2Val.Elem().Field(k).CanSet() {
										currentSub2Val.Elem().Field(k).Set(desiredSub2Val.Elem().Field(k))
										log.V(4).Info("Setting sub2Fields", "Sub2Field", currentSub2Val.Elem().Type().Field(k).Name, "Value", currentSub2Val.Elem().Field(k).Interface())
										log.V(4).Info("Setting sub2Fields", "Sub2Field", desiredSub2Val.Elem().Type().Field(k).Name, "Value", desiredSub2Val.Elem().Field(k).Interface())
										ok = true
									}
								}

								currentSub3Val := currentSub2Val.Elem().Field(k).Addr()
								desiredSub3Val := desiredSub2Val.Elem().Field(k).Addr()

								if currentSub3Val.Elem().Kind() == reflect.Struct {
									for l := range currentSub3Val.Elem().NumField() {
										log.V(5).Info("Setting sub3Fields", "Sub3Field", currentSub3Val.Elem().Type().Field(l).Name, "Value", currentSub3Val.Elem().Field(l).Interface())
										log.V(5).Info("Setting sub3Fields", "Sub3Field", desiredSub3Val.Elem().Type().Field(l).Name, "Value", desiredSub3Val.Elem().Field(l).Interface())
										if IsZeroValue(ctx, currentSub3Val.Elem().Field(l)) && !IsZeroValue(ctx, desiredSub3Val.Elem().Field(l)) {
											if currentSub3Val.Elem().Field(l).CanSet() {
												currentSub3Val.Elem().Field(l).Set(desiredSub3Val.Elem().Field(l))
												log.V(4).Info("Setting sub3Fields", "Sub3Field", currentSub3Val.Elem().Type().Field(l).Name, "Value", currentSub3Val.Elem().Field(l).Interface())
												log.V(4).Info("Setting sub3Fields", "Sub3Field", desiredSub3Val.Elem().Type().Field(l).Name, "Value", desiredSub3Val.Elem().Field(l).Interface())
												ok = true
											}
										}
									}
								}

							}
						}
					}
				}

			}
		}
	}
	log.V(5).Info("Merged Structs", "Current", currentVal.Elem().Interface(), "Desired", desiredVal.Elem().Interface())
	return nil, ok
}

func IsManagedField(ctx context.Context, fieldName string) bool {
	_ = log.FromContext(ctx)
	switch fieldName {
	case "Name", "ZitiControllerName", "Cert", "DeploymentSpec", "MutatingWebhookSpec", "ClusterRoleSpec", "ServiceAccount", "Revision":
		return true
	case "ZitiCtrlMgmtApi", "Model", "Config", "Deployment":
		return true
	default:
		return false
	}
}

func IsZeroValue(ctx context.Context, v reflect.Value) bool {
	_ = log.FromContext(ctx)
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	case reflect.Slice, reflect.Map, reflect.Chan:
		return v.IsNil() || v.Len() == 0
	case reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() < 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Complex64, reflect.Complex128:
		return v.Complex() == 0
	case reflect.Struct:
		return reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
	default:
		return v.IsZero()
	}
}

func FilterLabels(allLabels map[string]string) map[string]string {
	filtered := make(map[string]string)
	if val, ok := allLabels["app"]; ok {
		filtered["app"] = val
	}
	if val, ok := allLabels["app.kubernetes.io/name"]; ok {
		filtered["app.kubernetes.io/name"] = val
	}
	return filtered
}

func ConvertDeploymentConditions(conds []appsv1.DeploymentCondition) []appsv1.DeploymentCondition {
	result := make([]appsv1.DeploymentCondition, 0, len(conds))
	for _, c := range conds {
		result = append(result, appsv1.DeploymentCondition{
			Type:               appsv1.DeploymentConditionType(c.Type),
			Status:             corev1.ConditionStatus(c.Status),
			LastTransitionTime: c.LastTransitionTime,
			LastUpdateTime:     c.LastUpdateTime,
			Reason:             c.Reason,
			Message:            c.Message,
		})
	}
	return result
}

func ConvertIssuerConditions(conds []certmanagerv1.IssuerCondition) []certmanagerv1.IssuerCondition {
	result := make([]certmanagerv1.IssuerCondition, 0, len(conds))
	for _, c := range conds {
		result = append(result, certmanagerv1.IssuerCondition{
			Type:               certmanagerv1.IssuerConditionType(c.Type),
			Status:             c.Status,
			LastTransitionTime: c.LastTransitionTime,
			Reason:             c.Reason,
			Message:            c.Message,
		})
	}
	return result
}

func ConvertCertificateConditions(conds []certmanagerv1.CertificateCondition) []certmanagerv1.CertificateCondition {
	result := make([]certmanagerv1.CertificateCondition, 0, len(conds))
	for _, c := range conds {
		result = append(result, certmanagerv1.CertificateCondition{
			Type:               certmanagerv1.CertificateConditionType(c.Type),
			Status:             c.Status,
			LastTransitionTime: c.LastTransitionTime,
			Reason:             c.Reason,
			Message:            c.Message,
		})
	}
	return result
}

func DecodeJWT(tokenString string) (jwt.MapClaims, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("failed to extract claims from JWT")
	}

	return claims, nil
}

func GetUrlFromJwt(tokenString string) (string, error) {

	claims, err := DecodeJWT(tokenString)
	if err != nil {
		return "", err
	}
	issuer, err := claims.GetIssuer()
	if err != nil {
		return "", err
	}
	return issuer, nil
}

func GetHostAndPort(urlString string) (string, string, error) {
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse URL: %w", err)
	}

	host := parsedURL.Hostname()
	port := parsedURL.Port()

	if port == "" {
		switch parsedURL.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}

	return host, port, nil
}
