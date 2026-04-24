//go:build !windows

package agent

func IsAdminRequired(err error) bool {
	return false
}

func RelaunchElevated(args []string) error {
	return nil
}
