//go:build e2e

// Behavior: write cooldown — rejection, --force bypass, per-params scoping, dry-run exemption.

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("write cooldown", func() {
	for _, tgt := range targets {
		tgt := tgt

		Describe(tgt.Name, func() {
			// Test cooldown enforcement for write TAs. This is a critical security
			// feature: write TAs have a mandatory cooldown period to prevent
			// accidental rapid repeated mutations.

			It("rejects second write within cooldown window", func() {
				// First write succeeds (with --force to ensure clean slate)
				exec := runAction(tgt, "rollout_restart",
					"--resource", "deployment",
					"--namespace", coreDNSNamespace,
					"--name", coreDNSName,
					"--force")
				Expect(exec["status"]).To(Equal("succeeded"))

				// Second write (same target, within cooldown) should fail
				out := runActionExpectFailure(tgt, "rollout_restart",
					"--resource", "deployment",
					"--namespace", coreDNSNamespace,
					"--name", coreDNSName)
				Expect(out).To(SatisfyAny(
					ContainSubstring("cooldown"),
					ContainSubstring("within the last"),
					ContainSubstring("rate limit"),
					ContainSubstring("too soon"),
				))
			})

			It("allows second write with --force to bypass cooldown", func() {
				// Both writes use --force, so cooldown is bypassed
				exec1 := runAction(tgt, "rollout_restart",
					"--resource", "deployment",
					"--namespace", coreDNSNamespace,
					"--name", coreDNSName,
					"--force")
				Expect(exec1["status"]).To(Equal("succeeded"))

				exec2 := runAction(tgt, "rollout_restart",
					"--resource", "deployment",
					"--namespace", coreDNSNamespace,
					"--name", coreDNSName,
					"--force")
				Expect(exec2["status"]).To(Equal("succeeded"))
			})

			It("cooldown does not apply to dry-run executions", func() {
				// dry-run never mutates, so cooldown shouldn't block it
				exec1 := runAction(tgt, "rollout_restart",
					"--resource", "deployment",
					"--namespace", coreDNSNamespace,
					"--name", coreDNSName,
					"--dry-run")
				Expect(exec1["status"]).To(Equal("succeeded"))
				Expect(exec1["action"]).To(Equal("get_resource")) // dry-run executes get_resource

				// Immediate second dry-run should also succeed
				exec2 := runAction(tgt, "rollout_restart",
					"--resource", "deployment",
					"--namespace", coreDNSNamespace,
					"--name", coreDNSName,
					"--dry-run")
				Expect(exec2["status"]).To(Equal("succeeded"))
				Expect(exec2["action"]).To(Equal("get_resource"))
			})

			It("cooldown is per-params, not per-action", func() {
				// Write to one target workload
				exec1 := runAction(tgt, "rollout_restart",
					"--resource", "deployment",
					"--namespace", coreDNSNamespace,
					"--name", coreDNSName,
					"--force") // --force to ensure clean slate from prior tests
				Expect(exec1["status"]).To(Equal("succeeded"))

				// Real write to a DIFFERENT workload (different params) — should
				// NOT be blocked by the first write's cooldown because cooldown
				// is per (action, params), not per (action, target).
				// kube-proxy is an EKS-managed DaemonSet, always present, safe
				// to restart (self-heals on every node).
				exec2 := runAction(tgt, "rollout_restart",
					"--resource", "daemonset",
					"--namespace", "kube-system",
					"--name", "kube-proxy")
				Expect(exec2["status"]).To(Equal("succeeded"))
			})
		})
	}
})
