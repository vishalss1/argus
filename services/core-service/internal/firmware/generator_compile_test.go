package firmware

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type arduinoLibraryList struct {
	InstalledLibraries []struct {
		Library struct {
			Name       string `json:"name"`
			InstallDir string `json:"install_dir"`
		} `json:"library"`
	} `json:"installed_libraries"`
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, relative)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return err
		}
		if _, err := io.Copy(output, input); err != nil {
			output.Close()
			return err
		}
		return output.Close()
	})
}

func TestGeneratedFirmwareCompiles(t *testing.T) {
	if os.Getenv("ARGUS_TEST_ARDUINO_COMPILE") != "1" {
		t.Skip("set ARGUS_TEST_ARDUINO_COMPILE=1 to run the ESP32 compile gate")
	}

	arduinoCLI, err := exec.LookPath("arduino-cli")
	if err != nil {
		t.Fatalf("arduino-cli is required: %v", err)
	}

	certPEM, keyPEM, err := generateTestCertAndKey()
	if err != nil {
		t.Fatal(err)
	}
	gen, err := NewGenerator(GeneratorConfig{
		ServerHost:             "127.0.0.1",
		HTTPPort:               8443,
		MQTTPort:               8883,
		RootCAPEM:              certPEM,
		WiFiSSID:               "TestSSID",
		WiFiPassword:           "TestPassword",
		OTASigningKeyID:        "test-key",
		OTASigningPublicKeyB64: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	})
	if err != nil {
		t.Fatal(err)
	}

	// The provisioning and fleet sketches are the two artifacts the backend
	// actually serves to users.  Both must compile.
	provision, err := gen.GenerateProvision(GenerateOptions{
		DeviceID:        "test-device",
		WorkspaceID:     "test-workspace",
		APIKey:          "test-api-key",
		FirmwareVersion: "1.0.0",
		CertPEM:         certPEM,
		PrivKeyPEM:      keyPEM,
	})
	if err != nil {
		t.Fatal(err)
	}
	fleet, err := gen.GenerateFleetFirmware("")
	if err != nil {
		t.Fatal(err)
	}

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate repository root")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "..", ".."))
	librariesDir := filepath.Join(t.TempDir(), "libraries")
	if err := os.MkdirAll(librariesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := copyTree(filepath.Join(repoRoot, "argus_sdk"), filepath.Join(librariesDir, "ArgusSDK")); err != nil {
		t.Fatal(err)
	}

	listOutput, err := exec.Command(arduinoCLI, "lib", "list", "--format", "json").Output()
	if err != nil {
		t.Fatalf("could not list Arduino libraries: %v", err)
	}
	var libraries arduinoLibraryList
	if err := json.Unmarshal(listOutput, &libraries); err != nil {
		t.Fatal(err)
	}
	required := map[string]bool{"ArduinoJson": false, "PubSubClient": false}
	for _, installed := range libraries.InstalledLibraries {
		if _, ok := required[installed.Library.Name]; !ok {
			continue
		}
		if err := copyTree(installed.Library.InstallDir, filepath.Join(librariesDir, installed.Library.Name)); err != nil {
			t.Fatal(err)
		}
		required[installed.Library.Name] = true
	}
	for name, found := range required {
		if !found {
			t.Fatalf("required Arduino library %s is not installed", name)
		}
	}

	sketches := []struct {
		name   string
		source []byte
		// wantMarker is the version string expected in the compiled image, or
		// "" if the artifact is not expected to carry one.
		wantMarker string
	}{
		// The provisioning sketch flashes once and reboots into fleet firmware,
		// so it carries no marker.
		{name: "argus_provision", source: provision},
		{name: "argus_fleet_firmware", source: fleet, wantMarker: FleetFirmwareVersion},
	}

	for _, sketch := range sketches {
		t.Run(sketch.name, func(t *testing.T) {
			sketchDir := filepath.Join(t.TempDir(), sketch.name)
			if err := os.MkdirAll(sketchDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(sketchDir, sketch.name+".ino"), sketch.source, 0o600); err != nil {
				t.Fatal(err)
			}

			buildDir := filepath.Join(t.TempDir(), sketch.name+"-build")
			cmd := exec.Command(
				arduinoCLI,
				"compile",
				"--fqbn", "esp32:esp32:esp32",
				"--libraries", librariesDir,
				"--build-path", buildDir,
				"--build-property", "compiler.cpp.extra_flags=-Wall -Wextra -Werror",
				sketchDir,
			)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("generated firmware did not compile: %v\n%s", err, output)
			}

			if sketch.wantMarker == "" {
				return
			}

			// The OTA upload path derives an artifact's version by scanning the
			// compiled image. If the linker discarded the marker, every real
			// upload would be rejected, so assert it survives into the .bin
			// rather than only into the source.
			image := findCompiledImage(t, buildDir)
			want := []byte("ARGUSVER:\x00" + sketch.wantMarker + "\x00")
			if !bytes.Contains(image, want) {
				t.Fatalf("compiled %s image does not contain the marker %q; the OTA upload path would reject it",
					sketch.name, string(want))
			}
		})
	}
}

// findCompiledImage returns the largest .bin under dir, which is the
// application image rather than a bootloader or partition table.
func findCompiledImage(t *testing.T, dir string) []byte {
	t.Helper()

	var best []byte
	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || !strings.HasSuffix(path, ".bin") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if len(data) > len(best) {
			best = data
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk build output: %v", err)
	}
	if len(best) == 0 {
		t.Fatalf("no compiled .bin found under %s", dir)
	}
	return best
}
