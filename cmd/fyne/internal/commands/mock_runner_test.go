package commands

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// appendEnvKeys lists the env vars that build.go merges via appendEnv
// rather than blindly appending. When the test crafts these keys and asks
// for os.Environ to be added too, the original copy from os.Environ must
// be dropped or the comparison sees a duplicate the production code does
// not produce.
var appendEnvKeys = []string{"CGO_CFLAGS", "CGO_LDFLAGS"}

type expectedValue struct {
	dir   any
	env   []string
	osEnv bool
	args  []string
}

type mockReturn struct {
	ret []byte
	err error
}

type expectedCall struct {
	dirSet bool
	envSet bool
}

type mockRunner struct {
	expectedValue
	expectedCall
	mockReturn
}

type testCommandRuns struct {
	runs       []mockRunner
	currentRun int
	t          *testing.T
}

func (t *testCommandRuns) runOutput(args ...string) ([]byte, error) {
	require.Less(t.t, t.currentRun, len(t.runs))
	require.Equal(t.t, len(t.runs[t.currentRun].args), len(args))

	expectedArgs := t.runs[t.currentRun].args
	require.Equal(t.t, expectedArgs, args)

	ret, err := t.runs[t.currentRun].ret, t.runs[t.currentRun].err
	t.currentRun++

	return ret, err
}

func (t *testCommandRuns) setDir(dir string) {
	require.Less(t.t, t.currentRun, len(t.runs))

	require.Equal(t.t, t.runs[t.currentRun].dir.(string), dir)
	t.runs[t.currentRun].dirSet = true
}

func (t *testCommandRuns) setEnv(env []string) {
	require.Less(t.t, t.currentRun, len(t.runs))

	// Prepare array for comparison
	expectedEnv := t.runs[t.currentRun].env
	if t.runs[t.currentRun].osEnv {
		merged := map[string]bool{}
		for _, e := range expectedEnv {
			for _, k := range appendEnvKeys {
				if strings.HasPrefix(e, k+"=") {
					merged[k] = true
				}
			}
		}
		for _, e := range os.Environ() {
			skip := false
			for k := range merged {
				if strings.HasPrefix(e, k+"=") {
					skip = true
					break
				}
			}
			if !skip {
				expectedEnv = append(expectedEnv, e)
			}
		}
	}
	sort.Strings(expectedEnv)
	sort.Strings(env)

	require.Equal(t.t, expectedEnv, env)

	t.runs[t.currentRun].envSet = true
}

func (t *testCommandRuns) verifyExpectation() {
	require.Equal(t.t, len(t.runs), t.currentRun)

	for _, value := range t.runs {
		if value.dir != nil {
			assert.Equal(t.t, true, value.dirSet)
		}
		if len(value.env) > 0 {
			assert.Equal(t.t, true, value.envSet)
		}
	}
}
