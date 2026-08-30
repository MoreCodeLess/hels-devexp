package cli

import "testing"

func TestAssetName(t *testing.T) {
	got := assetName("linux", "amd64")
	want := "hels_linux_amd64.tar.gz"
	if got != want {
		t.Errorf("assetName() = %q, want %q", got, want)
	}
}

func TestDownloadURL(t *testing.T) {
	got := downloadURL("MoreCodeLess/hels-devexp", "v0.1.0", "hels_linux_amd64.tar.gz")
	want := "https://github.com/MoreCodeLess/hels-devexp/releases/download/v0.1.0/hels_linux_amd64.tar.gz"
	if got != want {
		t.Errorf("downloadURL() = %q, want %q", got, want)
	}
}
