package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"

	"github.com/ollama/ollama/api"
	"github.com/ollama/ollama/types/model"
	"github.com/ollama/ollama/app/lifecycle"
)

func TestShowInfo(t *testing.T) {
	t.Run("bare details", func(t *testing.T) {
		var b bytes.Buffer
		if err := showInfo(&api.ShowResponse{
			Details: api.ModelDetails{
				Family:            "test",
				ParameterSize:     "7B",
				QuantizationLevel: "FP16",
			},
		}, false, &b); err != nil {
			t.Fatal(err)
		}

		expect := `  Model
    architecture    test    
    parameters      7B      
    quantization    FP16    

`

		if diff := cmp.Diff(expect, b.String()); diff != "" {
			t.Errorf("unexpected output (-want +got):\n%s", diff)
		}
	})

	t.Run("bare model info", func(t *testing.T) {
		var b bytes.Buffer
		if err := showInfo(&api.ShowResponse{
			ModelInfo: map[string]any{
				"general.architecture":    "test",
				"general.parameter_count": float64(7_000_000_000),
				"test.context_length":     float64(0),
				"test.embedding_length":   float64(0),
			},
			Details: api.ModelDetails{
				Family:            "test",
				ParameterSize:     "7B",
				QuantizationLevel: "FP16",
			},
		}, false, &b); err != nil {
			t.Fatal(err)
		}

		expect := `  Model
    architecture        test    
    parameters          7B      
    context length      0       
    embedding length    0       
    quantization        FP16    

`
		if diff := cmp.Diff(expect, b.String()); diff != "" {
			t.Errorf("unexpected output (-want +got):\n%s", diff)
		}
	})

	t.Run("verbose model", func(t *testing.T) {
		var b bytes.Buffer
		if err := showInfo(&api.ShowResponse{
			Details: api.ModelDetails{
				Family:            "test",
				ParameterSize:     "8B",
				QuantizationLevel: "FP16",
			},
			Parameters: `
			stop up`,
			ModelInfo: map[string]any{
				"general.architecture":    "test",
				"general.parameter_count": float64(8_000_000_000),
				"some.true_bool":          true,
				"some.false_bool":         false,
				"test.context_length":     float64(1000),
				"test.embedding_length":   float64(11434),
			},
			Tensors: []api.Tensor{
				{Name: "blk.0.attn_k.weight", Type: "BF16", Shape: []uint64{42, 3117}},
				{Name: "blk.0.attn_q.weight", Type: "FP16", Shape: []uint64{3117, 42}},
			},
		}, true, &b); err != nil {
			t.Fatal(err)
		}

		expect := `  Model
    architecture        test     
    parameters          8B       
    context length      1000     
    embedding length    11434    
    quantization        FP16     

  Parameters
    stop    up    

  Metadata
    general.architecture       test     
    general.parameter_count    8e+09    
    some.false_bool            false    
    some.true_bool             true     
    test.context_length        1000     
    test.embedding_length      11434    

  Tensors
    blk.0.attn_k.weight    BF16    [42 3117]    
    blk.0.attn_q.weight    FP16    [3117 42]    

`
		if diff := cmp.Diff(expect, b.String()); diff != "" {
			t.Errorf("unexpected output (-want +got):\n%s", diff)
		}
	})

	t.Run("parameters", func(t *testing.T) {
		var b bytes.Buffer
		if err := showInfo(&api.ShowResponse{
			Details: api.ModelDetails{
				Family:            "test",
				ParameterSize:     "7B",
				QuantizationLevel: "FP16",
			},
			Parameters: `
			stop never
			stop gonna
			stop give
			stop you
			stop up
			temperature 99`,
		}, false, &b); err != nil {
			t.Fatal(err)
		}

		expect := `  Model
    architecture    test    
    parameters      7B      
    quantization    FP16    

  Parameters
    stop           never    
    stop           gonna    
    stop           give     
    stop           you      
    stop           up       
    temperature    99       

`
		if diff := cmp.Diff(expect, b.String()); diff != "" {
			t.Errorf("unexpected output (-want +got):\n%s", diff)
		}
	})

	t.Run("project info", func(t *testing.T) {
		var b bytes.Buffer
		if err := showInfo(&api.ShowResponse{
			Details: api.ModelDetails{
				Family:            "test",
				ParameterSize:     "7B",
				QuantizationLevel: "FP16",
			},
			ProjectorInfo: map[string]any{
				"general.architecture":         "clip",
				"general.parameter_count":      float64(133_700_000),
				"clip.vision.embedding_length": float64(0),
				"clip.vision.projection_dim":   float64(0),
			},
		}, false, &b); err != nil {
			t.Fatal(err)
		}

		expect := `  Model
    architecture    test    
    parameters      7B      
    quantization    FP16    

  Projector
    architecture        clip       
    parameters          133.70M    
    embedding length    0          
    dimensions          0          

`
		if diff := cmp.Diff(expect, b.String()); diff != "" {
			t.Errorf("unexpected output (-want +got):\n%s", diff)
		}
	})

	t.Run("system", func(t *testing.T) {
		var b bytes.Buffer
		if err := showInfo(&api.ShowResponse{
			Details: api.ModelDetails{
				Family:            "test",
				ParameterSize:     "7B",
				QuantizationLevel: "FP16",
			},
			System: `You are a pirate!
Ahoy, matey!
Weigh anchor!
			`,
		}, false, &b); err != nil {
			t.Fatal(err)
		}

		expect := `  Model
    architecture    test    
    parameters      7B      
    quantization    FP16    

  System
    You are a pirate!    
    Ahoy, matey!         
    ...                  

`
		if diff := cmp.Diff(expect, b.String()); diff != "" {
			t.Errorf("unexpected output (-want +got):\n%s", diff)
		}
	})

	t.Run("license", func(t *testing.T) {
		var b bytes.Buffer
		license := "MIT License\nCopyright (c) Ollama\n"
		if err := showInfo(&api.ShowResponse{
			Details: api.ModelDetails{
				Family:            "test",
				ParameterSize:     "7B",
				QuantizationLevel: "FP16",
			},
			License: license,
		}, false, &b); err != nil {
			t.Fatal(err)
		}

		expect := `  Model
    architecture    test    
    parameters      7B      
    quantization    FP16    

  License
    MIT License             
    Copyright (c) Ollama    

`
		if diff := cmp.Diff(expect, b.String()); diff != "" {
			t.Errorf("unexpected output (-want +got):\n%s", diff)
		}
	})

	t.Run("capabilities", func(t *testing.T) {
		var b bytes.Buffer
		if err := showInfo(&api.ShowResponse{
			Details: api.ModelDetails{
				Family:            "test",
				ParameterSize:     "7B",
				QuantizationLevel: "FP16",
			},
			Capabilities: []model.Capability{model.CapabilityVision, model.CapabilityTools},
		}, false, &b); err != nil {
			t.Fatal(err)
		}

		expect := "  Model\n" +
			"    architecture    test    \n" +
			"    parameters      7B      \n" +
			"    quantization    FP16    \n" +
			"\n" +
			"  Capabilities\n" +
			"    vision    \n" +
			"    tools     \n" +
			"\n"

		if diff := cmp.Diff(expect, b.String()); diff != "" {
			t.Errorf("unexpected output (-want +got):\n%s", diff)
		}
	})
}

func TestDeleteHandler(t *testing.T) {
	stopped := false
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/delete" && r.Method == http.MethodDelete {
			var req api.DeleteRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.Name == "test-model" {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
			return
		}
		if r.URL.Path == "/api/generate" && r.Method == http.MethodPost {
			var req api.GenerateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.Model == "test-model" {
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(api.GenerateResponse{
					Done: true,
				}); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				stopped = true
				return
			} else {
				w.WriteHeader(http.StatusNotFound)
				if err := json.NewEncoder(w).Encode(api.GenerateResponse{
					Done: false,
				}); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			}
		}
	}))

	t.Setenv("OLLAMA_HOST", mockServer.URL)
	t.Cleanup(mockServer.Close)

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	if err := DeleteHandler(cmd, []string{"test-model"}); err != nil {
		t.Fatalf("DeleteHandler failed: %v", err)
	}
	if !stopped {
		t.Fatal("Model was not stopped before deletion")
	}

	err := DeleteHandler(cmd, []string{"test-model-not-found"})
	if err == nil || !strings.Contains(err.Error(), "unable to stop existing running model \"test-model-not-found\"") {
		t.Fatalf("DeleteHandler failed: expected error about stopping non-existent model, got %v", err)
	}
}

func TestGetModelfileName(t *testing.T) {
	tests := []struct {
		name          string
		modelfileName string
		fileExists    bool
		expectedName  string
		expectedErr   error
	}{
		{
			name:          "no modelfile specified, no modelfile exists",
			modelfileName: "",
			fileExists:    false,
			expectedName:  "",
			expectedErr:   os.ErrNotExist,
		},
		{
			name:          "no modelfile specified, modelfile exists",
			modelfileName: "",
			fileExists:    true,
			expectedName:  "Modelfile",
			expectedErr:   nil,
		},
		{
			name:          "modelfile specified, no modelfile exists",
			modelfileName: "crazyfile",
			fileExists:    false,
			expectedName:  "",
			expectedErr:   os.ErrNotExist,
		},
		{
			name:          "modelfile specified, modelfile exists",
			modelfileName: "anotherfile",
			fileExists:    true,
			expectedName:  "anotherfile",
			expectedErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{
				Use: "fakecmd",
			}
			cmd.Flags().String("file", "", "path to modelfile")

			var expectedFilename string

			if tt.fileExists {
				var fn string
				if tt.modelfileName != "" {
					fn = tt.modelfileName
				} else {
					fn = "Modelfile"
				}

				tempFile, err := os.CreateTemp(t.TempDir(), fn)
				if err != nil {
					t.Fatalf("temp modelfile creation failed: %v", err)
				}
				defer tempFile.Close()

				expectedFilename = tempFile.Name()
				err = cmd.Flags().Set("file", expectedFilename)
				if err != nil {
					t.Fatalf("couldn't set file flag: %v", err)
				}
			} else {
				expectedFilename = tt.expectedName
				if tt.modelfileName != "" {
					err := cmd.Flags().Set("file", tt.modelfileName)
					if err != nil {
						t.Fatalf("couldn't set file flag: %v", err)
					}
				}
			}

			actualFilename, actualErr := getModelfileName(cmd)

			if actualFilename != expectedFilename {
				t.Errorf("expected filename: '%s' actual filename: '%s'", expectedFilename, actualFilename)
			}

			if tt.expectedErr != os.ErrNotExist {
				if actualErr != tt.expectedErr {
					t.Errorf("expected err: %v actual err: %v", tt.expectedErr, actualErr)
				}
			} else {
				if !os.IsNotExist(actualErr) {
					t.Errorf("expected err: %v actual err: %v", tt.expectedErr, actualErr)
				}
			}
		})
	}
}

func TestPushHandler(t *testing.T) {
	tests := []struct {
		name           string
		modelName      string
		serverResponse map[string]func(w http.ResponseWriter, r *http.Request)
		expectedError  string
		expectedOutput string
	}{
		{
			name:      "successful push",
			modelName: "test-model",
			serverResponse: map[string]func(w http.ResponseWriter, r *http.Request){
				"/api/push": func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodPost {
						t.Errorf("expected POST request, got %s", r.Method)
					}

					var req api.PushRequest
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}

					if req.Name != "test-model" {
						t.Errorf("expected model name 'test-model', got %s", req.Name)
					}

					// Simulate progress updates
					responses := []api.ProgressResponse{
						{Status: "preparing manifest"},
						{Digest: "sha256:abc123456789", Total: 100, Completed: 50},
						{Digest: "sha256:abc123456789", Total: 100, Completed: 100},
					}

					for _, resp := range responses {
						if err := json.NewEncoder(w).Encode(resp); err != nil {
							http.Error(w, err.Error(), http.StatusInternalServerError)
							return
						}
						w.(http.Flusher).Flush()
					}
				},
			},
			expectedOutput: "\nYou can find your model at:\n\n\thttps://ollama.com/test-model\n",
		},
		{
			name:      "unauthorized push",
			modelName: "unauthorized-model",
			serverResponse: map[string]func(w http.ResponseWriter, r *http.Request){
				"/api/push": func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					err := json.NewEncoder(w).Encode(map[string]string{
						"error": "access denied",
					})
					if err != nil {
						t.Fatal(err)
					}
				},
			},
			expectedError: "you are not authorized to push to this namespace, create the model under a namespace you own",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if handler, ok := tt.serverResponse[r.URL.Path]; ok {
					handler(w, r)
					return
				}
				http.Error(w, "not found", http.StatusNotFound)
			}))
			defer mockServer.Close()

			t.Setenv("OLLAMA_HOST", mockServer.URL)

			cmd := &cobra.Command{}
			cmd.Flags().Bool("insecure", false, "")
			cmd.SetContext(t.Context())

			// Redirect stderr to capture progress output
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			// Capture stdout for the "Model pushed" message
			oldStdout := os.Stdout
			outR, outW, _ := os.Pipe()
			os.Stdout = outW

			err := PushHandler(cmd, []string{tt.modelName})

			// Restore stderr
			w.Close()
			os.Stderr = oldStderr
			// drain the pipe
			if _, err := io.ReadAll(r); err != nil {
				t.Fatal(err)
			}

			// Restore stdout and get output
			outW.Close()
			os.Stdout = oldStdout
			stdout, _ := io.ReadAll(outR)

			if tt.expectedError == "" {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if tt.expectedOutput != "" {
					if got := string(stdout); got != tt.expectedOutput {
						t.Errorf("expected output %q, got %q", tt.expectedOutput, got)
					}
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("expected error containing %q, got %v", tt.expectedError, err)
				}
			}
		})
	}
}

func TestListHandler(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		serverResponse []api.ListModelResponse
		expectedError  string
		expectedOutput string
	}{
		{
			name: "list all models",
			args: []string{},
			serverResponse: []api.ListModelResponse{
				{Name: "model1", Digest: "sha256:abc123", Size: 1024, ModifiedAt: time.Now().Add(-24 * time.Hour)},
				{Name: "model2", Digest: "sha256:def456", Size: 2048, ModifiedAt: time.Now().Add(-48 * time.Hour)},
			},
			expectedOutput: "NAME      ID              SIZE      MODIFIED     \n" +
				"model1    sha256:abc12    1.0 KB    24 hours ago    \n" +
				"model2    sha256:def45    2.0 KB    2 days ago      \n",
		},
		{
			name: "filter models by prefix",
			args: []string{"model1"},
			serverResponse: []api.ListModelResponse{
				{Name: "model1", Digest: "sha256:abc123", Size: 1024, ModifiedAt: time.Now().Add(-24 * time.Hour)},
				{Name: "model2", Digest: "sha256:def456", Size: 2048, ModifiedAt: time.Now().Add(-24 * time.Hour)},
			},
			expectedOutput: "NAME      ID              SIZE      MODIFIED     \n" +
				"model1    sha256:abc12    1.0 KB    24 hours ago    \n",
		},
		{
			name:          "server error",
			args:          []string{},
			expectedError: "server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/tags" || r.Method != http.MethodGet {
					t.Errorf("unexpected request to %s %s", r.Method, r.URL.Path)
					http.Error(w, "not found", http.StatusNotFound)
					return
				}

				if tt.expectedError != "" {
					http.Error(w, tt.expectedError, http.StatusInternalServerError)
					return
				}

				response := api.ListResponse{Models: tt.serverResponse}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Fatal(err)
				}
			}))
			defer mockServer.Close()

			t.Setenv("OLLAMA_HOST", mockServer.URL)

			cmd := &cobra.Command{}
			cmd.SetContext(t.Context())

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := ListHandler(cmd, tt.args)

			// Restore stdout and get output
			w.Close()
			os.Stdout = oldStdout
			output, _ := io.ReadAll(r)

			if tt.expectedError == "" {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if got := string(output); got != tt.expectedOutput {
					t.Errorf("expected output:\n%s\ngot:\n%s", tt.expectedOutput, got)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("expected error containing %q, got %v", tt.expectedError, err)
				}
			}
		})
	}
}

func TestCreateHandler(t *testing.T) {
	tests := []struct {
		name           string
		modelName      string
		modelFile      string
		serverResponse map[string]func(w http.ResponseWriter, r *http.Request)
		expectedError  string
		expectedOutput string
	}{
		{
			name:      "successful create",
			modelName: "test-model",
			modelFile: "FROM foo",
			serverResponse: map[string]func(w http.ResponseWriter, r *http.Request){
				"/api/create": func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodPost {
						t.Errorf("expected POST request, got %s", r.Method)
					}

					req := api.CreateRequest{}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}

					if req.Model != "test-model" {
						t.Errorf("expected model name 'test-model', got %s", req.Name)
					}

					if req.From != "foo" {
						t.Errorf("expected from 'foo', got %s", req.From)
					}

					responses := []api.ProgressResponse{
						{Status: "using existing layer sha256:56bb8bd477a519ffa694fc449c2413c6f0e1d3b1c88fa7e3c9d88d3ae49d4dcb"},
						{Status: "writing manifest"},
						{Status: "success"},
					}

					for _, resp := range responses {
						if err := json.NewEncoder(w).Encode(resp); err != nil {
							http.Error(w, err.Error(), http.StatusInternalServerError)
							return
						}
						w.(http.Flusher).Flush()
					}
				},
			},
			expectedOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handler, ok := tt.serverResponse[r.URL.Path]
				if !ok {
					t.Errorf("unexpected request to %s", r.URL.Path)
					http.Error(w, "not found", http.StatusNotFound)
					return
				}
				handler(w, r)
			}))
			t.Setenv("OLLAMA_HOST", mockServer.URL)
			t.Cleanup(mockServer.Close)
			tempFile, err := os.CreateTemp(t.TempDir(), "modelfile")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tempFile.Name())

			if _, err := tempFile.WriteString(tt.modelFile); err != nil {
				t.Fatal(err)
			}
			if err := tempFile.Close(); err != nil {
				t.Fatal(err)
			}

			cmd := &cobra.Command{}
			cmd.Flags().String("file", "", "")
			if err := cmd.Flags().Set("file", tempFile.Name()); err != nil {
				t.Fatal(err)
			}

			cmd.Flags().Bool("insecure", false, "")
			cmd.SetContext(t.Context())

			// Redirect stderr to capture progress output
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			// Capture stdout for the "Model pushed" message
			oldStdout := os.Stdout
			outR, outW, _ := os.Pipe()
			os.Stdout = outW

			err = CreateHandler(cmd, []string{tt.modelName})

			// Restore stderr
			w.Close()
			os.Stderr = oldStderr
			// drain the pipe
			if _, err := io.ReadAll(r); err != nil {
				t.Fatal(err)
			}

			// Restore stdout and get output
			outW.Close()
			os.Stdout = oldStdout
			stdout, _ := io.ReadAll(outR)

			if tt.expectedError == "" {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}

				if tt.expectedOutput != "" {
					if got := string(stdout); got != tt.expectedOutput {
						t.Errorf("expected output %q, got %q", tt.expectedOutput, got)
					}
				}
			}
		})
	}
}

func TestNewCreateRequest(t *testing.T) {
	tests := []struct {
		name     string
		from     string
		opts     runOptions
		expected *api.CreateRequest
	}{
		{
			"basic test",
			"newmodel",
			runOptions{
				Model:       "mymodel",
				ParentModel: "",
				Prompt:      "You are a fun AI agent",
				Messages:    []api.Message{},
				WordWrap:    true,
			},
			&api.CreateRequest{
				From:  "mymodel",
				Model: "newmodel",
			},
		},
		{
			"parent model test",
			"newmodel",
			runOptions{
				Model:       "mymodel",
				ParentModel: "parentmodel",
				Messages:    []api.Message{},
				WordWrap:    true,
			},
			&api.CreateRequest{
				From:  "parentmodel",
				Model: "newmodel",
			},
		},
		{
			"parent model as filepath test",
			"newmodel",
			runOptions{
				Model:       "mymodel",
				ParentModel: "/some/file/like/etc/passwd",
				Messages:    []api.Message{},
				WordWrap:    true,
			},
			&api.CreateRequest{
				From:  "mymodel",
				Model: "newmodel",
			},
		},
		{
			"parent model as windows filepath test",
			"newmodel",
			runOptions{
				Model:       "mymodel",
				ParentModel: "D:\\some\\file\\like\\etc\\passwd",
				Messages:    []api.Message{},
				WordWrap:    true,
			},
			&api.CreateRequest{
				From:  "mymodel",
				Model: "newmodel",
			},
		},
		{
			"options test",
			"newmodel",
			runOptions{
				Model:       "mymodel",
				ParentModel: "parentmodel",
				Options: map[string]any{
					"temperature": 1.0,
				},
			},
			&api.CreateRequest{
				From:  "parentmodel",
				Model: "newmodel",
				Parameters: map[string]any{
					"temperature": 1.0,
				},
			},
		},
		{
			"messages test",
			"newmodel",
			runOptions{
				Model:       "mymodel",
				ParentModel: "parentmodel",
				System:      "You are a fun AI agent",
				Messages: []api.Message{
					{
						Role:    "user",
						Content: "hello there!",
					},
					{
						Role:    "assistant",
						Content: "hello to you!",
					},
				},
				WordWrap: true,
			},
			&api.CreateRequest{
				From:   "parentmodel",
				Model:  "newmodel",
				System: "You are a fun AI agent",
				Messages: []api.Message{
					{
						Role:    "user",
						Content: "hello there!",
					},
					{
						Role:    "assistant",
						Content: "hello to you!",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := NewCreateRequest(tt.from, tt.opts)
			if !cmp.Equal(actual, tt.expected) {
				t.Errorf("expected output %#v, got %#v", tt.expected, actual)
			}
		})
	}
}

func TestShowLogs(t *testing.T) {
	tests := []struct {
		name           string
		logContent     string
		tail           int
		expectedOutput string
	}{
		{
			name:           "show all logs",
			logContent:     "log line 1\nlog line 2\nlog line 3\n",
			tail:           0,
			expectedOutput: "log line 1\nlog line 2\nlog line 3\n",
		},
		{
			name:           "show last 2 lines with tail",
			logContent:     "log line 1\nlog line 2\nlog line 3\nlog line 4\n",
			tail:           2,
			expectedOutput: "log line 3\nlog line 4\n",
		},
		{
			name:           "show last 1 line with tail",
			logContent:     "log line 1\nlog line 2\nlog line 3\n",
			tail:           1,
			expectedOutput: "log line 3\n",
		},
		{
			name:           "tail larger than file",
			logContent:     "line 1\nline 2\n",
			tail:           10,
			expectedOutput: "line 1\nline 2\n",
		},
		{
			name:           "empty log file",
			logContent:     "",
			tail:           0,
			expectedOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary log file
			tempFile, err := os.CreateTemp(t.TempDir(), "ollama-test-*.log")
			if err != nil {
				t.Fatal(err)
			}
			defer tempFile.Close()

			if _, err := tempFile.WriteString(tt.logContent); err != nil {
				t.Fatal(err)
			}

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err = showLogs(tempFile.Name(), tt.tail)

			// Restore stdout and get output
			w.Close()
			os.Stdout = oldStdout
			output, _ := io.ReadAll(r)

			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if got := string(output); got != tt.expectedOutput {
				t.Errorf("expected output:\n%q\ngot:\n%q", tt.expectedOutput, got)
			}
		})
	}
}

func TestShowLogsError(t *testing.T) {
	err := showLogs("/nonexistent/path/ollama.log", 0)
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
	if !strings.Contains(err.Error(), "failed to open log file") {
		t.Errorf("expected error about opening file, got %v", err)
	}
}

// TestFollowLogsCapturesNewLines verifies that followLogs streams
// new lines appended to a log file in real-time. It focuses on
// observable behaviour (output) rather than the implementation
// details of followLogs.
func TestFollowLogsCapturesNewLines(t *testing.T) {
    tmp, err := os.CreateTemp(t.TempDir(), "ollama-follow-*.log")
    if err != nil {
        t.Fatalf("failed to create temp file: %v", err)
    }
    // write an initial line so tail logic has something to read
    _, _ = tmp.WriteString("initial\n")

    // Capture Stdout so we can assert on the output produced by followLogs
    origStdout := os.Stdout
    r, w, _ := os.Pipe()
    os.Stdout = w

    ctx, cancel := context.WithCancel(context.Background())

    // run followLogs in a goroutine
    done := make(chan error, 1)
    go func() {
        // tail=1 so only the last line should be emitted initially
        done <- followLogs(ctx, tmp.Name(), 1)
    }()

    // Give the goroutine a moment to read existing lines
    time.Sleep(50 * time.Millisecond)

    // Append a new line – followLogs should emit it almost instantly
    const newLine = "second line"
    _, _ = tmp.WriteString(newLine + "\n")

    // Allow time for followLogs to pick the change, then cancel
    time.Sleep(200 * time.Millisecond)
    cancel()

    // Wait for followLogs to exit
    if err := <-done; err != nil {
        t.Fatalf("followLogs returned error: %v", err)
    }

    // Restore Stdout and collect output
    w.Close()
    os.Stdout = origStdout
    out, _ := io.ReadAll(r)

    // We expect the output to contain the appended line
    if !strings.Contains(string(out), newLine) {
        t.Fatalf("expected output to contain %q, got %q", newLine, string(out))
    }
}

// TestGetLogFilePaths verifies that the lifecycle package exposes non-empty paths for the
// server and app logs. Detailed path validation is handled in platform-specific tests
// closer to the logging implementation.
func TestGetLogFilePaths(t *testing.T) {
    if lifecycle.ServerLogFile == "" {
        t.Fatalf("lifecycle.ServerLogFile should not be empty")
    }

    if lifecycle.AppLogFile == "" {
        t.Fatalf("lifecycle.AppLogFile should not be empty")
    }

    // Basic sanity checks to ensure the file names are correct.
    if !strings.HasSuffix(lifecycle.ServerLogFile, "server.log") {
        t.Errorf("expected server log to end with server.log, got %s", lifecycle.ServerLogFile)
    }

    if !strings.HasSuffix(lifecycle.AppLogFile, "app.log") {
        t.Errorf("expected app log to end with app.log, got %s", lifecycle.AppLogFile)
    }
}
