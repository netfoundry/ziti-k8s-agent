package zitiedge

import (
	"context"
	"fmt"
	"time"

	"github.com/openziti/edge-api/rest_management_api_client"
	"github.com/openziti/edge-api/rest_management_api_client/edge_router"
	rest_model_edge "github.com/openziti/edge-api/rest_model"
	"github.com/openziti/sdk-golang/ziti"
	"github.com/openziti/sdk-golang/ziti/enroll"
)

func CreateEdgeRouter(name string, options *rest_model_edge.EdgeRouterCreate, edge *rest_management_api_client.ZitiEdgeManagement) (*edge_router.CreateEdgeRouterCreated, error) {
	req := edge_router.NewCreateEdgeRouterParams()
	req.EdgeRouter = options
	req.SetTimeout(30 * time.Second)
	resp, err := edge.EdgeRouter.CreateEdgeRouter(req, nil)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func PatchEdgeRouter(zId string, roleAttributes rest_model_edge.Attributes, edge *rest_management_api_client.ZitiEdgeManagement) (*edge_router.PatchEdgeRouterOK, error) {
	req := edge_router.PatchEdgeRouterParams{
		ID: zId,
		EdgeRouter: &rest_model_edge.EdgeRouterPatch{
			RoleAttributes: &roleAttributes,
		},
	}
	resp, err := edge.EdgeRouter.PatchEdgeRouter(&req, nil)
	if err != nil {
		return nil, err
	}
	return resp, err
}

func GetEdgeRouterByName(name string, edge *rest_management_api_client.ZitiEdgeManagement) (*edge_router.ListEdgeRoutersOK, error) {
	filter := fmt.Sprintf("name=\"%v\"", name)
	limit := int64(0)
	offset := int64(0)
	req := &edge_router.ListEdgeRoutersParams{
		Filter:  &filter,
		Limit:   &limit,
		Offset:  &offset,
		Context: context.Background(),
	}
	req.SetTimeout(30 * time.Second)
	resp, err := edge.EdgeRouter.ListEdgeRouters(req, nil)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func GetEdgeRouterDetail(zId string, edge *rest_management_api_client.ZitiEdgeManagement) (*edge_router.DetailEdgeRouterOK, error) {
	p := &edge_router.DetailEdgeRouterParams{
		Context: context.Background(),
		ID:      zId,
	}
	p.SetTimeout(30 * time.Second)
	resp, err := edge.EdgeRouter.DetailEdgeRouter(p, nil)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func EnrollEdgeRouter(zId string, edge *rest_management_api_client.ZitiEdgeManagement) (*ziti.Config, error) {
	p := &edge_router.DetailEdgeRouterParams{
		Context: context.Background(),
		ID:      zId,
	}
	p.SetTimeout(30 * time.Second)
	resp, err := edge.EdgeRouter.DetailEdgeRouter(p, nil)
	if err != nil {
		return nil, err
	}
	tkn, _, err := enroll.ParseToken(*resp.GetPayload().Data.EnrollmentJWT)
	if err != nil {
		return nil, err
	}
	flags := enroll.EnrollmentFlags{
		Token:  tkn,
		KeyAlg: "RSA",
	}
	conf, err := enroll.Enroll(flags)
	if err != nil {
		return nil, err
	}
	return conf, nil
}

func ReEnrollEdgeRouter(zId string, edge *rest_management_api_client.ZitiEdgeManagement) (string, error) {
	p := &edge_router.ReEnrollEdgeRouterParams{
		Context: context.Background(),
		ID:      zId,
	}
	p.SetTimeout(30 * time.Second)
	_, err := edge.EdgeRouter.ReEnrollEdgeRouter(p, nil)
	if err != nil {
		return "", err
	}
	return "", nil
}

func DeleteEdgeRouter(zId string, edge *rest_management_api_client.ZitiEdgeManagement) error {
	req := &edge_router.DeleteEdgeRouterParams{
		ID:      zId,
		Context: context.Background(),
	}
	req.SetTimeout(30 * time.Second)
	_, err := edge.EdgeRouter.DeleteEdgeRouter(req, nil)
	if err != nil {
		return err
	}
	return nil
}
