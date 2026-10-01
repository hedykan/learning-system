package agent

// Facade helpers for mobile/bind: small exported wrappers over the unexported
// pieces a gomobile facade legitimately needs.

// InitVault initializes a Learning Vault at path (idempotent).
func InitVault(path string) (string, bool) {
	r := runCLI(path, "", "init", path)
	return r.Output, r.OK
}

// SetActiveModel switches the tutor model; id must be known.
func SetActiveModel(id string) {
	conf := loadModelConf()
	for _, m := range allModels() {
		if m.ID == id {
			conf.Active = id
			_ = saveModelConf(conf)
			return
		}
	}
}

// AddModel validates a custom OpenAI-compatible model with a ping call, then
// saves and activates it. The message is empty on success.
func AddModel(name, baseURL, modelID, key string) string {
	msg, _ := addModel(map[string]any{"name": name, "base_url": baseURL, "model": modelID, "key": key})
	s, _ := msg.(map[string]any)["message"].(string)
	return s
}

// RemoveModel deletes a custom model, falling back to the builtin tutor.
func RemoveModel(id string) {
	removeModel(map[string]any{"id": id})
}
