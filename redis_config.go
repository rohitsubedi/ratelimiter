package ratelimiter

type RedisConfig struct {
	Host     string
	Password string
}

func (r *RedisConfig) GetHost() string {
	if r == nil {
		return ""
	}

	return r.Host
}

func (r *RedisConfig) GetPassword() string {
	if r == nil {
		return ""
	}

	return r.Password
}
