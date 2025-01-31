package e2e

import "testing"

func TestMain(m *testing.M) {
	//FIXTURES (AKA SETUP)
	print("helloworld")
	// Calling of subtests by t.Run()

	//CLEANUP (AKA TEARDOWN)

}

func TestProjetResourcesAreCreated(t *testing.T) {
	//FIXTURES (AKA SETUP)

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
