package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/pathutil"
)

// setupDrives points C: and D: at fresh directories and leaves the shell on
// C:\sub, returning the Unix roots of both drives.
func setupDrives(t *testing.T) (driveC, driveD string) {
	t.Helper()

	root := t.TempDir()
	driveC = filepath.Join(root, "c")
	driveD = filepath.Join(root, "d")
	for _, dir := range []string{filepath.Join(driveC, "sub"), filepath.Join(driveD, "other")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("MSBATCH_DRIVE_C", driveC)
	t.Setenv("MSBATCH_DRIVE_D", driveD)
	t.Chdir(filepath.Join(driveC, "sub"))

	pathutil.SetDriveDir('C', filepath.Join(driveC, "sub"))
	pathutil.SetDriveDir('D', driveD)

	return driveC, driveD
}

func windowsCwd(t *testing.T) string {
	t.Helper()
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return pathutil.ToWindowsPath(pwd)
}

// TestCmdCdKeepsDriveWithoutSlashD pins cmd.exe's rule that CD to another
// drive records that drive's directory without moving the shell to it.
func TestCmdCdKeepsDriveWithoutSlashD(t *testing.T) {
	_, driveD := setupDrives(t)

	p, _, _ := newTestProc(nil)
	if err := cmdCd(p, testCmd("cd", `D:\other`)); err != nil {
		t.Fatal(err)
	}

	if got, want := windowsCwd(t), `C:\sub`; got != want {
		t.Errorf("cwd after CD to another drive = %q, want %q", got, want)
	}
	if got, want := pathutil.DriveDir('D'), filepath.Join(driveD, "other"); got != want {
		t.Errorf("recorded D: directory = %q, want %q", got, want)
	}
	if got := testErrorLevel(p); got != "0" {
		t.Errorf("ERRORLEVEL = %q, want 0", got)
	}
}

// TestCmdCdSlashDSwitchesDrive pins that /D does move the shell.
func TestCmdCdSlashDSwitchesDrive(t *testing.T) {
	setupDrives(t)

	p, _, _ := newTestProc(nil)
	if err := cmdCd(p, testCmd("cd", "/d", `D:\other`)); err != nil {
		t.Fatal(err)
	}

	if got, want := windowsCwd(t), `D:\other`; got != want {
		t.Errorf("cwd after CD /D = %q, want %q", got, want)
	}
}

// TestCmdCdMissingDirectoryOnOtherDrive pins that a bad path fails without
// disturbing the shell or the recorded directory.
func TestCmdCdMissingDirectoryOnOtherDrive(t *testing.T) {
	_, driveD := setupDrives(t)

	p, _, errOut := newTestProc(nil)
	if err := cmdCd(p, testCmd("cd", `D:\nope`)); err != nil {
		t.Fatal(err)
	}

	if got, want := windowsCwd(t), `C:\sub`; got != want {
		t.Errorf("cwd = %q, want %q", got, want)
	}
	if got, want := pathutil.DriveDir('D'), driveD; got != want {
		t.Errorf("recorded D: directory = %q, want it unchanged at %q", got, want)
	}
	if want := "The system cannot find the path specified."; !strings.Contains(errOut.String(), want) {
		t.Errorf("stderr = %q, want it to contain %q", errOut.String(), want)
	}
	if got := testErrorLevel(p); got == "0" {
		t.Error("ERRORLEVEL = 0, want non-zero")
	}
}

// TestCmdDriveSwitchRestoresRememberedDirectory pins that a bare "X:" lands in
// the directory last selected on that drive rather than its root.
func TestCmdDriveSwitchRestoresRememberedDirectory(t *testing.T) {
	setupDrives(t)

	p, _, _ := newTestProc(nil)
	if err := cmdCd(p, testCmd("cd", `D:\other`)); err != nil {
		t.Fatal(err)
	}
	if err := cmdDriveSwitch(p, testCmd("d:")); err != nil {
		t.Fatal(err)
	}

	if got, want := windowsCwd(t), `D:\other`; got != want {
		t.Errorf("cwd after bare D: = %q, want %q", got, want)
	}

	if err := cmdDriveSwitch(p, testCmd("c:")); err != nil {
		t.Fatal(err)
	}
	if got, want := windowsCwd(t), `C:\sub`; got != want {
		t.Errorf("cwd after bare C: = %q, want %q", got, want)
	}
}
