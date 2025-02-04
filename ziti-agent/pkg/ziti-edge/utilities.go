package zitiedge

import (
	"fmt"
	"strings"

	"github.com/openziti/edge-api/rest_management_api_client"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// func CreateAndEnrollIdentity(name string, podPrefix string, uid types.UID, roles []string, zec *rest_management_api_client.ZitiEdgeManagement) (*ziti.Config, string, error) {
func GetIdentityToken(name string, podPrefix string, uid types.UID, roles []string, zec *rest_management_api_client.ZitiEdgeManagement) (string, string, error) {
	identityName := fmt.Sprintf("%s-%s%s", trimString(name), podPrefix, uid)

	identityDetails, err := CreateIdentity(identityName, roles, "Device", zec)
	if err != nil {
		klog.Error(err)
		// return nil, identityName, err
		return "", identityName, err
	}

	identityToken, err := GetIdentityById(identityDetails.GetPayload().Data.ID, zec)
	if err != nil {
		klog.Error(err)
		return "", identityName, err
	}
	return identityToken.GetPayload().Data.Enrollment.Ott.JWT, identityName, nil
}

func FindIdentity(name string, zec *rest_management_api_client.ZitiEdgeManagement) (string, bool, error) {

	var zId string = ""
	// klog.Infof(fmt.Sprintf("Deleting Ziti Identity"))

	identityDetails, err := GetIdentityByName(name, zec)
	if err != nil {
		klog.Error(err)
		return zId, false, err
	}

	for _, identityItem := range identityDetails.GetPayload().Data {
		zId = *identityItem.ID
	}

	if len(zId) > 0 {
		klog.Infof("Identity %s was found", zId)
		return zId, true, nil
	}

	return zId, false, nil
}

func GetIdentityAttributes(roles map[string]string, key string) ([]string, bool) {
	// if a ziti role key is not present, use app name as a role attribute
	value, ok := roles[key]
	if ok {
		if len(value) > 0 {
			return strings.Split(value, ","), true
		}
	}
	return []string{}, false
}

func trimString(input string) string {
	if len(input) > 24 {
		return input[:24]
	}
	return input
}
