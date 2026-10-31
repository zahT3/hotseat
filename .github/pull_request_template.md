## What and why

## How it was tested

- [ ] `go vet ./... && go test ./...`
- [ ] `macos/test.sh` (if the app changed)
- [ ] Tried it on a real Mac, if it touches Claude Desktop or the menu bar

## Checklist

- [ ] No code path reads, copies or prints credentials
- [ ] Nothing switches accounts without the user asking
- [ ] Replaced or removed user data goes to a backup
- [ ] CHANGELOG.md updated for user-visible changes
