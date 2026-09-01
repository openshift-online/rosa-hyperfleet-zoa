//go:build e2e

// TA: delete_pod (kube-api scope, write) — deletes a pod and verifies owner self-heals.

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("delete_pod", func() {
	for _, tgt := range targets {
		tgt := tgt

		Describe(tgt.Name, func() {
			It("deletes a real coredns pod and confirms the owner self-heals", func() {
				before := coreDNSPodNames(tgt)
				Expect(before).NotTo(BeEmpty(), "coredns pods not found (selector %q in %s)", coreDNSSelector, coreDNSNamespace)
				victim := before[0]

				// The TA itself refuses to delete standalone pods (no
				// ownerReferences) — coredns pods are always owned by a
				// ReplicaSet, so the delete is safe regardless of replica
				// count because the controller will recreate the pod.
				exec := runAction(tgt, "delete_pod", "--namespace", coreDNSNamespace, "--name", victim, "--force")
				Expect(exec["status"]).To(Equal("succeeded"))
				Expect(exec["action"]).To(Equal("delete_pod"), "this must be a real (non-dry-run) execution")

				Eventually(func() []string {
					return coreDNSPodNamesOrEmpty(tgt)
				}, "2m", "5s").Should(And(
					HaveLen(len(before)),
					Not(ContainElement(victim)),
				), "coredns should self-heal back to %d pod(s) with a new pod replacing %q", len(before), victim)
			})

			It("rejects a missing required parameter before touching the cluster", func() {
				out := runActionExpectFailure(tgt, "delete_pod", "--namespace", coreDNSNamespace)
				Expect(out).To(ContainSubstring("name"))
			})

			It("rejects a pod that does not exist", func() {
				out := runActionExpectFailure(tgt, "delete_pod",
					"--namespace", coreDNSNamespace, "--name", "e2e-nonexistent-pod-zoa", "--dry-run")
				Expect(out).To(SatisfyAny(
					ContainSubstring("not found"),
					ContainSubstring("could not find"),
				))
			})
		})
	}
})
