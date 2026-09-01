//go:build e2e

// CLI: zoa version — server build info and health check.

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("zoa version", func() {
	for _, tgt := range targets {
		tgt := tgt

		Describe(tgt.Name, func() {
			It("reports client and server versions", Label("smoke"), func() {
				out, err := runZoa(tgt, "version")
				Expect(err).NotTo(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Client:"))
				Expect(out).To(ContainSubstring("Server:"))
				Expect(out).To(ContainSubstring("Target:"))
			})

			It("reports version in JSON format", func() {
				out, err := runZoa(tgt, "version", "-o", "json")
				Expect(err).NotTo(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("client"))
				Expect(out).To(ContainSubstring("server"))
			})
		})
	}
})
