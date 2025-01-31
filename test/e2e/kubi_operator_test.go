package e2e

import (
	"fmt"
	"testing"
)

func TestMain(m *testing.M) {
	//FIXTURES (AKA SETUP) . Kubectl apply of everything should probably be called here .
	fmt.Sprintf("Executing fixtures")

	t := &testing.T{}

	// Calling other tests
	TestResourcesAreCreatedWhenADStuffIsPresent(t)
	TestNetworkPoliciesAreCreatedWhenNetworkPolicyConfigIsPresent(t)

	//CLEANUP (AKA TEARDOWN)

}

// Fat test which will check that, when the connected OpenLDAP is populated with appropriate AD Groups and users, the following stuff is created:
// * Project resources are created
// * namespace is present
// * namespace has appropriate labels
// * service account 'service' is present in namespace
func TestResourcesAreCreatedWhenADStuffIsPresent(t *testing.T) {
	//FIXTURES (AKA SETUP)
	print("other tests")

	// test

	//CLEANUP (AKA TEARDOWN)

}

func TestNetworkPoliciesAreCreatedWhenNetworkPolicyConfigIsPresent(t *testing.T) {
	//FIXTURES (AKA SETUP)
	print("other tests")

	// test

	//CLEANUP (AKA TEARDOWN)

}

//1er test: TestKubiProjectsAreCreated
// quand y a les bons groupes AD, les resources Project sont créées

// 2e test : TestNamespacesAreCreated
// quand y a les bons groupes AD, les namespaces

// 3e test : TestNetworkPoliciesAreCreated
// quand y a les bons groupes AD et la NetworkPolicyConfig de présente, Les networkPolicies sont créées dans les namespaces

//3e test
