package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mcp-server-susedoc/internal/docparse"
	"mcp-server-susedoc/internal/docparse/loader"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// connectPending connects a client to a server for p.
func connectPending(t *testing.T, p *PendingCorpus, opts *mcp.ClientOptions) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	_, err := NewLoadingServer(p, Options{}).Connect(ctx, st, nil)
	require.NoError(t, err)
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, opts).Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// toolSchema returns the input schema of the tool name as listed by cs.
func toolSchema(t *testing.T, cs *mcp.ClientSession, name string) map[string]any {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	for _, tool := range res.Tools {
		if tool.Name == name {
			schema, _ := tool.InputSchema.(map[string]any)
			return schema
		}
	}
	t.Fatalf("tool %s not listed", name)
	return nil
}

// productEnum returns the enum of the product property of schema.
func productEnum(schema map[string]any) any {
	props, _ := schema["properties"].(map[string]any)
	product, _ := props["product"].(map[string]any)
	return product["enum"]
}

// TestLoadingServer checks that the tools are listed while the corpus is
// loading, that a tool call waits for it and reports progress meanwhile, and
// that the tools get their enums with a list_changed notification once the
// corpus is loaded.
func TestLoadingServer(t *testing.T) {
	root := t.TempDir()
	for name, content := range docbookFiles {
		p := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	files, err := loader.Find(root)
	require.NoError(t, err)

	release := make(chan struct{})
	p := LoadInBackground(context.Background(), time.Minute, 0, func(context.Context) (*docparse.Corpus, error) {
		<-release
		return loader.Load(context.Background(), files)
	})
	progress := make(chan float64, 10)
	listChanged := make(chan struct{}, 1)
	cs := connectPending(t, p, &mcp.ClientOptions{
		ProgressNotificationHandler: func(_ context.Context, req *mcp.ProgressNotificationClientRequest) {
			progress <- req.Params.Progress
		},
		ToolListChangedHandler: func(context.Context, *mcp.ToolListChangedRequest) {
			select {
			case listChanged <- struct{}{}:
			default:
			}
		},
	})

	assert.Nil(t, productEnum(toolSchema(t, cs, "list_documents")), "enum before the corpus is loaded")

	type result struct {
		res *mcp.CallToolResult
		err error
	}
	done := make(chan result, 1)
	go func() {
		params := &mcp.CallToolParams{Name: "list_documents", Arguments: map[string]any{}}
		params.SetProgressToken("load")
		res, err := cs.CallTool(context.Background(), params)
		done <- result{res, err}
	}()
	select {
	case <-progress:
	case <-time.After(5 * time.Second):
		t.Fatal("no progress notification while loading")
	}
	select {
	case <-done:
		t.Fatal("tool call returned before the corpus was loaded")
	default:
	}

	close(release)
	r := <-done
	require.NoError(t, r.err)
	require.False(t, r.res.IsError, "%+v", r.res)

	select {
	case <-listChanged:
	case <-time.After(5 * time.Second):
		t.Fatal("no tools/list_changed notification")
	}
	assert.NotEmpty(t, productEnum(toolSchema(t, cs, "list_documents")), "enum after the corpus is loaded")
}

// TestLoadingServerTimeout checks that tool calls fail if the corpus does not
// load in time, and that the load is cancelled.
func TestLoadingServerTimeout(t *testing.T) {
	cancelled := make(chan struct{})
	p := LoadInBackground(context.Background(), 50*time.Millisecond, 0, func(ctx context.Context) (*docparse.Corpus, error) {
		<-ctx.Done()
		close(cancelled)
		return nil, ctx.Err()
	})
	cs := connectPending(t, p, nil)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_glossary", Arguments: map[string]any{}})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	require.Len(t, res.Content, 1)
	assert.Contains(t, res.Content[0].(*mcp.TextContent).Text, "timed out")
	<-cancelled
}

// TestLoadingServerWait checks that a tool call gives up waiting for the
// corpus after the wait duration while the load goes on, so that a later call
// gets the loaded corpus.
func TestLoadingServerWait(t *testing.T) {
	root := t.TempDir()
	for name, content := range docbookFiles {
		p := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	files, err := loader.Find(root)
	require.NoError(t, err)

	release := make(chan struct{})
	p := LoadInBackground(context.Background(), 0, 50*time.Millisecond, func(ctx context.Context) (*docparse.Corpus, error) {
		<-release
		return loader.Load(ctx, files)
	})
	cs := connectPending(t, p, nil)
	params := &mcp.CallToolParams{Name: "list_documents", Arguments: map[string]any{}}
	res, err := cs.CallTool(context.Background(), params)
	require.NoError(t, err)
	assert.True(t, res.IsError)
	require.Len(t, res.Content, 1)
	assert.Contains(t, res.Content[0].(*mcp.TextContent).Text, "still loading")

	close(release)
	_, err = p.Wait(context.Background(), nil)
	require.NoError(t, err)
	res, err = cs.CallTool(context.Background(), params)
	require.NoError(t, err)
	assert.False(t, res.IsError, "%+v", res)
}

// TestLoadCancelled checks that loading stops once its context is done.
func TestLoadCancelled(t *testing.T) {
	root := t.TempDir()
	for name, content := range docbookFiles {
		p := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	files, err := loader.Find(root)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = loader.Load(ctx, files)
	assert.ErrorIs(t, err, context.Canceled)
}
