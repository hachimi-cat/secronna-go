# secronna-go

Typed Go client for the [secronna.com](https://secronna.com) secrets
REST API.

```bash
go get github.com/hachimi-cat/secronna-go
```

```go
import "github.com/hachimi-cat/secronna-go"

c := secronna.New(secronna.Config{Token: os.Getenv("SECRONNA_TOKEN")})

project, err := c.CreateProject(ctx, "Web")
env, err := c.CreateEnvironment(ctx, project.ID, "production")
_, err = c.SetSecret(ctx, env.ID, "DATABASE_URL", "postgres://…")

// Pull the whole environment into a map[KEY]value:
secrets, err := c.FetchSecrets(ctx, env.ID)
// map[DATABASE_URL:postgres://…]
```

Auth is a Bearer token — an `sk_live_…` API key from the dashboard (or a
Huudis-minted access token). Pass `Config.Token` or set `SECRONNA_TOKEN`.

Every response rides the Forjio envelope `{data, error, meta}`; the
client unwraps `data` and returns a `*secronna.Error` (carrying the
envelope's `error.code`) on failure.

## Every route: `c.API`

`c.API.<Area><Action>(ctx, pathParams…, *<Area><Action>Args)` covers every
feature route of the API — generated from the API spec (`api_generated.go`,
by `scripts/apigen.sh`) — with the same bearer and envelope handling as the
methods above, and returns the response's `data` as `json.RawMessage`.
Required fields are plain values, optional ones pointers (`secronna.Ptr`),
slices or maps; `Body` passes the whole JSON body.

```go
data, err := c.API.SecretsReveal(ctx, secretID, &secronna.SecretsRevealArgs{Version: secronna.Ptr(2)})
```

## Family

Sister to:

- [`@forjio/secronna`](https://www.npmjs.com/package/@forjio/secronna) (JS/TS)
- [`forjio-secronna`](https://pypi.org/project/forjio-secronna/) (Python)
