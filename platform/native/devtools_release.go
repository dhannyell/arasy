//go:build (windows || linux || darwin) && (amd64 || arm64) && !arr_dev

package native

// mountDevPanel is the release-build stub: the dev panel compiles in only
// under the arr_dev tag, so a shipped game cannot open it — the code is not
// in the binary at all. Asking for it here means a release build kept the
// development options on, which is worth one log line.
func (h *host) mountDevPanel(opts Options) {
	if opts.DevTools || opts.RecordReplay {
		opts.Log.Warn("dev panel requested but not compiled in; rebuild with -tags arr_dev")
	}
}
