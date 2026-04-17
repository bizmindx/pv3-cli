package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadPythonProject_Django(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "python", "python-django")
	info, err := readPythonProject(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertField(t, "Runtime", info.Runtime, "python")
	assertField(t, "ScriptName", info.ScriptName, "runserver")
	assertField(t, "RunCmd", info.RunCmd, "python manage.py runserver 0.0.0.0:8000")
	assertField(t, "InstallCmd", info.InstallCmd, "pip install -r requirements.txt")
	assertField(t, "PkgManager", info.PkgManager, "pip")
	assertField(t, "Image", info.Image, pythonImage)
}

func TestReadPythonProject_Flask(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "python", "python-flask")
	info, err := readPythonProject(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertField(t, "Runtime", info.Runtime, "python")
	assertField(t, "ScriptName", info.ScriptName, "flask run")
	assertField(t, "RunCmd", info.RunCmd, "flask run --host=0.0.0.0 --port=5000")
	assertField(t, "InstallCmd", info.InstallCmd, "pip install -r requirements.txt")
	assertField(t, "PkgManager", info.PkgManager, "pip")
}

func TestReadPythonProject_FastAPI(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "python", "python-fastapi")
	info, err := readPythonProject(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertField(t, "Runtime", info.Runtime, "python")
	assertField(t, "ScriptName", info.ScriptName, "uvicorn")
	assertField(t, "RunCmd", info.RunCmd, "uvicorn main:app --host 0.0.0.0 --port 8000 --reload")
	assertField(t, "InstallCmd", info.InstallCmd, "pip install -r requirements.txt")
	assertField(t, "PkgManager", info.PkgManager, "pip")
}

func TestReadPythonProject_PyprojectScripts(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "python", "python-pyproject-scripts")
	info, err := readPythonProject(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertField(t, "Runtime", info.Runtime, "python")
	assertField(t, "ScriptName", info.ScriptName, "dev")
	assertField(t, "ScriptCmd", info.ScriptCmd, "app:main")
	assertField(t, "InstallCmd", info.InstallCmd, "pip install -r requirements.txt")
	assertField(t, "PkgManager", info.PkgManager, "pip")
}

func TestReadPythonProject_Poetry(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "python", "python-poetry")
	info, err := readPythonProject(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertField(t, "Runtime", info.Runtime, "python")
	assertField(t, "ScriptName", info.ScriptName, "serve")
	assertField(t, "ScriptCmd", info.ScriptCmd, "app:main")
	assertField(t, "InstallCmd", info.InstallCmd, "poetry install")
	assertField(t, "PkgManager", info.PkgManager, "poetry")
}

func TestReadPythonProject_Uv(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "python", "python-uv")
	info, err := readPythonProject(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertField(t, "Runtime", info.Runtime, "python")
	assertField(t, "ScriptName", info.ScriptName, "runserver")
	assertField(t, "InstallCmd", info.InstallCmd, "uv sync")
	assertField(t, "PkgManager", info.PkgManager, "uv")
}

func TestPythonInstallCmd(t *testing.T) {
	tests := []struct {
		pm   string
		want string
	}{
		{"uv", "uv sync"},
		{"poetry", "poetry install"},
		{"pipenv", "pipenv install"},
		{"pip", "pip install -r requirements.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.pm, func(t *testing.T) {
			got := pythonInstallCmd(tt.pm)
			if got != tt.want {
				t.Errorf("pythonInstallCmd(%q) = %q, want %q", tt.pm, got, tt.want)
			}
		})
	}
}

func TestReadPythonProject_NoDev(t *testing.T) {
	dir := filepath.Join(testdataDir(t), "python", "python-no-dev")
	_, err := readPythonProject(dir)
	if err == nil {
		t.Error("expected error for Python project with no detectable dev command")
	}
}

func TestDetectPythonPkgManager(t *testing.T) {
	tests := []struct {
		name   string
		files  []string
		wantPM string
	}{
		{"uv from lockfile", []string{"uv.lock"}, "uv"},
		{"poetry from lockfile", []string{"poetry.lock"}, "poetry"},
		{"pipenv from Pipfile", []string{"Pipfile"}, "pipenv"},
		{"pipenv from Pipfile.lock", []string{"Pipfile.lock"}, "pipenv"},
		{"pip by default", []string{}, "pip"},
		{"uv takes priority over poetry", []string{"uv.lock", "poetry.lock"}, "uv"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, f), []byte{}, 0644); err != nil {
					t.Fatal(err)
				}
			}

			got := detectPythonPkgManager(dir)
			if got != tt.wantPM {
				t.Errorf("detectPythonPkgManager() = %q, want %q", got, tt.wantPM)
			}
		})
	}
}

func TestIsPythonProject(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  bool
	}{
		{"pyproject.toml", []string{"pyproject.toml"}, true},
		{"requirements.txt", []string{"requirements.txt"}, true},
		{"setup.py", []string{"setup.py"}, true},
		{"Pipfile", []string{"Pipfile"}, true},
		{"empty directory", []string{}, false},
		{"only go.mod", []string{"go.mod"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, f), []byte{}, 0644); err != nil {
					t.Fatal(err)
				}
			}

			got := isPythonProject(dir)
			if got != tt.want {
				t.Errorf("isPythonProject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractDepName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"django>=5.1", "django"},
		{"flask==3.1.0", "flask"},
		{"uvicorn[standard]>=0.34.0", "uvicorn"},
		{"requests", "requests"},
		{"FastAPI>=0.115.0", "fastapi"},
		{`"django>=5.1",`, "django"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractDepName(tt.input)
			if got != tt.want {
				t.Errorf("extractDepName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func assertField(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", field, got, want)
	}
}
