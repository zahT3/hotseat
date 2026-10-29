package desktop

import "testing"

func SetBeforeSwapStep(t *testing.T, fn func(step string) error) {
	t.Helper()
	old := beforeSwapStep
	beforeSwapStep = fn
	t.Cleanup(func() { beforeSwapStep = old })
}
