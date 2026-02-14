package config

// Merge combines two configs using hybrid merge strategy:
//   - Scalar fields (image, dockerfile, etc.): override takes precedence if non-zero
//   - Map fields (env): keys are merged, override keys win
//   - List fields (volumes): append override to base
//
// Returns a new Config with the merged values.
func Merge(base, override Config) Config {
	result := base

	// Scalar overrides
	if override.Runtime != "" {
		result.Runtime = override.Runtime
	}
	if override.Image != "" {
		result.Image = override.Image
	}
	if override.WorkingDir != "" {
		result.WorkingDir = override.WorkingDir
	}
	if override.Dockerfile != "" {
		result.Dockerfile = override.Dockerfile
	}
	if override.Network != "" {
		result.Network = override.Network
	}
	if override.StopTimeout != 0 {
		result.StopTimeout = override.StopTimeout
	}
	if override.TTYRetries != 0 {
		result.TTYRetries = override.TTYRetries
	}
	if override.RetryDelay != 0 {
		result.RetryDelay = override.RetryDelay
	}
	if override.Git.User.Name != "" {
		result.Git.User.Name = override.Git.User.Name
	}
	if override.Git.User.Email != "" {
		result.Git.User.Email = override.Git.User.Email
	}

	// Env map merge
	result.Env = MergeEnv(base.Env, override.Env)

	// Volumes list append
	result.Volumes = append(result.Volumes, override.Volumes...)

	return result
}

// MergeEnv merges two environment variable maps.
// Keys from override take precedence over keys in base.
// Returns a new map with merged values.
func MergeEnv(base, override map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
}
