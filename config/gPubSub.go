package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

type PubSubMessage struct {
	ID                  int       `json:"id"`
	BusinessId          string    `json:"business_id"`
	TransactionDateTime time.Time `json:"transaction_date_time"`
	ReferenceId         int       `json:"reference_id"`
	ReferenceType       string    `json:"reference_type"`
	Action              string    `json:"action"`
	OldObj              []byte    `json:"old_obj"`
	NewObj              []byte    `json:"new_obj"`
	CorrelationId       string    `json:"correlation_id"`
}

var (
	pubsubClient   *pubsub.Client
	pubsubClientMu sync.Mutex
)

func init() {
	// Load env from .env
	godotenv.Load()
}

// GetClient returns a Pub/Sub client, initializing with retries if needed.
// It uses Application Default Credentials unless PUBSUB_CREDENTIALS_JSON is provided.
func GetClient(ctx context.Context) (*pubsub.Client, error) {
	return getPubSubClient(ctx)
}

func getPubSubProjectID() string {
	// Prefer explicit override.
	if v := os.Getenv("PUBSUB_PROJECT_ID"); v != "" {
		return v
	}
	// Cloud Run/Cloud Functions often set this.
	if v := os.Getenv("GOOGLE_CLOUD_PROJECT"); v != "" {
		return v
	}
	// Common fallback.
	if v := os.Getenv("GCP_PROJECT"); v != "" {
		return v
	}
	return ""
}

func getPubSubClient(ctx context.Context) (*pubsub.Client, error) {
	pubsubClientMu.Lock()
	if pubsubClient != nil {
		c := pubsubClient
		pubsubClientMu.Unlock()
		return c, nil
	}
	pubsubClientMu.Unlock()

	projectID := getPubSubProjectID()
	if projectID == "" {
		return nil, errors.New("PUBSUB_PROJECT_ID/GOOGLE_CLOUD_PROJECT not set")
	}

	credJSON := os.Getenv("PUBSUB_CREDENTIALS_JSON")

	var attempt int
	for {
		attempt++

		var (
			c   *pubsub.Client
			err error
		)
		if credJSON != "" {
			c, err = pubsub.NewClient(ctx, projectID, option.WithCredentialsJSON([]byte(credJSON)))
		} else {
			// Uses Application Default Credentials (Cloud Run service account or GOOGLE_APPLICATION_CREDENTIALS).
			c, err = pubsub.NewClient(ctx, projectID)
		}
		if err == nil {
			pubsubClientMu.Lock()
			if pubsubClient == nil {
				pubsubClient = c
			} else {
				// Another goroutine won the race; close ours.
				_ = c.Close()
			}
			c2 := pubsubClient
			pubsubClientMu.Unlock()

			log.Printf("pubsub client ready (project_id=%s attempt=%d)", projectID, attempt)
			return c2, nil
		}

		sleep := time.Second * time.Duration(1<<min(attempt, 5))
		if sleep > 30*time.Second {
			sleep = 30 * time.Second
		}
		log.Printf("failed to init pubsub client (project_id=%s attempt=%d): %v; retrying in %s", projectID, attempt, err, sleep)
		time.Sleep(sleep)
	}
}

func CreateTopicIfNotExists(c *pubsub.Client, topic string) (*pubsub.Topic, error) {
	if c == nil {
		return nil, errors.New("pubsub client is nil")
	}
	if topic == "" {
		return nil, errors.New("topic is required")
	}

	ctx := context.Background()
	t := c.Topic(topic)
	ok, err := t.Exists(ctx)
	if err != nil {
		return nil, err
	}
	if ok {
		return t, nil
	}
	t, err = c.CreateTopic(ctx, topic)
	if err != nil {
		return nil, fmt.Errorf("create topic %q: %w", topic, err)
	}
	return t, nil
}

func CreateSubscriptionIfNotExists(client *pubsub.Client, name string, topic *pubsub.Topic) (*pubsub.Subscription, error) {
	if client == nil {
		return nil, errors.New("pubsub client is nil")
	}
	if name == "" {
		return nil, errors.New("subscription name is required")
	}
	if topic == nil {
		return nil, errors.New("topic is required")
	}

	ctx := context.Background()
	sub := client.Subscription(name)
	subExists, err := sub.Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("check subscription exists: %w", err)
	}
	if !subExists {
		sub, err = client.CreateSubscription(ctx, name, pubsub.SubscriptionConfig{
			Topic:       topic,
			AckDeadline: 20 * time.Second,
		})
		if err != nil {
			return nil, fmt.Errorf("create subscription %q: %w", name, err)
		}
	}
	return sub, nil
}

func PublishAccountingWorkflow(businessId string, msg PubSubMessage) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := PublishAccountingWorkflowWithResult(ctx, businessId, msg)
	return err
}

// PublishAccountingWorkflowWithResult publishes and returns the Pub/Sub server-assigned message ID.
func PublishAccountingWorkflowWithResult(ctx context.Context, businessId string, msg PubSubMessage) (string, error) {
	client, err := getPubSubClient(ctx)
	if err != nil {
		return "", err
	}

	topicName := os.Getenv("PUBSUB_TOPIC")
	if topicName == "" {
		return "", errors.New("PUBSUB_TOPIC is required")
	}

	t := client.Topic(topicName)
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}
	result := t.Publish(ctx, &pubsub.Message{
		Data: msgJSON,
	})

	id, err := result.Get(ctx)
	return id, err
}

func PublicIntegrationWorkflow(topicName string, obj interface{}) error {
	if os.Getenv("GO_ENV") == "" {
		return fmt.Errorf("integration not allow")
	}
	if topicName == "" {
		return errors.New("topicName is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := getPubSubClient(ctx)
	if err != nil {
		return err
	}

	t := client.Topic(topicName)
	jsonData, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	result := t.Publish(ctx, &pubsub.Message{Data: jsonData})
	_, err = result.Get(ctx)
	return err
}
