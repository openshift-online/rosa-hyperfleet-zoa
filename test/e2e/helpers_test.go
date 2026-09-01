//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/gomega" //nolint:staticcheck // dot-import is the Gomega/Ginkgo convention
)

// target is one ZOA Lambda deployment (RC or MC) under test. RC and MC are
// separate AWS accounts — each has its own IAM principal allowed to invoke
// its Lambda Function URL, so every subprocess call must be scoped to the
// right AWS_PROFILE for the target it's actually talking to.
type target struct {
	Name       string // "rc" or "mc"
	APIURL     string
	AWSProfile string
}

var (
	jiraTicket = envOrDefault("E2E_JIRA_TICKET", "ZOAE2E-1")
	zoaBin     = envOrDefault("ZOA_BIN", "zoa")

	// coredns is the standard EKS system Deployment used for delete_pod and
	// rollout_restart real (non-dry-run) tests. Pods are always owned by a
	// ReplicaSet, so the TA's ownerReferences safety check passes and the
	// controller recreates deleted pods automatically.
	coreDNSNamespace = envOrDefault("E2E_COREDNS_NAMESPACE", "kube-system")
	coreDNSName      = envOrDefault("E2E_COREDNS_NAME", "coredns")
	coreDNSSelector  = envOrDefault("E2E_COREDNS_SELECTOR", "k8s-app=kube-dns")

	targets = discoverTargets()
)

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// discoverTargets reads ZOA_RC_API_URL / ZOA_MC_API_URL from the environment.
// Either, both, or neither may be set — the suite runs the same spec set
// against whichever targets are present so it can validate a single cluster
// in isolation (e.g. `zoa run` against a dev env) or both at once (CI).
func discoverTargets() []target {
	var out []target
	if url := os.Getenv("ZOA_RC_API_URL"); url != "" {
		out = append(out, target{Name: "RC (Regional Cluster)", APIURL: url, AWSProfile: envOrDefault("ZOA_RC_AWS_PROFILE", "rrp-rc")})
	}
	if url := os.Getenv("ZOA_MC_API_URL"); url != "" {
		out = append(out, target{Name: "MC (Management Cluster)", APIURL: url, AWSProfile: envOrDefault("ZOA_MC_AWS_PROFILE", "rrp-mc")})
	}
	return out
}

// runZoa runs the zoa CLI against tgt, scoping ZOA_API_URL/AWS_PROFILE to
// this subprocess's environment only. It deliberately never mutates the
// parent process's environment (os.Setenv) so RC and MC specs can safely run
// in parallel without racing on which profile is "current".
//
// We filter AWS_PROFILE from the inherited environment before setting our own
// to avoid relying on exec.Cmd's "last value wins" behavior for duplicate keys.
func runZoa(tgt target, args ...string) (string, error) {
	cmd := exec.Command(zoaBin, args...) //nolint:gosec // test helper, args are test-controlled
	cmd.Env = append(filterEnv(os.Environ(), "AWS_PROFILE", "ZOA_API_URL"),
		"ZOA_API_URL="+tgt.APIURL,
		"AWS_PROFILE="+tgt.AWSProfile,
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// extractJSON extracts the JSON value from CLI output that may contain
// human-readable prefix lines (status indicators like "✓ <id> [cluster]").
// The CLI outputs these even with -o json.
//
// We look for '{' at the start of a line (after newline or at position 0),
// NOT just any '{' or '[' — because status lines contain brackets like
// "[eph-xxx-regional]" which are not JSON.
func extractJSON(output string) string {
	// Look for '{\n' pattern (JSON object starting a line) — this is the most common case
	// Check if output starts with '{'
	if len(output) > 0 && output[0] == '{' {
		return output
	}

	// Look for '\n{' (JSON object after a newline)
	idx := strings.Index(output, "\n{")
	if idx >= 0 {
		return output[idx+1:] // skip the newline, return from '{'
	}

	// Fallback: look for '\n[' for JSON arrays starting a line
	idx = strings.Index(output, "\n[")
	if idx >= 0 {
		return output[idx+1:]
	}

	// Last resort: just find first '{' anywhere (original behavior)
	idx = strings.Index(output, "{")
	if idx >= 0 {
		return output[idx:]
	}

	return output
}

// filterEnv returns a copy of env with the specified keys removed.
func filterEnv(env []string, keys ...string) []string {
	keySet := make(map[string]bool, len(keys))
	for _, k := range keys {
		keySet[k+"="] = true
	}
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		skip := false
		for prefix := range keySet {
			if strings.HasPrefix(e, prefix) {
				skip = true
				break
			}
		}
		if !skip {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// runActionResult dispatches a Trusted Action via `zoa run` and returns the
// parsed client.Execution JSON (as a generic map, so this suite stays
// decoupled from internal/client's Go types), or an error if the CLI call
// failed, the output wasn't valid JSON, or the execution didn't succeed.
// Unlike runAction, it never asserts — use it from Eventually() polling
// loops where a transient failure (e.g. a Lambda cold start) should be
// retried rather than hard-failing the spec immediately.
func runActionResult(tgt target, action string, extra ...string) (map[string]interface{}, error) {
	args := append([]string{"run", action, "--jira", jiraTicket}, extra...)
	args = append(args, "-o", "json")

	out, err := runZoa(tgt, args...)
	if err != nil {
		return nil, fmt.Errorf("[%s] zoa run %s %v failed: %w:\n%s", tgt.Name, action, extra, err, out)
	}

	// CLI outputs human-readable status lines before JSON even with -o json
	jsonStr := extractJSON(out)

	var exec map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(jsonStr), &exec); jsonErr != nil {
		return nil, fmt.Errorf("[%s] invalid JSON from zoa run %s: %w:\njsonStr=%q\nraw output:\n%s", tgt.Name, action, jsonErr, jsonStr, out)
	}
	if exec["status"] != "succeeded" {
		return nil, fmt.Errorf("[%s] zoa run %s did not succeed:\n%s", tgt.Name, action, out)
	}
	return exec, nil
}

// runAction dispatches a Trusted Action expected to succeed and fails the
// calling spec immediately if it didn't. See runActionResult for a
// non-asserting variant.
func runAction(tgt target, action string, extra ...string) map[string]interface{} {
	exec, err := runActionResult(tgt, action, extra...)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return exec
}

// runActionExpectFailure dispatches a Trusted Action expected to fail
// (parameter validation, RBAC denial, HCP namespace protection, ...) and
// returns the combined CLI output for substring assertions on the error.
func runActionExpectFailure(tgt target, action string, extra ...string) string {
	args := append([]string{"run", action, "--jira", jiraTicket}, extra...)
	out, err := runZoa(tgt, args...)
	ExpectWithOffset(1, err).To(HaveOccurred(), "zoa run %s %v unexpectedly succeeded:\n%s", action, extra, out)
	return out
}

// outputArray extracts the "output" field of a parsed Execution map as a
// []interface{}, treating a JSON null (server returned zero rows) as an
// empty slice rather than a type-assertion failure.
func outputArray(exec map[string]interface{}) []interface{} {
	v := exec["output"]
	if v == nil {
		return nil
	}
	arr, ok := v.([]interface{})
	ExpectWithOffset(1, ok).To(BeTrue(), "expected output to be a JSON array, got %T: %v", v, v)
	return arr
}

// outputMap extracts the "output" field of a parsed Execution map as a
// map[string]interface{}.
func outputMap(exec map[string]interface{}) map[string]interface{} {
	v := exec["output"]
	m, ok := v.(map[string]interface{})
	ExpectWithOffset(1, ok).To(BeTrue(), "expected output to be a JSON object, got %T: %v", v, v)
	return m
}

// coreDNSPodNames lists current coredns pod names via get_resource — always
// live state, never an assumed/cached count. Fails the calling spec
// immediately on error; use coreDNSPodNamesOrEmpty inside Eventually loops.
func coreDNSPodNames(tgt target) []string {
	exec := runAction(tgt, "get_resource", "--resource", "pods", "--namespace", coreDNSNamespace, "--selector", coreDNSSelector)
	return podNamesFromOutput(exec["output"])
}

// coreDNSPodNamesOrEmpty is the non-asserting counterpart of
// coreDNSPodNames, returning an empty slice on any error so Eventually()
// treats a transient failure as "not there yet" instead of aborting the
// spec.
func coreDNSPodNamesOrEmpty(tgt target) []string {
	exec, err := runActionResult(tgt, "get_resource", "--resource", "pods", "--namespace", coreDNSNamespace, "--selector", coreDNSSelector)
	if err != nil {
		return nil
	}
	return podNamesFromOutput(exec["output"])
}

func podNamesFromOutput(output interface{}) []string {
	rows, ok := output.([]interface{})
	if !ok {
		return nil
	}
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		row, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		if name, ok := caseInsensitiveField(row, "name"); ok {
			names = append(names, name)
		}
	}
	return names
}

// caseInsensitiveField looks up key in a row map without hardcoding a
// casing convention: get_resource's rows come straight from the Kubernetes
// server-side Table API (columns like "Name"), which is a K8s API server
// detail, not something ZOA's own code controls.
func caseInsensitiveField(row map[string]interface{}, key string) (string, bool) {
	for k, v := range row {
		if strings.EqualFold(k, key) {
			return fmt.Sprintf("%v", v), true
		}
	}
	return "", false
}
