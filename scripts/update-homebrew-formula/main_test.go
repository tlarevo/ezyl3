package main

import (
	"strings"
	"testing"
)

const (
	amd64Archive = "ezyl3_0.1.0_darwin_amd64.tar.gz"
	arm64Archive = "ezyl3_0.1.0_darwin_arm64.tar.gz"
	amd64Sum     = "1111111111111111111111111111111111111111111111111111111111111111"
	arm64Sum     = "2222222222222222222222222222222222222222222222222222222222222222"
)

func TestParseChecksumsFindsRequiredArchives(t *testing.T) {
	got, err := parseChecksums(amd64Sum + "  " + amd64Archive + "\n" + arm64Sum + "  " + arm64Archive + "\n")
	if err != nil {
		t.Fatalf("parseChecksums returned error: %v", err)
	}
	if got[amd64Archive] != amd64Sum {
		t.Fatalf("missing amd64 checksum: %#v", got)
	}
	if got[arm64Archive] != arm64Sum {
		t.Fatalf("missing arm64 checksum: %#v", got)
	}
}

func TestParseChecksumsRejectsDuplicateArchive(t *testing.T) {
	_, err := parseChecksums(amd64Sum + "  " + amd64Archive + "\n" + arm64Sum + "  " + amd64Archive + "\n")
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate archive error, got %v", err)
	}
}

func TestParseChecksumsRejectsMalformedLine(t *testing.T) {
	_, err := parseChecksums("not-a-checksum-line\n")
	if err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("expected malformed line error, got %v", err)
	}
}

func TestFormulaDataRequiresBothMacArchives(t *testing.T) {
	checksums := map[string]string{arm64Archive: arm64Sum}
	_, err := buildFormulaData("v0.1.0", "tlarevo", "ezyl3", checksums)
	if err == nil || !strings.Contains(err.Error(), amd64Archive) {
		t.Fatalf("expected missing amd64 archive to fail, got %v", err)
	}

	checksums = map[string]string{amd64Archive: amd64Sum}
	_, err = buildFormulaData("v0.1.0", "tlarevo", "ezyl3", checksums)
	if err == nil || !strings.Contains(err.Error(), arm64Archive) {
		t.Fatalf("expected missing arm64 archive to fail, got %v", err)
	}
}

func TestBuildFormulaDataCreatesReleaseURLs(t *testing.T) {
	data, err := buildFormulaData("v0.1.0", "tlarevo", "ezyl3", map[string]string{
		amd64Archive: amd64Sum,
		arm64Archive: arm64Sum,
	})
	if err != nil {
		t.Fatalf("buildFormulaData returned error: %v", err)
	}
	if data.Version != "0.1.0" {
		t.Fatalf("Version = %q, want 0.1.0", data.Version)
	}
	if data.AMD64URL != "https://github.com/tlarevo/ezyl3/releases/download/v0.1.0/"+amd64Archive {
		t.Fatalf("AMD64URL = %q", data.AMD64URL)
	}
	if data.ARM64URL != "https://github.com/tlarevo/ezyl3/releases/download/v0.1.0/"+arm64Archive {
		t.Fatalf("ARM64URL = %q", data.ARM64URL)
	}
}

func TestRenderFormulaIncludesInstallAndPlatformAssets(t *testing.T) {
	tmpl := `class Ezyl3 < Formula
  version "{{ .Version }}"
  on_arm do
    url "{{ .ARM64URL }}"
    sha256 "{{ .ARM64SHA256 }}"
  end
  on_intel do
    url "{{ .AMD64URL }}"
    sha256 "{{ .AMD64SHA256 }}"
  end
  def install
    bin.install "ezyl3"
  end
end
`
	data, err := buildFormulaData("v0.1.0", "tlarevo", "ezyl3", map[string]string{
		amd64Archive: amd64Sum,
		arm64Archive: arm64Sum,
	})
	if err != nil {
		t.Fatalf("buildFormulaData returned error: %v", err)
	}
	rendered, err := renderFormula(tmpl, data)
	if err != nil {
		t.Fatalf("renderFormula returned error: %v", err)
	}
	for _, want := range []string{"0.1.0", data.AMD64URL, data.ARM64URL, amd64Sum, arm64Sum, `bin.install "ezyl3"`} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered formula missing %q:\n%s", want, rendered)
		}
	}
}
