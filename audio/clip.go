package audio

// ClipID names a registered audio asset. Games fire cues and tracks by ID;
// hosts resolve the ID to real bytes via the clip registry in the runtime
// config. IDs are semantic ("brick_break"), never file names.
type ClipID string

// Clip registers an audio asset: a semantic ID and a host-resolved path
// (relative to whatever asset source the host mounts — a fs.FS on web,
// a directory on native).
type Clip struct {
	ID   ClipID
	Path string
}
