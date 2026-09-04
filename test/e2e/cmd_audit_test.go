//go:build e2e

// CLI: zoa audit — query the audit trail with filters and output formats.

package e2e

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("zoa audit", func() {
	for _, tgt := range targets {
		tgt := tgt

		Describe(tgt.Name, func() {
			It("records dispatched executions in the audit log", func() {
				exec := runAction(tgt, "get_resource", "--resource", "nodes")
				id, ok := exec["id"].(string)
				Expect(ok).To(BeTrue())
				Expect(id).NotTo(BeEmpty())

				Eventually(func() bool {
					out, err := runZoa(tgt, "audit", "-o", "json", "--since", "10m")
					Expect(err).NotTo(HaveOccurred(), out)

					jsonStr := extractJSON(out)
					var list struct {
						Items []struct {
							ExecutionID string `json:"execution_id"`
							Action      string `json:"action"`
						} `json:"items"`
					}
					Expect(json.Unmarshal([]byte(jsonStr), &list)).To(Succeed(), out)

					for _, e := range list.Items {
						if e.ExecutionID == id {
							Expect(e.Action).To(Equal("get_resource"))
							return true
						}
					}
					return false
				}, "30s", "3s").Should(BeTrue(), "dispatched execution %s should show up in `zoa audit`", id)
			})

			It("lists audit entries in table format", func() {
				out, err := runZoa(tgt, "audit", "--limit", "5")
				Expect(err).NotTo(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("TIMESTAMP"))
			})

			It("filters audit entries by method", func() {
				out, err := runZoa(tgt, "audit", "-o", "json", "--method", "POST", "--since", "1h", "--limit", "10")
				Expect(err).NotTo(HaveOccurred(), out)

				jsonStr := extractJSON(out)
				var list struct {
					Items []struct {
						Method string `json:"method"`
					} `json:"items"`
				}
				Expect(json.Unmarshal([]byte(jsonStr), &list)).To(Succeed(), out)

				for _, e := range list.Items {
					Expect(e.Method).To(Equal("POST"))
				}
			})

			It("filters audit entries by action", func() {
				out, err := runZoa(tgt, "audit", "-o", "json", "--action", "get_resource", "--since", "1h", "--limit", "10")
				Expect(err).NotTo(HaveOccurred(), out)

				jsonStr := extractJSON(out)
				var list struct {
					Items []struct {
						Action string `json:"action"`
					} `json:"items"`
				}
				Expect(json.Unmarshal([]byte(jsonStr), &list)).To(Succeed(), out)
				for _, e := range list.Items {
					Expect(e.Action).To(Equal("get_resource"))
				}
			})

			It("filters with --until time window", func() {
				out, err := runZoa(tgt, "audit", "-o", "json", "--since", "1h", "--until", "0s", "--limit", "5")
				Expect(err).NotTo(HaveOccurred(), out)

				jsonStr := extractJSON(out)
				var list struct {
					Items []interface{} `json:"items"`
				}
				Expect(json.Unmarshal([]byte(jsonStr), &list)).To(Succeed(), out)
			})
		})
	}
})
