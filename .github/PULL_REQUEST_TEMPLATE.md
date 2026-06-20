<!-- Thanks for contributing to CashFlux! Keep PRs focused — one feature/fix each. -->

## What & why

<!-- What does this change, and why? Link any issue: "Closes #123". -->

## Type

- [ ] feat  - [ ] fix  - [ ] test  - [ ] docs  - [ ] refactor  - [ ] chore

## Checklist (see CONTRIBUTING.md / CLAUDE.md)

- [ ] Built **bottom-up** (data model → tested logic → persistence → state → UI last)
- [ ] New/changed **logic has table-driven tests**; export↔import stays lossless
- [ ] `go test ./...` passes (native) and `GOOS=js GOARCH=wasm go build ./...` succeeds
- [ ] `gofmt` + `go vet` clean; no dead code; no `syscall/js` in logic packages
- [ ] Money handled as **integer minor units** (not floats); logging via `log/slog`
- [ ] **One feature per commit**, Conventional Commit subject
- [ ] Updated **CHANGELOG.md** and **DEVLOG.md**

## Screenshots / notes

<!-- For UI changes, before/after screenshots help a lot. -->
