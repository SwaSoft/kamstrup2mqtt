package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const cliHelperEnv = "KAMSTRUP2MQTT_CLI_HELPER_ARGS"

// runCLI runs main() in a subprocess (this test binary re-executed) with args and
// returns its combined output and exit code.
func runCLI(t *testing.T, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestCLIHelperProcess$")
	cmd.Env = append(os.Environ(), cliHelperEnv+"="+strings.Join(args, "\x1f"))
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("running CLI: %v", err)
	}
	return out.String(), code
}

// TestCLIHelperProcess is not a real test: it runs main() when re-executed by runCLI.
func TestCLIHelperProcess(t *testing.T) {
	raw, ok := os.LookupEnv(cliHelperEnv)
	if !ok {
		t.Skip("helper process for CLI tests")
	}
	args := []string{"kamstrup2mqtt"}
	if raw != "" {
		args = append(args, strings.Split(raw, "\x1f")...)
	}
	flag.CommandLine = flag.NewFlagSet(args[0], flag.ExitOnError)
	os.Args = args
	main()
	os.Exit(0)
}

// TestCLIFlagsUnchanged checks that the refactored binary still offers every
// command-line flag of the original, with the same defaults.
func TestCLIFlagsUnchanged(t *testing.T) {
	out, code := runCLI(t, "-h")
	if code != 0 {
		t.Fatalf("-h exit code %d, want 0; output:\n%s", code, out)
	}
	for _, want := range []string{
		"-config", "-credfile", "-device", "-broker", "-topic", "-user", "-pass",
		"-clientid", "-qos", "-tls-ca", "-tls-cert", "-tls-key", "-tls-insecure",
		"MQTT QoS level (0, 1, or 2) (default -1)",
		"Skip TLS certificate verification (0=no, 1=yes) (default -1)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("usage output lacks %q:\n%s", want, out)
		}
	}
}

// TestCLIInvalidConfigFailsAsBefore checks that a configuration error still stops
// the binary with exit code 1 and the same message, before any MQTT connection.
func TestCLIInvalidConfigFailsAsBefore(t *testing.T) {
	out, code := runCLI(t, "-qos", "5")
	if code != 1 {
		t.Fatalf("exit code %d, want 1; output:\n%s", code, out)
	}
	if !strings.Contains(out, "Fatal error building configuration:") {
		t.Errorf("output lacks the configuration error:\n%s", out)
	}

	out, code = runCLI(t, "-config", "does-not-exist.json")
	if code != 1 {
		t.Fatalf("exit code %d, want 1; output:\n%s", code, out)
	}
	if !strings.Contains(out, "Fatal error building configuration: error loading config file:") {
		t.Errorf("output lacks the config file error:\n%s", out)
	}
}

// TestPackagedConfigExampleStillLoads checks that the shipped example config file
// is still read by the refactored binary's configuration loading.
func TestPackagedConfigExampleStillLoads(t *testing.T) {
	cfg, err := buildConfig("packaging/config.json.example", "", "", "", "", "", "", "", -1, "", "", "", -1)
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if cfg.Device != "/dev/ttyUSB0" || cfg.Broker != "tcp://localhost:1883" || cfg.Topic != "serial/bridge" ||
		cfg.ClientID != "kamstrup2mqtt-system" || cfg.QoS == nil || *cfg.QoS != 0 {
		t.Errorf("unexpected config from example: %+v", cfg)
	}
}
