package controller

import "os"

var SleepInterval = 10
var ReplicasLimit = 20
var CPU = "cpu"
var GPU = "gpu"

// getEnv returns the value of the environment variable named by key,
// or fallback if the variable is unset or empty.
func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// Redis connection settings, read from the controller's environment so that
// no host/credential is baked into the source. Defaults target a typical
// in-cluster Redis; override via the env vars below.
func RedisAddr() string {
	return getEnv("REDIS_ADDR", "redis-master.redis.svc.cluster.local:6379")
}

func RedisPassword() string {
	return getEnv("REDIS_PASSWORD", "")
}

// RedisHost is the host (without port) injected into inference pods.
func RedisHost() string {
	return getEnv("REDIS_HOST", "redis-master.redis.svc.cluster.local")
}

// RedisSecretName / RedisSecretPasswordKey identify the Secret used to inject
// the Redis password into inference pods (via SecretKeyRef) instead of a
// plaintext value.
func RedisSecretName() string {
	return getEnv("REDIS_SECRET_NAME", "redis")
}

func RedisSecretPasswordKey() string {
	return getEnv("REDIS_SECRET_PASSWORD_KEY", "redis-password")
}
