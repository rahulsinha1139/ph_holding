package main

import "testing"

func TestBuildMachineIDHash_IsStableAcrossMACOrder(t *testing.T) {
	first := []string{
		"rahuls-macbook-air.local",
		"36:93:c7:af:e2:c4",
		"2e:de:24:60:b0:09",
		"darwin",
		"arm64",
	}
	second := []string{
		"rahuls-macbook-air.local",
		"2e:de:24:60:b0:09",
		"36:93:c7:af:e2:c4",
		"darwin",
		"arm64",
	}

	if got, want := buildMachineIDHash(first), buildMachineIDHash(second); got == want {
		t.Fatalf("expected different hashes before sorting, got identical %s", got)
	}

	sortedFirst := []string{
		"rahuls-macbook-air.local",
		"2e:de:24:60:b0:09",
		"36:93:c7:af:e2:c4",
		"darwin",
		"arm64",
	}
	sortedSecond := []string{
		"rahuls-macbook-air.local",
		"2e:de:24:60:b0:09",
		"36:93:c7:af:e2:c4",
		"darwin",
		"arm64",
	}

	if got, want := buildMachineIDHash(sortedFirst), buildMachineIDHash(sortedSecond); got != want {
		t.Fatalf("expected stable hash after sorting, got %s want %s", got, want)
	}
}

func TestGetMachineID_CurrentDeviceIsStable(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("APPDATA", tempHome)

	first := GetMachineID()
	second := GetMachineID()
	if first == "" || second == "" {
		t.Fatal("expected non-empty machine id")
	}
	if first != second {
		t.Fatalf("expected stable machine id across consecutive calls, got %s and %s", first, second)
	}
	t.Logf("current_machine_id=%s", first)
}

func TestParseQuotedIoregValue(t *testing.T) {
	const sample = `
      "IOPlatformSerialNumber" = "SERIAL123"
      "IOPlatformUUID" = "AD2D3714-BD70-50EA-897F-EA1101425DE6"
`
	if got := parseQuotedIoregValue(sample, "IOPlatformUUID"); got != "AD2D3714-BD70-50EA-897F-EA1101425DE6" {
		t.Fatalf("unexpected platform UUID: %q", got)
	}
}

func TestMachineIDHashValidation(t *testing.T) {
	valid := buildMachineIDHash([]string{"stable", "device"})
	if !isMachineIDHash(valid) {
		t.Fatalf("expected generated hash to be valid")
	}
	if isMachineIDHash("not-a-hash") {
		t.Fatalf("expected invalid hash to be rejected")
	}
}
