// Package asynqx wraps github.com/hibiken/asynq with project-local
// conventions (queue names, task types, default options) and supplies a
// Mux pre-wired with the proxy_VPN periodic / dispatched task handlers.
package asynqx

import (
	"github.com/hibiken/asynq"
)

const (
	QueueDefault  = "default"
	QueueScheduler = "scheduler"
)

// Config carries Redis connection + concurrency knobs.
type Config struct {
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	Concurrency   int
}

// RedisOpt converts to asynq's RedisClientOpt.
func (c Config) RedisOpt() asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:     c.RedisAddr,
		Password: c.RedisPassword,
		DB:       c.RedisDB,
	}
}

// NewClient returns an asynq client connected via the given config.
func NewClient(c Config) *asynq.Client {
	return asynq.NewClient(c.RedisOpt())
}

// NewServer returns an asynq Server with sensible defaults.
func NewServer(c Config) *asynq.Server {
	if c.Concurrency <= 0 {
		c.Concurrency = 8
	}
	return asynq.NewServer(c.RedisOpt(), asynq.Config{
		Concurrency: c.Concurrency,
		Queues: map[string]int{
			QueueDefault:  6,
			QueueScheduler: 2,
		},
	})
}

// NewScheduler returns an asynq PeriodicTaskManager-friendly Scheduler.
func NewScheduler(c Config) *asynq.Scheduler {
	return asynq.NewScheduler(c.RedisOpt(), &asynq.SchedulerOpts{})
}
