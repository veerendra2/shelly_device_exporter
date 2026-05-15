package shelly_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/veerendra2/shelly-plug-exporter/internal/config"
	"github.com/veerendra2/shelly-plug-exporter/internal/shelly"
)

func TestShelly(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shelly Suite")
}

var _ = Describe("Shelly Client", func() {
	var (
		server *httptest.Server
		client shelly.Client
	)

	BeforeEach(func() {
		// Mock Shelly API
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"sys":{"mac":"001122334455","uptime":100},"switch:0":{"apower":10.5}}`))
		}))

		cfg := config.Config{
			Devices: []config.Device{
				{Name: "test-1", Address: server.URL},
				{Name: "test-2", Address: server.URL},
			},
		}
		var err error
		client, err = shelly.New(cfg)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("BulkStatus", func() {
		It("should fetch statuses from multiple devices concurrently", func() {
			ctx := context.Background()
			statuses := client.BulkStatus(ctx)

			Expect(statuses).To(HaveLen(2))
			Expect(statuses[0].Name).To(Or(Equal("test-1"), Equal("test-2")))
			Expect(statuses[0].System.MAC).To(Equal("001122334455"))
		})

		It("should respect context cancellation and prevent deadlock", func() {
			// Create a context that is already cancelled
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			statuses := client.BulkStatus(ctx)

			// All should be filtered out because they contain errors
			Expect(statuses).To(HaveLen(0))
		})
	})
})
