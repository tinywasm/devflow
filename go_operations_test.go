package gitgo

import (
	"os"
	"testing"
)

func TestGoGetModulePath(t *testing.T) {
	dir, cleanup := testCreateGoModule("github.com/test/repo")
	defer cleanup()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	path, err := GoGetModulePath()
	if err != nil {
		t.Fatal(err)
	}

	if path != "github.com/test/repo" {
		t.Errorf("Expected github.com/test/repo, got %s", path)
	}
}

func TestGoGetModuleName(t *testing.T) {
	dir, cleanup := testCreateGoModule("github.com/test/myrepo")
	defer cleanup()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	name, err := GoGetModuleName()
	if err != nil {
		t.Fatal(err)
	}

	if name != "myrepo" {
		t.Errorf("Expected myrepo, got %s", name)
	}
}

func TestGoModVerify(t *testing.T) {
	dir, cleanup := testCreateGoModule("github.com/test/repo")
	defer cleanup()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	err := GoModVerify()
	if err != nil {
		t.Fatal(err)
	}
}

func TestGoTest(t *testing.T) {
	dir, cleanup := testCreateGoModule("github.com/test/repo")
	defer cleanup()

	// Create passing test
	testContent := `package main
import "testing"
func TestExample(t *testing.T) {}
`
	os.WriteFile(dir+"/main_test.go", []byte(testContent), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	err := GoTest()
	if err != nil {
		t.Fatal(err)
	}
}
