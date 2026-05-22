# Project conventions

Conventions that aren't enforced by `go vet` / the compiler but are
enforced by our linter setup. Follow these when writing new code so we
don't get repeat review comments.

## Linting

### `noctx`: network calls must take an explicit context

The `noctx` linter flags any network operation that doesn't accept a
`context.Context`. The flagged shorthand functions are wrappers that
internally call the context-aware form with `context.Background()`,
so the rule is purely about making context propagation visible at the
call site — there is no behavior change.

| Don't write           | Write instead                                                  |
|-----------------------|----------------------------------------------------------------|
| `net.Listen(...)`     | `(&net.ListenConfig{}).Listen(ctx, ...)`                       |
| `net.Dial(...)`       | `(&net.Dialer{}).DialContext(ctx, ...)`                        |
| `http.Get(url)`       | `req, _ := http.NewRequestWithContext(ctx, "GET", url, nil); http.DefaultClient.Do(req)` |

#### Which context to pass

- **Production code** (e.g. `driver.Run`, `server.Run`): pass
  `context.Background()`. We don't have request-scoped context at the
  `Run` boundary; the explicit `Background` makes the intent obvious to
  the linter and to readers.
- **Test code**: pass `t.Context()` (Go 1.24+). It's cancelled when the
  test ends, so a listen/dial attempt is aborted automatically if the
  test is interrupted or times out.

#### Canonical examples

Production (`pkg/server/server.go`, `pkg/driver/driver.go`):

```go
var lc net.ListenConfig
lis, err := lc.Listen(context.Background(), "tcp", addr)
```

Test (`pkg/driver/controller_test.go`):

```go
var lc net.ListenConfig
lis, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
```

### `nilerr`: by design in the CSSI server-side RPC, suppress with rationale

`nilerr` flags code that receives a non-nil error and then returns
`nil` as the function's error value. In `pkg/server/server.go`, that's
**intentional**: the CSSI gRPC contract distinguishes transport
failures (returned as a gRPC `error`) from application failures
(returned as `Success=false` with a `Reason` *inside* the response
payload). The server translates LVM errors into the payload, so it
deliberately returns `nil` at the gRPC layer.

Don't rewrite the handler to return a gRPC `status.Errorf` — that
would break the protocol. Instead suppress `nilerr` with a comment
that names the protocol reason, so the next reader knows it's design,
not oversight:

```go
handle, err := s.lvm.CreateVolume(...)
if err != nil {
    //nolint:nilerr // CSSI protocol returns application errors in the payload (Success/Reason), not as gRPC status.
    return &cssiv1.CreateVolumeResponse{
        Success: false,
        Reason:  err.Error(),
    }, nil
}
```

This is the only place this pattern is used today. If you find
yourself wanting a `nolint:nilerr` outside `pkg/server/`, stop and
think — the driver side (`pkg/driver/controller.go`) *does* use gRPC
status codes (`codes.AlreadyExists`, `codes.Internal`) and should
keep doing so.

### `errcheck`: don't drop errors from `Close()` and friends

`defer foo.Close()` silently discards the error that `Close` returns,
which `errcheck` flags. Two patterns we use:

- **Production code** — wrap the defer in a closure and log:

  ```go
  defer func() {
      if cerr := cli.Close(); cerr != nil {
          log.Printf("CreateVolume: closing CSSI client to %s:%d: %v", host, port, cerr)
      }
  }()
  ```

  Don't *return* the close error from a handler that already produced a
  response: if the operation succeeded, turning the response into a
  failure forces a wasteful retry; if it already failed, the close
  error masks the real cause.

- **Test code** — register cleanup via `t.Cleanup` and surface failures
  with `t.Errorf`:

  ```go
  t.Cleanup(func() {
      if err := m.Close(); err != nil {
          t.Errorf("provisioner.Close: %v", err)
      }
  })
  ```

  Prefer `t.Cleanup` over `defer` in tests: it runs even on `t.Fatal`/
  panic, and it can be registered inside a helper so the call site
  doesn't need to know what to clean up.
