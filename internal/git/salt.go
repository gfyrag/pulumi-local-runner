package git

import "os"

const defaultPassphrase = "plr"

func getPassphrase() string {
	if p := os.Getenv("PULUMI_CONFIG_PASSPHRASE"); p != "" {
		return p
	}
	return defaultPassphrase
}

// EnsurePassphrase sets PULUMI_CONFIG_PASSPHRASE to match what plr uses,
// so Pulumi can decrypt secrets.
func EnsurePassphrase() {
	os.Setenv("PULUMI_CONFIG_PASSPHRASE", getPassphrase())
}
