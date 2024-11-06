package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/api/cloudidentity/v1"
	"google.golang.org/api/option"
)

type ciTestServer struct {
	LookupFail     bool
	HasMembership  bool
	MembershipFail bool
}

func (s *ciTestServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var b []byte
	var err error
	method := strings.Split(r.URL.Path, ":")[1]
	switch method {
	case "lookup":
		if s.LookupFail {
			http.Error(w, "error", http.StatusForbidden)
			return
		}
		resp := &cloudidentity.LookupGroupNameResponse{
			Name: "dummy",
		}
		b, err = json.Marshal(resp)
	case "checkTransitiveMembership":
		if s.MembershipFail {
			http.Error(w, "error", http.StatusForbidden)
		}
		resp := &cloudidentity.CheckTransitiveMembershipResponse{
			HasMembership: s.HasMembership,
		}
		b, err = json.Marshal(resp)
	}
	if err != nil {
		http.Error(w, "unable to marshal request: "+err.Error(), http.StatusBadRequest)
		return
	}
	w.Write(b)
}

func TestIsMemberOf(t *testing.T) {
	tests := map[string]struct {
		LookupFail     bool
		HasMembership  bool
		MembershipFail bool
	}{
		"User should be member": {
			HasMembership: true,
		},
		"Group lookup should fail": {
			LookupFail:    true,
			HasMembership: true,
		},
		"Membership check should fail": {
			MembershipFail: true,
			HasMembership:  true,
		},
		"User should not be member": {
			HasMembership: false,
		},
	}

	ctx := context.Background()

	testServer := &ciTestServer{}
	cis := httptest.NewServer(testServer)
	defer cis.Close()

	ciService, err := cloudidentity.NewService(ctx, option.WithoutAuthentication(), option.WithEndpoint(cis.URL))
	if err != nil {
		t.Fatalf("unable to create cloudidentity client: %v", err)
	}

	gAuth := GoogleAuther{ciService}

	for desc, test := range tests {
		testServer.HasMembership = test.HasMembership
		testServer.LookupFail = test.LookupFail
		testServer.MembershipFail = test.MembershipFail

		isMember, err := gAuth.UserIsMemberOf("dummy", "dummy")

		shouldErr := test.MembershipFail || test.LookupFail
		expectedIsMember := !shouldErr && test.HasMembership

		if isMember != expectedIsMember {
			t.Errorf("%s: isMember=%v, expected %v", desc, isMember, expectedIsMember)
		}

		if shouldErr && err == nil || !shouldErr && err != nil {
			t.Errorf("%s: err=%v", desc, err)
		}
	}
}
