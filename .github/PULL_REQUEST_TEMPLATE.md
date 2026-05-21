## What this changes

One or two sentences. The diff explains the *what*; this section explains the *why*.

## Linked issue

Closes #___ (or "no issue — small fix" / "discussed in #___")

## Checklist

- [ ] Tests added for the new behavior or bug fix (PRs without tests get bounced)
- [ ] `cd apps/cli && go vet ./...` clean
- [ ] `cd apps/cli && go test ./... -count=1` green
- [ ] If this touches the differ / inspector / codegen — matrix still 130/130 (`BIN=$(pwd)/pg-flux bash test/matrix/run.sh`)
- [ ] Docs updated if user-visible behavior changed (`apps/web/content/docs/`)
- [ ] `CHANGELOG.md` `[Unreleased]` section updated if user-visible

## Anything reviewers should pay extra attention to

(Performance hotspot? Backwards-compat risk? An ugly hack you'd like a second opinion on?)
