//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// E2E conformance — dynamically ensures every Trusted Action reported by
// the live Lambda has e2e coverage. No hardcoded TA lists to maintain.

var _ = Describe("e2e conformance", func() {
	// liveActions queries a target's Lambda for the full TA inventory.
	// Uses the first available target (RC or MC — the registry is identical).
	liveActions := func() []string {
		Expect(targets).NotTo(BeEmpty(), "no targets configured")
		tgt := targets[0]

		out, err := runZoa(tgt, "actions", "-o", "json")
		Expect(err).NotTo(HaveOccurred(), out)

		jsonStr := extractJSON(out)
		var list struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
		}
		Expect(json.Unmarshal([]byte(jsonStr), &list)).To(Succeed(), out)

		names := make([]string, 0, len(list.Items))
		for _, a := range list.Items {
			names = append(names, a.Name)
		}
		return names
	}

	It("every registered TA has a ta_* e2e test file", func() {
		testDir := "."
		if _, err := os.Stat("test/e2e"); err == nil {
			testDir = "test/e2e"
		}

		taFiles, _ := filepath.Glob(filepath.Join(testDir, "ta_*_test.go"))
		taFileContents := make(map[string]string, len(taFiles))
		for _, f := range taFiles {
			data, _ := os.ReadFile(f)
			taFileContents[filepath.Base(f)] = string(data)
		}

		for _, ta := range liveActions() {
			found := false
			for file, content := range taFileContents {
				if strings.Contains(content, ta) {
					found = true
					_ = file
					break
				}
			}
			Expect(found).To(BeTrue(),
				"TA %q is registered in the Lambda but has no ta_*_test.go e2e coverage — "+
					"add a test file or add the TA name to an existing ta_* file", ta)
		}
	})

	It("knownActions list matches the live Lambda registry", func() {
		live := liveActions()

		for _, want := range knownActions {
			Expect(live).To(ContainElement(want),
				"knownActions has %q but it's not in the live registry — remove it from cmd_actions_test.go", want)
		}
		for _, got := range live {
			found := false
			for _, known := range knownActions {
				if known == got {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(),
				"TA %q is live but not in knownActions — add it to cmd_actions_test.go", got)
		}
	})

	It("smoke tests cover both kube-api and aws-api scopes", func() {
		testDir := "."
		if _, err := os.Stat("test/e2e"); err == nil {
			testDir = "test/e2e"
		}

		// Query live scopes for each TA
		Expect(targets).NotTo(BeEmpty())
		tgt := targets[0]

		out, err := runZoa(tgt, "actions", "-o", "json")
		Expect(err).NotTo(HaveOccurred(), out)

		jsonStr := extractJSON(out)
		var list struct {
			Items []struct {
				Name  string `json:"name"`
				Scope string `json:"scope"`
			} `json:"items"`
		}
		Expect(json.Unmarshal([]byte(jsonStr), &list)).To(Succeed(), out)

		taScope := make(map[string]string, len(list.Items))
		for _, a := range list.Items {
			taScope[a.Name] = a.Scope
		}

		hasKubeAPISmoke := false
		hasAWSAPISmoke := false

		taFiles, _ := filepath.Glob(filepath.Join(testDir, "ta_*_test.go"))
		for _, f := range taFiles {
			data, _ := os.ReadFile(f)
			content := string(data)
			if !strings.Contains(content, `Label("smoke")`) {
				continue
			}
			for ta, scope := range taScope {
				if strings.Contains(content, ta) {
					if scope == "kube-api" {
						hasKubeAPISmoke = true
					}
					if scope == "aws-api" {
						hasAWSAPISmoke = true
					}
				}
			}
		}

		Expect(hasKubeAPISmoke).To(BeTrue(), "smoke suite should include at least one kube-api TA")
		Expect(hasAWSAPISmoke).To(BeTrue(), "smoke suite should include at least one aws-api TA")
	})
})
