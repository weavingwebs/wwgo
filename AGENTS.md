# Agent instructions

Shared Go library used by most `weavingwebs` Go projects, so think about the knock-on effect on consumers before changing exported APIs.

- Brevity is good.
- **Do not use mdash (—) characters.**
- Your edit file tool may automatically run `go fmt`/`eslint --fix` style tools after edits, don't assume the file to stay as you expect and don't try and remove imports before removing their usage (they may keep coming back!).
- Multi-module repo: core `github.com/weavingwebs/wwgo` plus submodules with their own `go.mod` (`wwgraphql`, `wwaws`, `wwauth`, `wwstripe/stripev*`) so heavy/fast-moving deps (gqlgen, AWS SDK, Stripe SDK) aren't forced on every consumer. Keep it that way: new heavy deps go in a submodule or behind an interface in core (see `wwstripe.StripeClient`, `wwalert.SecretFetcher`).
- Submodules depend on a **tagged** core version, so core changes they need must be released first. Tags are `vX.Y.Z` for core & `<dir>/vX.Y.Z` for submodules.
- Don't commit a `go.work`; use one outside the repo via `GOWORK=...` for local cross-module dev.
- Run `go build ./... && go vet ./...` in every module you touch (there are few tests).
- `wwdb.OpenDb` overrides `sqlx.NameMapper` so Go struct field names are assumed to be camelCase in the SQL.
