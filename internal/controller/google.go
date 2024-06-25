package controller

import (
	"context"
	"fmt"

	"google.golang.org/api/cloudidentity/v1"
)

type GoogleAuther struct {
	ci *cloudidentity.Service
}

func NewGoogleAuth() (*GoogleAuther, error) {
	ciService, err := cloudidentity.NewService(context.Background())
	if err != nil {
		return nil, err
	}

	return &GoogleAuther{ci: ciService}, nil
}

func (a *GoogleAuther) UserIsMemberOf(user, group string) (bool, error) {
	userEmail := fmt.Sprintf("%s@ssb.no", user)
	groupEmail := fmt.Sprintf("%s@groups.ssb.no", group)

	groupResponse, err := a.ci.Groups.Lookup().GroupKeyId(groupEmail).Do()
	if err != nil {
		return false, err
	}

	isMemberResponse, err := a.ci.Groups.Memberships.
		CheckTransitiveMembership(groupResponse.Name).
		Query(fmt.Sprintf("member_key_id == '%s'", userEmail)).Do()
	if err != nil {
		return false, err
	}

	return isMemberResponse.HasMembership, nil
}
