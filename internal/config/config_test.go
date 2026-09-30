package config_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/veerendra2/shelly_device_exporter/internal/config"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Suite")
}

func writeConfig(t GinkgoTInterface, yaml string) string {
	tmpfile, err := os.CreateTemp("", "config*.yml")
	Expect(err).NotTo(HaveOccurred())
	_, err = tmpfile.Write([]byte(yaml))
	Expect(err).NotTo(HaveOccurred())
	Expect(tmpfile.Close()).To(Succeed())
	t.Cleanup(func() { _ = os.Remove(tmpfile.Name()) })
	return tmpfile.Name()
}

var _ = Describe("Config", func() {
	It("parses modules and defaults the username", func() {
		path := writeConfig(GinkgoT(), `
modules:
  auth:
    password: "direct-password"
`)
		cfg, err := config.LoadConfig(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.Modules).To(HaveKey("auth"))
		Expect(cfg.Modules["auth"].Username).To(Equal("admin"))
		Expect(cfg.Modules["auth"].Password).To(Equal("direct-password"))
	})

	It("returns the built-in default module when name is empty", func() {
		path := writeConfig(GinkgoT(), `modules: {}`)
		cfg, err := config.LoadConfig(path)
		Expect(err).NotTo(HaveOccurred())
		mod, ok := cfg.Module("")
		Expect(ok).To(BeTrue())
		Expect(mod.Username).To(Equal("admin"))
	})

	It("resolves an unknown module as not ok", func() {
		path := writeConfig(GinkgoT(), `modules: {}`)
		cfg, err := config.LoadConfig(path)
		Expect(err).NotTo(HaveOccurred())
		_, ok := cfg.Module("nope")
		Expect(ok).To(BeFalse())
	})

	It("reads password_file and trims trailing whitespace", func() {
		secret, err := os.CreateTemp("", "secret*.txt")
		Expect(err).NotTo(HaveOccurred())
		_, err = secret.WriteString("s3cret \t\r\n")
		Expect(err).NotTo(HaveOccurred())
		Expect(secret.Close()).To(Succeed())
		GinkgoT().Cleanup(func() { _ = os.Remove(secret.Name()) })

		path := writeConfig(GinkgoT(), `
modules:
  auth:
    password_file: `+secret.Name()+`
`)
		cfg, err := config.LoadConfig(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.Modules["auth"].Password).To(Equal("s3cret"))
	})

	It("errors on unreadable password_file", func() {
		path := writeConfig(GinkgoT(), `
modules:
  auth:
    password_file: /nonexistent/does-not-exist.txt
`)
		_, err := config.LoadConfig(path)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("auth"))
	})

	It("errors when password and password_file are both set", func() {
		path := writeConfig(GinkgoT(), `
modules:
  auth:
    password: "a"
    password_file: /tmp/anything.txt
`)
		_, err := config.LoadConfig(path)
		Expect(err).To(HaveOccurred())
	})

	It("expands env vars via the existing template", func() {
		Expect(os.Setenv("TEST_PASSWORD", "secret-password")).To(Succeed())
		GinkgoT().Cleanup(func() { _ = os.Unsetenv("TEST_PASSWORD") })

		path := writeConfig(GinkgoT(), `
modules:
  auth:
    password: '{{ env "TEST_PASSWORD" }}'
`)
		cfg, err := config.LoadConfig(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.Modules["auth"].Password).To(Equal("secret-password"))
	})

	It("rejects the old devices format (strict parsing)", func() {
		path := writeConfig(GinkgoT(), `
devices:
  - name: "test-device"
    address: "http://1.2.3.4"
`)
		_, err := config.LoadConfig(path)
		Expect(err).To(HaveOccurred())
	})
})
