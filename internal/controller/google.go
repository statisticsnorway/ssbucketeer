package controller

import (
	"fmt"

	"google.golang.org/api/cloudidentity/v1"
)

type GoogleAuther struct {
	Ci *cloudidentity.Service
}

func (a *GoogleAuther) UserIsMemberOf(user, group string) (bool, error) {
	userEmail := fmt.Sprintf("%s@ssb.no", user)
	groupEmail := fmt.Sprintf("%s@groups.ssb.no", group)

	groupResponse, err := a.Ci.Groups.Lookup().GroupKeyId(groupEmail).Do()
	if err != nil {
		return false, err
	}

	isMemberResponse, err := a.Ci.Groups.Memberships.
		CheckTransitiveMembership(groupResponse.Name).
		Query(fmt.Sprintf("member_key_id == '%s'", userEmail)).Do()
	if err != nil {
		return false, err
	}

	return isMemberResponse.HasMembership, nil
}
