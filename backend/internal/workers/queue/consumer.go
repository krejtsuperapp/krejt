// Package queue — konsumator SQS (§43): long polling, përpunim për mesazh, fshirje vetëm pas suksesit;
// dështimi e lë mesazhin të rishfaqet (visibility timeout) dhe pas maxReceiveCount shkon në DLQ (alarm).
// Mesazhet e pakuptueshme (jo Event) hidhen me log ERROR — s'ka kuptim të riprovohen.
package queue

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/events"
)

type Handler func(ctx context.Context, ev events.Event) error

type Consumer struct {
	client  *sqs.Client
	url     string
	name    string
	handler Handler
	log     *slog.Logger
}

func New(ctx context.Context, region, queueURL, name string, h Handler, log *slog.Logger) (*Consumer, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, err
	}
	return &Consumer{client: sqs.NewFromConfig(awsCfg), url: queueURL, name: name, handler: h, log: log}, nil
}

func (c *Consumer) Run(ctx context.Context) {
	c.log.Info("queue consumer started", "queue", c.name)
	for ctx.Err() == nil {
		out, err := c.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(c.url),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20,
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.log.Error("sqs receive", "queue", c.name, "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}
		for _, m := range out.Messages {
			var ev events.Event
			if err := json.Unmarshal([]byte(aws.ToString(m.Body)), &ev); err != nil || ev.ID == uuid.Nil || ev.EventType == "" {
				c.log.Error("sqs poison message dropped", "queue", c.name, "message_id", aws.ToString(m.MessageId), "err", err)
				c.delete(ctx, m.ReceiptHandle)
				continue
			}
			hctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			herr := c.handler(hctx, ev)
			cancel()
			if herr != nil {
				c.log.Warn("handler failed, message will be retried", "queue", c.name, "event_id", ev.ID, "type", ev.EventType, "err", herr)
				continue
			}
			c.delete(ctx, m.ReceiptHandle)
		}
	}
}

func (c *Consumer) delete(ctx context.Context, receipt *string) {
	dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if _, err := c.client.DeleteMessage(dctx, &sqs.DeleteMessageInput{QueueUrl: aws.String(c.url), ReceiptHandle: receipt}); err != nil {
		c.log.Error("sqs delete", "queue", c.name, "err", err)
	}
}
