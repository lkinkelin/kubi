/*
Copyright 2024.

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

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/ca-gip/kubi/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// namespace where the project is deployed in
const namespace = "kube-system"

// serviceAccountName created for the project
const serviceAccountName = "test-kubebuilder-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "test-kubebuilder-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "test-kubebuilder-metrics-binding"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// installing CRDs, and deploying the controller.
	BeforeAll(func() {
		// By("creating manager namespace")
		// cmd := exec.Command("kubectl", "create", "ns", namespace)
		// _, err := utils.Run(cmd)
		// Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		// By("installing CRDs")
		// cmd = exec.Command("make", "install")
		// _, err = utils.Run(cmd)
		// Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		// By("deploying the controller-manager")
		// cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		// _, err = utils.Run(cmd)
		// Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		// By("cleaning up the curl pod for metrics")
		// cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		// _, _ = utils.Run(cmd)

		// By("undeploying the controller-manager")
		// cmd = exec.Command("make", "undeploy")
		// _, _ = utils.Run(cmd)

		// By("uninstalling CRDs")
		// cmd = exec.Command("make", "uninstall")
		// _, _ = utils.Run(cmd)

		// By("removing manager namespace")
		// cmd = exec.Command("kubectl", "delete", "ns", namespace)
		// _, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		// specReport := CurrentSpecReport()
		// if specReport.Failed() {
		// 	By("Fetching controller manager pod logs")
		// 	cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
		// 	controllerLogs, err := utils.Run(cmd)
		// 	if err == nil {
		// 		_, _ = fmt.Fprintf(GinkgoWriter, fmt.Sprintf("Controller logs:\n %s", controllerLogs))
		// 	} else {
		// 		_, _ = fmt.Fprintf(GinkgoWriter, fmt.Sprintf("Failed to get Controller logs: %s", err))
		// 	}

		// 	By("Fetching Kubernetes events")
		// 	cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
		// 	eventsOutput, err := utils.Run(cmd)
		// 	if err == nil {
		// 		_, _ = fmt.Fprintf(GinkgoWriter, fmt.Sprintf("Kubernetes events:\n%s", eventsOutput))
		// 	} else {
		// 		_, _ = fmt.Fprintf(GinkgoWriter, fmt.Sprintf("Failed to get Kubernetes events: %s", err))
		// 	}

		// 	By("Fetching curl-metrics logs")
		// 	cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
		// 	metricsOutput, err := utils.Run(cmd)
		// 	if err == nil {
		// 		_, _ = fmt.Fprintf(GinkgoWriter, fmt.Sprintf("Metrics logs:\n %s", metricsOutput))
		// 	} else {
		// 		_, _ = fmt.Fprintf(GinkgoWriter, fmt.Sprintf("Failed to get curl-metrics logs: %s", err))
		// 	}

		// 	By("Fetching controller manager pod description")
		// 	cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
		// 	podDescription, err := utils.Run(cmd)
		// 	if err == nil {
		// 		fmt.Println("Pod description:\n", podDescription)
		// 	} else {
		// 		fmt.Println("Failed to describe controller pod")
		// 	}
		// }
	})

	SetDefaultEventuallyTimeout(30 * time.Second) // 2 * time.Minute
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Kubi operator", func() {
		It("should run successfully", func() {
			By("validating that the kubi operator pod is running as expected")
			verifyKubiUp := func(g Gomega) {
				// Get the name of the kubi pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "app=kubi-operator",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve Kubi operator pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 Kubi operator pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("kubi-operator"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect Kubi operator pod status")
			}
			Eventually(verifyKubiUp).Should(Succeed())
		})

		It("should have created all appropriate objects (project, namespace, service account, rolebinding, network policies)", func() {
			By("validating that the kubi project has been created by kubi-operator")
			verifyTestKubiProjectHasBeenCreated := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"projects.cagip.github.com", "-l", "creator=kubi", "-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
				)

				projectsOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve projects")
				projectNames := utils.GetNonEmptyLines(projectsOutput)
				g.Expect(projectNames).To(HaveLen(1), "expected 1 Kubi project")
				controllerPodName = projectNames[0]
				g.Expect(controllerPodName).To(Equal("projet-toto-development"))

				cmd = exec.Command("kubectl", "get", "projects.cagip.github.com", "projet-toto-development", "-o", "jsonpath={.metadata.labels.creator}")
				projectOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to get project label creator")
				g.Expect(projectOutput).To(Equal("kubi"), "the label creator is not equal to kubi")

				cmd = exec.Command("kubectl", "get", "projects.cagip.github.com", "projet-toto-development", "-o", "jsonpath={.spec.environment}")
				projectOutput, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to get project spec environment")
				g.Expect(projectOutput).To(Equal("development"), "the spec environment is not equal to development")

				cmd = exec.Command("kubectl", "get", "projects.cagip.github.com", "projet-toto-development", "-o", "jsonpath={.spec.project}")
				projectOutput, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to get project spec project")
				g.Expect(projectOutput).To(Equal("projet-toto"), "the spec project is not equal to projet-toto")

				cmd = exec.Command("kubectl", "get", "projects.cagip.github.com", "projet-toto-development", "-o", "jsonpath={.spec.sourceEntity}")
				projectOutput, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to get project spec sourceEntity")
				g.Expect(projectOutput).To(Equal("DL_KUB_CAGIPHP_PROJET-TOTO-DEV_ADMIN"), "the spec sourceEntity is not equal to DL_KUB_CAGIPHP_PROJET-TOTO-DEV_ADMIN")

				cmd = exec.Command("kubectl", "get", "projects.cagip.github.com", "projet-toto-development", "-o", "jsonpath={.spec.stages}")
				projectOutput, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to get project spec stages")
				g.Expect(projectOutput).To(Equal("[\"scratch\",\"staging\",\"stable\"]"), "the spec stages is not equal to [\"scratch\",\"staging\",\"stable\"] ")

				cmd = exec.Command("kubectl", "get", "projects.cagip.github.com", "projet-toto-development", "-o", "jsonpath={.spec.tenant}")
				projectOutput, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to get project spec sourceEntity")
				g.Expect(projectOutput).To(Equal("cagip"), "the spec sourceEntity is not equal to cagip")

			}

			By("validating that the test namespace has been created by kubi-operator")
			verifyTestNamespaceHasBeenCreated := func(g Gomega) {
				// Get the name of the kubi pod
				cmd := exec.Command("kubectl", "get",
					"ns", "-l", "creator=kubi", "-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
				)

				namespaceOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve namespaces")
				namespaceNames := utils.GetNonEmptyLines(namespaceOutput)
				g.Expect(namespaceNames).To(HaveLen(1), "expected 1 Kubi namespace")
				controllerPodName = namespaceNames[0]
				g.Expect(controllerPodName).To(Equal("projet-toto-development"))

				cmd = exec.Command("kubectl", "get", "namespace", "projet-toto-development", "-o", "jsonpath={.metadata.labels.creator}")
				namespaceOutput, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to get namespace label creator")
				g.Expect(namespaceOutput).To(Equal("kubi"), "the label creator is not equal to kubi")

				cmd = exec.Command("kubectl", "get", "namespace", "projet-toto-development", "-o", "jsonpath={.metadata.labels.environment}")
				namespaceOutput, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to get namespace label environment")
				g.Expect(namespaceOutput).To(Equal("development"), "the label creator is not equal to development")

				// Could be useful
				// https://gist.github.com/PrasadG193/589975a55ed992a7b138a53c3d0d1836

				// Cannot parse because there are dots in the label
				// cmd = exec.Command("kubectl", "get", "namespace", "projet-toto-development", "-o", "jsonpath={.metadata.labels.pod-security.kubernetes.io/audit}")
				// namespaceOutput, err = utils.Run(cmd)
				// g.Expect(err).NotTo(HaveOccurred(), "Failed to get namespace label pod-security.kubernetes.io/audit")
				// g.Expect(namespaceOutput).To(Equal("restricted"), "the label pod-security.kubernetes.io/audit is not equal to restricted")

				// cmd = exec.Command("kubectl", "get", "namespace", "projet-toto-development", "-o", "jsonpath={.metadata.labels.pod-security.kubernetes.io/enforce}")
				// namespaceOutput, err = utils.Run(cmd)
				// g.Expect(err).NotTo(HaveOccurred(), "Failed to get namespace label pod-security.kubernetes.io/enforce")
				// g.Expect(namespaceOutput).To(Equal("baseline"), "the label pod-security.kubernetes.io/enforce is not equal to baseline")

				// cmd = exec.Command("kubectl", "get", "namespace", "projet-toto-development", "-o", "jsonpath={.metadata.labels.pod-security.kubernetes.io/warn}")
				// namespaceOutput, err = utils.Run(cmd)
				// g.Expect(err).NotTo(HaveOccurred(), "Failed to get namespace label pod-security.kubernetes.io/warn")
				// g.Expect(namespaceOutput).To(Equal("restricted"), "the label pod-security.kubernetes.io/warn is not equal to restricted")

				cmd = exec.Command("kubectl", "get", "namespace", "projet-toto-development", "-o", "jsonpath={.metadata.labels.quota}")
				namespaceOutput, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to get namespace label quota")
				g.Expect(namespaceOutput).To(Equal("managed"), "the label quota is not equal to managed")

				cmd = exec.Command("kubectl", "get", "namespace", "projet-toto-development", "-o", "jsonpath={.metadata.labels.type}")
				namespaceOutput, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to get namespace label type")
				g.Expect(namespaceOutput).To(Equal("customer"), "the label type is not equal to customer")

			}

			By("validating that the service account has been created by kubi-operator")
			verifyTestServiceAccountHasBeenCreated := func(g Gomega) {
				// Get the name of the kubi pod
				cmd := exec.Command("kubectl", "get", "sa", "-n", "projet-toto-development", "-l", "creator=kubi",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
				)

				saOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve service accounts")
				saNames := utils.GetNonEmptyLines(saOutput)
				g.Expect(saNames).To(HaveLen(1), "expected 1 sa created by kubi operator")
				controllerPodName = saNames[0]
				g.Expect(controllerPodName).To(Equal("service"))

				// cmd = exec.Command("kubectl", "get", "namespace", "projet-toto-development", "-o", "jsonpath={.metadata.labels.creator}")
				// saOutput, err = utils.Run(cmd)
				// g.Expect(err).NotTo(HaveOccurred(), "Failed to get namespace label creator")
				// g.Expect(saOutput).To(Equal("kubi"), "the label creator is not equal to kubi")

			}

			Eventually(verifyTestKubiProjectHasBeenCreated).Should(Succeed())
			Eventually(verifyTestNamespaceHasBeenCreated).Should(Succeed())
			Eventually(verifyTestServiceAccountHasBeenCreated).Should(Succeed())
			// Eventually(verifyTestRoleBindingHaveBeenCreated).Should(Succeed())

		})

		It("should watch the network policy config objects and create the network policies", func() {
			By("validating that kubi operator has created the default network policy in the test namespace")
			verifyNetworkPoliciesHaveBeenCreated := func(g Gomega) {
				// Get the name of the kubi pod

				// "kubectl", "get",
				// 	"pods", "-l", "app=kubi-operator",
				// 	"-o", "go-template={{ range .items }}"+
				// 		"{{ if not .metadata.deletionTimestamp }}"+
				// 		"{{ .metadata.name }}"+
				// 		"{{ \"\\n\" }}{{ end }}{{ end }}",
				// 	"-n", namespace,

				cmd := exec.Command("kubectl", "get",
					"networkpolicy", "kubi-default",
					"-n", "projet-toto-development", "-o", "go-template="+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}",
				)

				netpolOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve network policy kubi-default")
				netpolNames := utils.GetNonEmptyLines(netpolOutput)
				g.Expect(netpolNames).To(HaveLen(1), "expected 1 network policy") // pretty useless today as we 'kubectl get' one netpol in  particular, kubi-default. Could be useful, if later, we 'kubectl get' using a label selector, typically if more than one netpolconf is created.
				netpolName := netpolNames[0]
				g.Expect(netpolName).To(Equal("kubi-default"))

				// Parse the json and validate the rules inside the netpol
				cmd = exec.Command("kubectl", "get", "networkpolicy", "kubi-default", "-n",
					"projet-toto-development", "-o", "jsonpath={.spec.egress}",
				)
				netpolOutput, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to get netpol spec egress")
				//	expectedJsonBlock := '[{"ports":[{"port":636,"protocol":"UDP"},{"port":636,"protocol":"TCP"},{"port":389,"protocol":"UDP"},{"port":389,"protocol":"TCP"},{"port":123,"protocol":"UDP"},{"port":123,"protocol":"TCP"},{"port":53,"protocol":"UDP"},{"port":53,"protocol":"TCP"},{"port":53,"protocol":"UDP"}]},{"to":[{"podSelector":{}},{"namespaceSelector":{"matchLabels":{"name":"kube-system"}},"podSelector":{"matchLabels":{"component":"kube-apiserver","tier":"control-plane"}}},{"ipBlock":{"cidr":"172.20.0.0/16"}}]}]'
				g.Expect(netpolOutput).To(Equal("[{\"ports\":[{\"port\":636,\"protocol\":\"UDP\"},{\"port\":636,\"protocol\":\"TCP\"},{\"port\":389,\"protocol\":\"UDP\"},{\"port\":389,\"protocol\":\"TCP\"},{\"port\":123,\"protocol\":\"UDP\"},{\"port\":123,\"protocol\":\"TCP\"},{\"port\":53,\"protocol\":\"UDP\"},{\"port\":53,\"protocol\":\"TCP\"},{\"port\":53,\"protocol\":\"UDP\"}]},{\"to\":[{\"podSelector\":{}},{\"namespaceSelector\":{\"matchLabels\":{\"name\":\"kube-system\"}},\"podSelector\":{\"matchLabels\":{\"component\":\"kube-apiserver\",\"tier\":\"control-plane\"}}},{\"ipBlock\":{\"cidr\":\"172.20.0.0/16\"}}]}]"), "the spec egress of the network policy is not equal to what was requested in NetworkPolicyConfig object")

				// Parse the json and validate the rules inside the netpol
				cmd = exec.Command("kubectl", "get", "networkpolicy", "kubi-default", "-n",
					"projet-toto-development", "-o", "jsonpath={.spec.ingress}",
				)
				netpolOutput, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to get netpol spec ingress")
				g.Expect(netpolOutput).To(Equal("[{\"from\":[{\"podSelector\":{}},{\"namespaceSelector\":{\"matchLabels\":{\"name\":\"ingress-nginx\"}},\"podSelector\":{}},{\"namespaceSelector\":{\"matchLabels\":{\"name\":\"monitoring\"}},\"podSelector\":{}}]}]"), "the netpol spec ingress is not equal to what was configured in the networkPolicyConfig object")

				// Parse the json and validate the rules inside the netpol
				cmd = exec.Command("kubectl", "get", "networkpolicy", "kubi-default", "-n",
					"projet-toto-development", "-o", "jsonpath={.spec.podSelector}",
				)
				netpolOutput, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to get netpol spec podSelector")
				g.Expect(netpolOutput).To(Equal("{}"), "the netpol spec podSelector is not equal to what was configured in the networkPolicyConfig object")

				// Parse the json and validate the rules inside the netpol
				cmd = exec.Command("kubectl", "get", "networkpolicy", "kubi-default", "-n",
					"projet-toto-development", "-o", "jsonpath={.spec.policyTypes}",
				)
				netpolOutput, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to get netpol spec policyTypes")
				g.Expect(netpolOutput).To(Equal("[\"Ingress\",\"Egress\"]"), "the netpol spec policyTypes is not equal to what was configured in the networkPolicyConfig object")
			}
			Eventually(verifyNetworkPoliciesHaveBeenCreated).Should(Succeed())
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks

		// TODO: Customize the e2e test suite with scenarios specific to your project.
		// Consider applying sample/CR(s) and check their status and/or verifying
		// the reconciliation by using the metrics, i.e.:
		// metricsOutput := getMetricsOutput()
		// Expect(metricsOutput).To(ContainSubstring(
		//    fmt.Sprintf(`controller_runtime_reconcile_total{controller="%s",result="success"} 1`,
		//    strings.ToLower(<Kind>),
		// ))
	})
})

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest
		err = json.Unmarshal([]byte(output), &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() string {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
	Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
	return metricsOutput
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
