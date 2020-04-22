package appcredentials

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestMustExistOrUnsetEnv(t *testing.T) {
	tests := []struct {
		name string
		file bool
		// if env var is set before MustExistOrUnsetEnv
		before bool
		// if env is set after MustExistOrUnsetEnv
		after bool
	}{
		{
			name:   "credential file exists and env var is set",
			file:   true,
			before: true,
			after:  true,
		},
		{
			name: "env var not set",
		},
		{
			name:   "env var set but credential file doesn't exist",
			file:   false,
			before: true,
			after:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make sure env is clean before testing.
			os.Unsetenv(envKey)

			path := "/tmp/app-credential-eng-test"
			if tt.file {
				f, err := ioutil.TempFile("/tmp", "app-credential-eng-test-*")
				if err != nil {
					t.Fatalf("unexpected error from creating temp file: %v", err)
				}
				path = f.Name()
				defer f.Close()
			}

			if tt.before {
				os.Setenv(envKey, path)
			}

			MustExistOrUnsetEnv()

			if tt.after && os.Getenv(envKey) == "" {
				t.Fatalf("Expect env to be set however it's not.")
			}
			if !tt.after && os.Getenv(envKey) != "" {
				t.Fatalf("Expect env to be empty however it's set.")

			}
		})
	}
}
