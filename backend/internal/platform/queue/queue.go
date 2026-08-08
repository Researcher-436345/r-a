package queue

import (
	"encoding/json"
	"net/url"

	"github.com/hibiken/asynq"
)

const (
	ProcessArxivPDF     = "process_arxiv_pdf"
	FinalizeUploadedPDF = "finalize_uploaded_pdf"
)

type Payload struct {
	VersionID string `json:"version_id"`
}

func redisOptions(raw string) (asynq.RedisClientOpt, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return asynq.RedisClientOpt{}, err
	}
	return asynq.RedisClientOpt{Addr: u.Host, Password: func() string {
		if u.User != nil {
			p, _ := u.User.Password()
			return p
		}
		return ""
	}(), DB: 0}, nil
}
func NewClient(redisURL string) (*asynq.Client, error) {
	o, e := redisOptions(redisURL)
	if e != nil {
		return nil, e
	}
	return asynq.NewClient(o), nil
}
func NewServer(redisURL string) (*asynq.Server, error) {
	o, e := redisOptions(redisURL)
	if e != nil {
		return nil, e
	}
	return asynq.NewServer(o, asynq.Config{Concurrency: 5}), nil
}
func Enqueue(c *asynq.Client, typ, versionID string) error {
	b, _ := json.Marshal(Payload{VersionID: versionID})
	_, err := c.Enqueue(asynq.NewTask(typ, b))
	return err
}
func Decode(t *asynq.Task) (Payload, error) { var p Payload; return p, json.Unmarshal(t.Payload(), &p) }
