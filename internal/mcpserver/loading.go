package mcpserver

import (
	"context"
	"fmt"
	"time"

	"mcp-server-susedoc/internal/docparse"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// progressInterval is how often a request waiting for the corpus to load
// sends a progress notification, so that the client does not give up on it.
const progressInterval = time.Second

// PendingCorpus is a corpus that may still be loading in the background, so
// that the MCP server can answer initialize and tools/list right away. Tool
// calls and resource reads wait for it with Wait.
type PendingCorpus struct {
	ready  chan struct{} // closed once corpus or err is set
	corpus *docparse.Corpus
	err    error
	wait   time.Duration // how long a request waits, 0 for no limit
}

// Ready returns a PendingCorpus that is already loaded with c.
func Ready(c *docparse.Corpus) *PendingCorpus {
	p := &PendingCorpus{ready: make(chan struct{}), corpus: c}
	close(p.ready)
	return p
}

// LoadInBackground runs load in a goroutine. If it has not finished after
// timeout, its context is cancelled and the corpus fails with a timeout error;
// a timeout of 0 lets it take as long as it needs. A request waiting for the
// corpus gives up after wait, see Wait, while the load goes on.
func LoadInBackground(ctx context.Context, timeout, wait time.Duration, load func(context.Context) (*docparse.Corpus, error)) *PendingCorpus {
	p := &PendingCorpus{ready: make(chan struct{}), wait: wait}
	go func() {
		var cancel context.CancelFunc
		if timeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, timeout)
		} else {
			ctx, cancel = context.WithCancel(ctx)
		}
		defer cancel()
		type result struct {
			corpus *docparse.Corpus
			err    error
		}
		done := make(chan result, 1)
		go func() {
			c, err := load(ctx)
			done <- result{c, err}
		}()
		select {
		case r := <-done:
			p.corpus, p.err = r.corpus, r.err
		case <-ctx.Done():
			// load may still be running, its result is dropped
		}
		if ctx.Err() != nil && (p.corpus == nil || p.err != nil) {
			p.corpus, p.err = nil, fmt.Errorf("timed out after %s", timeout)
		}
		if p.err != nil {
			p.err = fmt.Errorf("loading documentation corpus: %w", p.err)
		}
		close(p.ready)
	}()
	return p
}

// loaded returns the corpus if it has been loaded successfully and nil if it
// is still loading or failed to load.
func (p *PendingCorpus) loaded() *docparse.Corpus {
	select {
	case <-p.ready:
		return p.corpus
	default:
		return nil
	}
}

// Wait returns the corpus once it is loaded, or the error loading it. While
// waiting it sends progress notifications for req if the client asked for
// them with a progress token. A request gives up after the wait duration of
// LoadInBackground with an error asking the client to try again, the load is
// not affected; req may be nil to wait without a limit.
func (p *PendingCorpus) Wait(ctx context.Context, req mcp.Request) (*docparse.Corpus, error) {
	select {
	case <-p.ready:
		return p.corpus, p.err
	default:
	}
	var session *mcp.ServerSession
	var token any
	if req != nil {
		session, _ = req.GetSession().(*mcp.ServerSession)
		if params, ok := req.GetParams().(mcp.RequestParams); ok {
			token = params.GetProgressToken()
		}
	}
	debugf("waiting for the corpus to load, progress token %v", token)
	ticker := time.NewTicker(progressInterval)
	defer ticker.Stop()
	var giveUp <-chan time.Time
	if req != nil && p.wait > 0 {
		timer := time.NewTimer(p.wait)
		defer timer.Stop()
		giveUp = timer.C
	}
	for n := 1; ; n++ {
		select {
		case <-p.ready:
			return p.corpus, p.err
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-giveUp:
			return nil, fmt.Errorf("documentation corpus is still loading after %s, try again later", p.wait)
		case <-ticker.C:
			if session != nil && token != nil {
				_ = session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
					ProgressToken: token,
					Message:       "loading documentation corpus",
					Progress:      float64(n),
				})
			}
		}
	}
}
