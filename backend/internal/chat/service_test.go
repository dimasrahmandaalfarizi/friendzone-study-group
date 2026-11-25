package chat

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newChatRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return srv, client
}

func TestPublishGlobalValidation(t *testing.T) {
	_, rdb := newChatRedis(t)
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := NewService(db, rdb)

	if _, err := service.PublishGlobal(context.Background(), "u1", "  "); !errors.Is(err, ErrEmptyMessage) {
		t.Fatalf("expected ErrEmptyMessage, got %v", err)
	}
	if _, err := service.PublishGlobal(context.Background(), "u1", strings.Repeat("x", 501)); !errors.Is(err, ErrMessageLong) {
		t.Fatalf("expected ErrMessageLong, got %v", err)
	}
}

func TestPublishGlobalRateLimitAndPublish(t *testing.T) {
	srv, rdb := newChatRedis(t)
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.MatchExpectationsInOrder(true)
	for i := 0; i < 5; i++ {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT u.name")).WithArgs("u1").
			WillReturnRows(sqlmock.NewRows([]string{"name", "picture", "university"}).
				AddRow("Alice", "pic.jpg", "ITB"))
	}
	service := NewService(db, rdb)
	ctx := context.Background()

	pubsub := rdb.Subscribe(ctx, globalChannelKey)
	defer pubsub.Close()
	_, _ = pubsub.Receive(ctx)

	for i := 0; i < 5; i++ {
		if _, err := service.PublishGlobal(ctx, "u1", "hello"); err != nil {
			t.Fatalf("message %d returned error: %v", i+1, err)
		}
	}
	if _, err := service.PublishGlobal(ctx, "u1", "sixth"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited on sixth message, got %v", err)
	}

	select {
	case redisMsg := <-pubsub.Channel():
		if !strings.Contains(redisMsg.Payload, `"content":"hello"`) {
			t.Fatalf("unexpected published payload: %s", redisMsg.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("global message was not published")
	}

	if srv.Exists("chat:history:global") {
		t.Fatal("global chat must not store history in Redis")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyWorkspaceMemberAndWorkspaceMessage(t *testing.T) {
	_, rdb := newChatRedis(t)
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT EXISTS").WithArgs("ws1", "u1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	service := NewService(db, rdb)
	if err := service.VerifyWorkspaceMember(context.Background(), "ws1", "u1"); !errors.Is(err, ErrForbiddenAccess) {
		t.Fatalf("expected ErrForbiddenAccess, got %v", err)
	}
	mock.ExpectQuery("SELECT EXISTS").WithArgs("ws1", "u1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	if _, err := service.SendWorkspaceMessage(context.Background(), "ws1", "u1", "hello"); !errors.Is(err, ErrForbiddenAccess) {
		t.Fatalf("SendWorkspaceMessage must return forbidden, got %v", err)
	}

	mock.ExpectQuery("SELECT EXISTS").WithArgs("ws1", "u1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("WITH inserted AS").WithArgs("ws1", "u1", "hello").
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "name", "picture"}).
			AddRow("m1", time.Now(), "Alice", "pic.jpg"))
	if _, err := service.SendWorkspaceMessage(context.Background(), "ws1", "u1", "hello"); err != nil {
		t.Fatalf("valid workspace message returned error: %v", err)
	}

	mock.ExpectQuery("SELECT EXISTS").WithArgs("ws1", "u1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("SELECT id, workspace_id").WithArgs("ws1", 50).
		WillReturnRows(sqlmock.NewRows([]string{"id", "workspace_id", "sender_id", "sender_name", "picture", "content", "created_at"}))
	if _, err := service.ListWorkspaceMessages(context.Background(), "ws1", "u1", 0); err != nil {
		t.Fatalf("ListWorkspaceMessages returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceMessageLengthValidation(t *testing.T) {
	_, rdb := newChatRedis(t)
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT EXISTS").WithArgs("ws1", "u1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	service := NewService(db, rdb)

	if _, err := service.SendWorkspaceMessage(context.Background(), "ws1", "u1", strings.Repeat("x", 1001)); !errors.Is(err, ErrMessageLong) {
		t.Fatalf("expected ErrMessageLong, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
