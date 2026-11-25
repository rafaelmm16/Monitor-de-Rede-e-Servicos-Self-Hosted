package checker_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/rafael/monitor-de-rede-backend/internal/checker"
	"github.com/rafael/monitor-de-rede-backend/internal/models"
)

func TestCheckHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("service-healthy-ok"))
	}))
	defer srv.Close()

	res := checker.Run(context.Background(), models.Monitor{
		ID:             "m1",
		Type:           models.MonitorHTTP,
		Target:         srv.URL,
		ExpectedStatus: 200,
		Keyword:        "service-healthy-ok",
		TimeoutSeconds: 5,
	})
	if res.Status != models.StatusUp {
		t.Fatalf("expected StatusUp, got %s (err: %s)", res.Status, res.Error)
	}
	if res.StatusCode != 200 {
		t.Fatalf("expected status code 200, got %d", res.StatusCode)
	}

	resFail := checker.Run(context.Background(), models.Monitor{
		ID:             "m2",
		Type:           models.MonitorHTTP,
		Target:         srv.URL,
		ExpectedStatus: 201,
		TimeoutSeconds: 5,
	})
	if resFail.Status != models.StatusDown {
		t.Fatalf("expected StatusDown on status mismatch, got %s", resFail.Status)
	}

	resKeywordFail := checker.Run(context.Background(), models.Monitor{
		ID:             "m3",
		Type:           models.MonitorHTTP,
		Target:         srv.URL,
		ExpectedStatus: 200,
		Keyword:        "missing-token",
		TimeoutSeconds: 5,
	})
	if resKeywordFail.Status != models.StatusDown {
		t.Fatalf("expected StatusDown on keyword mismatch, got %s", resKeywordFail.Status)
	}
}

func TestCheckTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)

	res := checker.Run(context.Background(), models.Monitor{
		ID:             "tcp1",
		Type:           models.MonitorTCP,
		Target:         "127.0.0.1",
		Port:           port,
		TimeoutSeconds: 3,
	})
	if res.Status != models.StatusUp {
		t.Fatalf("expected TCP StatusUp, got %s (err: %s)", res.Status, res.Error)
	}

	_ = ln.Close()
	resDown := checker.Run(context.Background(), models.Monitor{
		ID:             "tcp2",
		Type:           models.MonitorTCP,
		Target:         "127.0.0.1",
		Port:           port,
		TimeoutSeconds: 1,
	})
	if resDown.Status != models.StatusDown {
		t.Fatalf("expected TCP StatusDown after close, got %s", resDown.Status)
	}
}
