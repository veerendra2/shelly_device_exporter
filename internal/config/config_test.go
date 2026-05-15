package config_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/veerendra2/shelly-plug-exporter/internal/config"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Suite")
}

var _ = Describe("Config", func() {
	Context("When loading a valid configuration", func() {
		It("should parse the devices and set default values", func() {
			yaml := `
devices:
  - name: "test-device"
    address: "http://1.2.3.4"
`
			tmpfile, err := os.CreateTemp("", "config*.yml")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(tmpfile.Name())

			_, err = tmpfile.Write([]byte(yaml))
			Expect(err).NotTo(HaveOccurred())
			tmpfile.Close()

			cfg, err := config.LoadConfig(tmpfile.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.Devices).To(HaveLen(1))
			Expect(cfg.Devices[0].Name).To(Equal("test-device"))
			Expect(cfg.Devices[0].Username).To(Equal("admin")) // Default value
		})
	})

	Context("When validation fails", func() {
		It("should return an error for missing required fields", func() {
			yaml := `
devices:
  - name: "" # Required
    address: "not-a-url"
`
			tmpfile, err := os.CreateTemp("", "config-fail*.yml")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(tmpfile.Name())

			_, err = tmpfile.Write([]byte(yaml))
			Expect(err).NotTo(HaveOccurred())
			tmpfile.Close()

			_, err = config.LoadConfig(tmpfile.Name())
			Expect(err).To(HaveOccurred())
		})
	})
})
