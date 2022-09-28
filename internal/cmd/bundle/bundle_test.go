package bundle

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"code-intelligence.com/cifuzz/internal/bundler"
	"code-intelligence.com/cifuzz/internal/cmdutils"
	"code-intelligence.com/cifuzz/internal/config"
	"code-intelligence.com/cifuzz/internal/testutil"
	"code-intelligence.com/cifuzz/pkg/dependencies"
	"code-intelligence.com/cifuzz/pkg/log"
)

var testOut io.ReadWriter

func TestMain(m *testing.M) {
	// capture log output
	testOut = bytes.NewBuffer([]byte{})
	oldOut := log.Output
	log.Output = testOut
	viper.Set("verbose", true)

	// Make the bundle command not fail on unsupported platforms to be
	// able to test it on all platforms
	err := os.Setenv("CIFUZZ_BUNDLE_ON_UNSUPPORTED_PLATFORMS", "1")
	if err != nil {
		panic(err)
	}

	m.Run()

	log.Output = oldOut
	dependencies.ResetDefaultsForTestsOnly()
}

func TestUnknownBuildSystem(t *testing.T) {
	_, err := cmdutils.ExecuteCommand(t, newWithOptions(&bundler.Opts{}), os.Stdin)
	require.Error(t, err)
}

func TestClangMissing(t *testing.T) {
	deps := dependencies.CreateTestDeps(t, []dependencies.Key{
		dependencies.CLANG, dependencies.CMAKE,
	})
	dependencies.OverwriteInstalledWithFalse(deps[dependencies.CLANG])

	opts := &bundler.Opts{}
	opts.BuildSystem = config.BuildSystemCMake

	// clone the example project because this command needs to parse an actual
	// project config... if there is none it will fail before the dependency check
	_, cleanup := testutil.BootstrapExampleProjectForTest("run-cmd-test", config.BuildSystemCMake)
	defer cleanup()

	_, err := cmdutils.ExecuteCommand(t, newWithOptions(opts), os.Stdin)
	require.Error(t, err)

	output, err := io.ReadAll(testOut)
	require.NoError(t, err)
	assert.Contains(t, string(output), fmt.Sprintf(dependencies.MESSAGE_MISSING, "clang"))
}

func TestClangVersion(t *testing.T) {
	deps := dependencies.CreateTestDeps(t, []dependencies.Key{
		dependencies.CLANG, dependencies.CMAKE,
	})

	dep := deps[dependencies.CLANG]
	version := dependencies.OverwriteGetVersionWith0(dep)

	opts := &bundler.Opts{}
	opts.BuildSystem = config.BuildSystemCMake

	// clone the example project because this command needs to parse an actual
	// project config... if there is none it will fail before the dependency check
	_, cleanup := testutil.BootstrapExampleProjectForTest("run-cmd-test", config.BuildSystemCMake)
	defer cleanup()

	_, err := cmdutils.ExecuteCommand(t, newWithOptions(opts), os.Stdin)
	require.Error(t, err)

	output, err := io.ReadAll(testOut)
	require.NoError(t, err)
	assert.Contains(t, string(output),
		fmt.Sprintf(dependencies.MESSAGE_VERSION, "clang", dep.MinVersion.String(), version))
}

func TestCMakeMissing(t *testing.T) {
	deps := dependencies.CreateTestDeps(t, []dependencies.Key{
		dependencies.CLANG, dependencies.CMAKE,
	})
	dependencies.OverwriteInstalledWithFalse(deps[dependencies.CMAKE])

	opts := &bundler.Opts{}
	opts.BuildSystem = config.BuildSystemCMake

	// clone the example project because this command needs to parse an actual
	// project config... if there is none it will fail before the dependency check
	_, cleanup := testutil.BootstrapExampleProjectForTest("run-cmd-test", config.BuildSystemCMake)
	defer cleanup()

	_, err := cmdutils.ExecuteCommand(t, newWithOptions(opts), os.Stdin)
	require.Error(t, err)

	output, err := io.ReadAll(testOut)
	require.NoError(t, err)

	assert.Contains(t, string(output), fmt.Sprintf(dependencies.MESSAGE_MISSING, "cmake"))
}

func TestEnvVarsSetInConfigFile(t *testing.T) {
	projectDir := testutil.BootstrapEmptyProject(t, "bundle-test-")
	configFileContent := `env:
  - FOO=foo
  - BAR
  - NO_SUCH_VARIABLE
`
	err := os.WriteFile(filepath.Join(projectDir, "cifuzz.yaml"), []byte(configFileContent), 0644)
	require.NoError(t, err)

	err = os.Setenv("BAR", "bar")
	require.NoError(t, err)

	opts := &bundler.Opts{
		ProjectDir:  projectDir,
		ConfigDir:   projectDir,
		BuildSystem: config.BuildSystemCMake,
	}

	cmd := newWithOptions(opts)
	err = cmd.PreRunE(cmd, nil)
	require.NoError(t, err)

	require.Equal(t, []string{"FOO=foo", "BAR=bar"}, opts.Env)
}
