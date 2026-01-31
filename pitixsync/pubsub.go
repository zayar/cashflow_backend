package pitixsync

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"

	"cloud.google.com/go/pubsub"
	"github.com/gin-gonic/gin"
	"github.com/mmdatafocus/books_backend/config"
)

func PublishSyncRun(ctx context.Context, runId uint, businessId string, connectionId uint) error {
	topicName := strings.TrimSpace(os.Getenv("PITIX_SYNC_TOPIC"))
	if topicName == "" {
		topicName = "pitix-sync"
	}

	client, err := config.GetClient(ctx)
	if err != nil {
		return err
	}

	topic := client.Topic(topicName)
	if envBoolDefault("PITIX_SYNC_CREATE_TOPIC", false) {
		topic, err = config.CreateTopicIfNotExists(client, topicName)
		if err != nil {
			return err
		}
	}

	payload := SyncPubSubPayload{
		RunId:        runId,
		BusinessId:   businessId,
		ConnectionId: connectionId,
	}
	data, _ := json.Marshal(payload)
	res := topic.Publish(ctx, &pubsub.Message{Data: data})
	_, err = res.Get(ctx)
	return err
}

func PubSubPushHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !envBoolDefault("ENABLE_PITIX_PUBSUB_PUSH_ENDPOINT", true) {
			c.Status(204)
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Status(204)
			return
		}

		var envelope PubSubPushEnvelope
		if err := json.Unmarshal(body, &envelope); err != nil {
			c.Status(204)
			return
		}

		var payload SyncPubSubPayload
		if err := json.Unmarshal(envelope.Message.Data, &payload); err != nil {
			c.Status(204)
			return
		}
		if payload.RunId == 0 || payload.BusinessId == "" {
			c.Status(204)
			return
		}

		_ = processSyncRun(c.Request.Context(), payload)
		c.Status(204)
	}
}

func envBoolDefault(key string, def bool) bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch val {
	case "true", "1", "yes", "y", "on":
		return true
	case "false", "0", "no", "n", "off":
		return false
	default:
		return def
	}
}
