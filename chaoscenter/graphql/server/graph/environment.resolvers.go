package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.
// Code generated by github.com/99designs/gqlgen version v0.17.49

import (
	"context"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/authorization"
	"github.com/sirupsen/logrus"
)

// CreateEnvironment is the resolver for the createEnvironment field.
func (r *mutationResolver) CreateEnvironment(ctx context.Context, projectID string, request *model.CreateEnvironmentRequest) (*model.Environment, error) {
	logFields := logrus.Fields{
		"projectId": projectID,
	}
	logrus.WithFields(logFields).Info("request received to create new environment")
	err := authorization.ValidateRole(ctx, projectID,
		authorization.MutationRbacRules[authorization.CreateEnvironment],
		model.InvitationAccepted.String())
	if err != nil {
		return nil, err
	}

	tkn := ctx.Value(authorization.AuthKey).(string)
	username, err := authorization.GetUsername(tkn)
	if err != nil {
		return nil, err
	}

	return r.environmentService.CreateEnvironment(ctx, projectID, request, username)
}

// UpdateEnvironment is the resolver for the updateEnvironment field.
func (r *mutationResolver) UpdateEnvironment(ctx context.Context, projectID string, request *model.UpdateEnvironmentRequest) (string, error) {
	logFields := logrus.Fields{
		"projectId":     projectID,
		"environmentId": request.EnvironmentID,
	}
	logrus.WithFields(logFields).Info("request received to update environment")
	err := authorization.ValidateRole(ctx, projectID,
		authorization.MutationRbacRules[authorization.UpdateEnvironment],
		model.InvitationAccepted.String())
	if err != nil {
		return "", err
	}

	tkn := ctx.Value(authorization.AuthKey).(string)
	username, err := authorization.GetUsername(tkn)
	if err != nil {
		return "", err
	}

	return r.environmentService.UpdateEnvironment(ctx, projectID, request, username)
}

// DeleteEnvironment is the resolver for the deleteEnvironment field.
func (r *mutationResolver) DeleteEnvironment(ctx context.Context, projectID string, environmentID string) (string, error) {
	logFields := logrus.Fields{
		"projectId":     projectID,
		"environmentId": environmentID,
	}
	logrus.WithFields(logFields).Info("request received to delete environment")
	err := authorization.ValidateRole(ctx, projectID,
		authorization.MutationRbacRules[authorization.DeleteEnvironment],
		model.InvitationAccepted.String())
	if err != nil {
		return "", err
	}

	tkn := ctx.Value(authorization.AuthKey).(string)
	username, err := authorization.GetUsername(tkn)
	if err != nil {
		return "", err
	}

	return r.environmentService.DeleteEnvironment(ctx, projectID, environmentID, username)
}

// GetEnvironment is the resolver for the getEnvironment field.
func (r *queryResolver) GetEnvironment(ctx context.Context, projectID string, environmentID string) (*model.Environment, error) {
	logFields := logrus.Fields{
		"projectId":     projectID,
		"environmentId": environmentID,
	}
	logrus.WithFields(logFields).Info("request received to get environment")
	err := authorization.ValidateRole(ctx, projectID,
		authorization.MutationRbacRules[authorization.GetEnvironment],
		model.InvitationAccepted.String())
	if err != nil {
		return nil, err
	}
	return r.environmentService.GetEnvironment(projectID, environmentID)
}

// ListEnvironments is the resolver for the listEnvironments field.
func (r *queryResolver) ListEnvironments(ctx context.Context, projectID string, request *model.ListEnvironmentRequest) (*model.ListEnvironmentResponse, error) {
	logFields := logrus.Fields{
		"projectId": projectID,
	}
	logrus.WithFields(logFields).Info("request received to list environments")
	err := authorization.ValidateRole(ctx, projectID,
		authorization.MutationRbacRules[authorization.ListEnvironments],
		model.InvitationAccepted.String())
	if err != nil {
		return nil, err
	}
	return r.environmentService.ListEnvironments(projectID, request)
}
