package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
)

// SNSPublisher — publikon në topic-un `domain-events` (Terraform: modules/messaging).
// Atributet e mesazhit lejojnë filtrim në abonimet SQS (p.sh. vetëm `Ride*` te dispatch).
type SNSPublisher struct {
	client   *sns.Client
	topicARN string
}

func NewSNSPublisher(ctx context.Context, region, topicARN string) (*SNSPublisher, error) {
	if topicARN == "" {
		return nil, errors.New("events: SNS_DOMAIN_EVENTS_TOPIC_ARN mungon")
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("events: aws config: %w", err)
	}
	return &SNSPublisher{client: sns.NewFromConfig(awsCfg), topicARN: topicARN}, nil
}

func (p *SNSPublisher) Publish(ctx context.Context, ev Event) error {
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	_, err = p.client.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(p.topicARN),
		Message:  aws.String(string(body)),
		MessageAttributes: map[string]types.MessageAttributeValue{
			"event_type":     {DataType: aws.String("String"), StringValue: aws.String(ev.EventType)},
			"aggregate_type": {DataType: aws.String("String"), StringValue: aws.String(ev.AggregateType)},
		},
	})
	return err
}

// DevLogPublisher — VETËM development: ngjarjet logohen, nuk dërgohen askund. Refuzohet jashtë dev.
type DevLogPublisher struct{ log *slog.Logger }

func (p *DevLogPublisher) Publish(_ context.Context, ev Event) error {
	p.log.Info("DEV ONLY — domain event (not delivered anywhere)",
		"id", ev.ID, "type", ev.EventType, "aggregate", ev.AggregateType+":"+ev.AggregateID)
	return nil
}

// NewPublisherFromEnv zgjedh publikuesin sipas EVENTS_PUBLISHER (sns | devlog).
func NewPublisherFromEnv(ctx context.Context, env, kind, region, topicARN string, log *slog.Logger) (Publisher, error) {
	switch kind {
	case "sns":
		return NewSNSPublisher(ctx, region, topicARN)
	case "devlog":
		if env != "development" {
			return nil, fmt.Errorf("events: publisher devlog lejohet vetëm në development (APP_ENV=%s)", env)
		}
		log.Warn("DEV ONLY — EVENTS_PUBLISHER=devlog: ngjarjet vetëm logohen, nuk dërgohen në SNS")
		return &DevLogPublisher{log: log}, nil
	default:
		return nil, fmt.Errorf("events: publisher i panjohur %q (sns | devlog)", kind)
	}
}
