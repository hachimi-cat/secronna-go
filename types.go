package secronna

// Project is the top-level container that owns environments + secrets.
type Project struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	CreatedAt string `json:"createdAt"`
	// CreatedBy is the Huudis sub of the member who created the project.
	CreatedBy string `json:"createdBy"`
}

// Environment is an environment within a project (e.g. "production").
type Environment struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	CreatedAt string `json:"createdAt"`
}

// SecretVersion is the result of writing a secret — a freshly minted
// immutable version.
type SecretVersion struct {
	ID  string `json:"id"`
	Key string `json:"key"`
	// Version is the version number just written (1-based).
	Version int `json:"version"`
}

// SecretMeta is a secret listing entry. Values are NEVER included — only
// keys + the current version pointer. Use RevealSecret to read a value.
type SecretMeta struct {
	ID             string `json:"id"`
	Key            string `json:"key"`
	CurrentVersion int    `json:"currentVersion"`
	CreatedAt      string `json:"createdAt"`
}

// RevealedSecret is the plaintext of a single secret version — returned
// only by RevealSecret, and every reveal is written to the audit log.
type RevealedSecret struct {
	Key     string `json:"key"`
	Version int    `json:"version"`
	Value   string `json:"value"`
}

// DeletedSecret is the tombstone returned when a secret is deleted.
type DeletedSecret struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// AuditEntry is an audit-log entry — every reveal, write, and delete is
// recorded.
type AuditEntry struct {
	ID string `json:"id"`
	// Action is machine-readable, e.g. "secret.reveal", "secret.write".
	Action string `json:"action"`
	// ActorSub is the Huudis sub of the actor.
	ActorSub string `json:"actorSub"`
	// Target is the ARN / id of the resource the action touched.
	Target string `json:"target"`
	// Metadata is action-specific context (e.g. {key, version} on reveal).
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt string         `json:"createdAt"`
}
