package ratelimiter

type RedisConfig struct {
	Host     string
	Password string
}

func (r *RedisConfig) getHost() string {
	if r == nil {
		return ""
	}

	return r.Host
}

func (r *RedisConfig) getPassword() string {
	if r == nil {
		return ""
	}

	return r.Password
}
