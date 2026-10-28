package share

import "testing"

// SetBeforeStep installs a hook that runs before each plan action, for tests
// that need to inject a failure between steps.
func SetBeforeStep(t *testing.T, fn func(i int)) {
	t.Helper()
	old := beforeStep
	beforeStep = fn
	t.Cleanup(func() { beforeStep = old })
}
