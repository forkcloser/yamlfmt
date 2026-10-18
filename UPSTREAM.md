# Upstream

Fork of https://github.com/google/yamlfmt (`main`).

- Forked from: [`dea0b79`](https://github.com/google/yamlfmt/commit/dea0b791be9796b82c2e7cf6864cb7dc36023d0b), committed 2026-10-13
- Reviewed through: [`dea0b79`](https://github.com/google/yamlfmt/commit/dea0b791be9796b82c2e7cf6864cb7dc36023d0b), reviewed 2026-10-14

## Incorporated

| Upstream commit | Here | What |
|---|---|---|

## Not incorporated

| Upstream commit | Why |
|---|---|

## Changes upstream does not have

| Change | Here |
| --- | --- |
| `ConsecutiveEngine` reports a path given twice instead of formatting it twice | https://github.com/forkcloser/yamlfmt/pull/2 |
| `preserve_comment_indents` keeps line comments at their source column | https://github.com/forkcloser/yamlfmt/pull/3 |
| `exclude` matches paths, not strings, in the default collector | https://github.com/forkcloser/yamlfmt/pull/4 |
| `line_ending` defaults to `lf` on every platform | https://github.com/forkcloser/yamlfmt/pull/5 |
| Retaining line breaks leaves block scalars' blank lines alone | https://github.com/forkcloser/yamlfmt/pull/21 |
| The emitter keeps a folded scalar's more-indented line as it is | https://github.com/forkcloser/yamlfmt/pull/25 |
| Retaining line breaks leaves quoted and plain multi-line scalars alone | https://github.com/forkcloser/yamlfmt/pull/26 |
| Retaining line breaks keeps an input's missing final newline | https://github.com/forkcloser/yamlfmt/pull/27 |
| Retaining line breaks keeps a block scalar's trailing lines of spaces | https://github.com/forkcloser/yamlfmt/pull/28 |
| The emitter keeps a block scalar's leading empty line | https://github.com/forkcloser/yamlfmt/pull/29 |
| An empty document keeps its marker and its comments | https://github.com/forkcloser/yamlfmt/pull/30 |
| `gitignore_excludes` finds the repository's gitignore and matches from it | https://github.com/forkcloser/yamlfmt/pull/31 |
| The configuration lookup is deterministic and does not dereference a failed stat | https://github.com/forkcloser/yamlfmt/pull/32 |
| The `line` and `gitlab` outputs report each file's own finding | https://github.com/forkcloser/yamlfmt/pull/33 |
| An `Unmarshaler` that wraps a `*TypeError` reports type errors, not a hard failure | https://github.com/forkcloser/yamlfmt/pull/34 |
| The empty value of a flow-sequence pair is positioned on the token after the `:`, as libyaml does | https://github.com/forkcloser/yamlfmt/pull/49 |
| The configuration decodes through the vendored YAML library instead of mapstructure | https://github.com/forkcloser/yamlfmt/pull/53 |
| `strip_directives` no longer panics beside `retain_line_breaks`, `trim_trailing_whitespace` or `eof_newline` | https://github.com/forkcloser/yamlfmt/pull/54 |
| `strip_directives` restores every directive of a run, not only the first | https://github.com/forkcloser/yamlfmt/pull/55 |
