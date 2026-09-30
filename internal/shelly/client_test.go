package shelly_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/veerendra2/shelly_device_exporter/internal/shelly"
)

func TestShelly(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shelly Suite")
}

var _ = Describe("Shelly Client", func() {
	It("fetches the status of one device", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.URL.Path).To(Equal("/rpc/Shelly.GetStatus"))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"sys":{"mac":"001122334455","uptime":100},"switch:0":{"apower":10.5}}`))
		}))
		defer server.Close()

		client := shelly.New(server.URL, "", "")
		status, err := client.Status(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(status.System.MAC).To(Equal("001122334455"))
		Expect(status.Switch0.APower).NotTo(BeNil())
	})

	It("returns an error for an unreachable device", func() {
		client := shelly.New("http://127.0.0.1:1", "", "")
		_, err := client.Status(context.Background())
		Expect(err).To(HaveOccurred())
	})

	It("returns an error for a non-200 response", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := shelly.New(server.URL, "", "")
		_, err := client.Status(context.Background())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("500"))
	})
})
